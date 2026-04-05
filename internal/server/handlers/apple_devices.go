package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/apple"
	"github.com/openprx/fenfa/internal/store"
)

// getAppleClient creates an Apple API client from system settings
func getAppleClient(db *gorm.DB) (*apple.Client, error) {
	var settings store.SystemSettings
	if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
		return nil, err
	}

	return apple.NewClient(
		settings.AppleKeyID,
		settings.AppleIssuerID,
		settings.ApplePrivateKey,
		settings.AppleTeamID,
	)
}

// AppleStatus checks if Apple API is configured and connection works
func AppleStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings store.SystemSettings
		if err := db.Where("id = ?", "default").First(&settings).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"data": gin.H{
					"configured": false,
					"connected":  false,
					"message":    "Settings not found",
				},
			})
			return
		}

		configured := settings.AppleKeyID != "" && settings.AppleIssuerID != "" && settings.ApplePrivateKey != ""
		if !configured {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"data": gin.H{
					"configured": false,
					"connected":  false,
					"message":    "Apple API credentials not configured",
				},
			})
			return
		}

		// Test connection
		client, err := getAppleClient(db)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"data": gin.H{
					"configured": true,
					"connected":  false,
					"message":    "Failed to create client: " + err.Error(),
				},
			})
			return
		}

		if err := client.TestConnection(); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"data": gin.H{
					"configured": true,
					"connected":  false,
					"message":    "Connection test failed: " + err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"configured": true,
				"connected":  true,
				"message":    "Connected successfully",
			},
		})
	}
}

// AppleListDevices lists all devices registered in Apple Developer Portal
func AppleListDevices(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := getAppleClient(db)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "APPLE_NOT_CONFIGURED",
					"message": "Apple API not configured: " + err.Error(),
				},
			})
			return
		}

		devices, err := client.ListDevices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "APPLE_API_ERROR",
					"message": "Failed to list devices: " + err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": devices,
		})
	}
}

// RegisterDeviceToApple registers a single device to Apple Developer Portal
func RegisterDeviceToApple(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := c.Param("id")
		if deviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "Device ID is required",
				},
			})
			return
		}

		// Find device in our database
		var device store.IOSDevice
		if err := db.Where("id = ?", deviceID).First(&device).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DEVICE_NOT_FOUND",
					"message": "Device not found",
				},
			})
			return
		}

		// Check if already registered
		if device.AppleRegistered {
			c.JSON(http.StatusOK, gin.H{
				"ok": true,
				"data": gin.H{
					"message":         "Device already registered",
					"apple_device_id": device.AppleDeviceID,
				},
			})
			return
		}

		// Get Apple client
		client, err := getAppleClient(db)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "APPLE_NOT_CONFIGURED",
					"message": "Apple API not configured: " + err.Error(),
				},
			})
			return
		}

		// Check if device already exists in Apple
		existingDevice, err := client.FindDeviceByUDID(device.UDID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "APPLE_API_ERROR",
					"message": "Failed to check device: " + err.Error(),
				},
			})
			return
		}

		var appleDeviceID string
		if existingDevice != nil {
			// Device already exists in Apple
			appleDeviceID = existingDevice.ID
		} else {
			// Register new device
			deviceName := device.DeviceName
			if deviceName == "" {
				deviceName = device.Model
			}
			if deviceName == "" {
				deviceName = "Device " + device.UDID[:8]
			}

			newDevice, err := client.RegisterDevice(device.UDID, deviceName)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"ok": false,
					"error": gin.H{
						"code":    "APPLE_REGISTER_FAILED",
						"message": "Failed to register device: " + err.Error(),
					},
				})
				return
			}
			appleDeviceID = newDevice.ID
		}

		// Update our database
		now := time.Now()
		updates := map[string]interface{}{
			"apple_registered":    true,
			"apple_registered_at": now,
			"apple_device_id":     appleDeviceID,
		}
		if err := db.Model(&device).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "DATABASE_ERROR",
					"message": "Failed to update device status",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"message":         "Device registered successfully",
				"apple_device_id": appleDeviceID,
			},
		})
	}
}

// BatchRegisterDevicesToApple registers multiple devices to Apple Developer Portal
func BatchRegisterDevicesToApple(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			DeviceIDs []string `json:"device_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "Invalid request body",
				},
			})
			return
		}

		if len(req.DeviceIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "INVALID_REQUEST",
					"message": "No device IDs provided",
				},
			})
			return
		}

		// Get Apple client
		client, err := getAppleClient(db)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "APPLE_NOT_CONFIGURED",
					"message": "Apple API not configured: " + err.Error(),
				},
			})
			return
		}

		// Process each device
		results := make([]map[string]interface{}, 0, len(req.DeviceIDs))
		successCount := 0
		failCount := 0

		for _, deviceID := range req.DeviceIDs {
			result := map[string]interface{}{
				"device_id": deviceID,
			}

			// Find device
			var device store.IOSDevice
			if err := db.Where("id = ?", deviceID).First(&device).Error; err != nil {
				result["success"] = false
				result["error"] = "Device not found"
				failCount++
				results = append(results, result)
				continue
			}

			// Skip if already registered
			if device.AppleRegistered {
				result["success"] = true
				result["message"] = "Already registered"
				result["apple_device_id"] = device.AppleDeviceID
				successCount++
				results = append(results, result)
				continue
			}

			// Check if exists in Apple
			existingDevice, err := client.FindDeviceByUDID(device.UDID)
			if err != nil {
				result["success"] = false
				result["error"] = "Failed to check device: " + err.Error()
				failCount++
				results = append(results, result)
				continue
			}

			var appleDeviceID string
			if existingDevice != nil {
				appleDeviceID = existingDevice.ID
			} else {
				// Register new device
				deviceName := device.DeviceName
				if deviceName == "" {
					deviceName = device.Model
				}
				if deviceName == "" {
					deviceName = "Device " + device.UDID[:8]
				}

				newDevice, err := client.RegisterDevice(device.UDID, deviceName)
				if err != nil {
					result["success"] = false
					result["error"] = "Failed to register: " + err.Error()
					failCount++
					results = append(results, result)
					continue
				}
				appleDeviceID = newDevice.ID
			}

			// Update database
			now := time.Now()
			updates := map[string]interface{}{
				"apple_registered":    true,
				"apple_registered_at": now,
				"apple_device_id":     appleDeviceID,
			}
			if err := db.Model(&device).Updates(updates).Error; err != nil {
				result["success"] = false
				result["error"] = "Failed to update database"
				failCount++
				results = append(results, result)
				continue
			}

			result["success"] = true
			result["message"] = "Registered successfully"
			result["apple_device_id"] = appleDeviceID
			successCount++
			results = append(results, result)
		}

		c.JSON(http.StatusOK, gin.H{
			"ok": true,
			"data": gin.H{
				"success_count": successCount,
				"fail_count":    failCount,
				"results":       results,
			},
		})
	}
}
