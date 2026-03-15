package store

import "time"

// Product represents a multi-platform product page.
type Product struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`
	Slug        string    `gorm:"uniqueIndex;size:128" json:"slug"`
	Name        string    `gorm:"size:255" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IconPath    string    `gorm:"size:255" json:"icon_path"`
	Published   bool      `gorm:"default:true;index" json:"published"`
	LegacyAppID *string   `gorm:"uniqueIndex;size:64" json:"legacy_app_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Variant represents a single platform under a product.
type Variant struct {
	ID            string    `gorm:"primaryKey;size:64" json:"id"`
	ProductID     string    `gorm:"index;size:64" json:"product_id"`
	Platform      string    `gorm:"index;size:16" json:"platform"`
	Identifier    string    `gorm:"index;size:255" json:"identifier"`
	DisplayName   string    `gorm:"size:255" json:"display_name"`
	Arch          string    `gorm:"size:32" json:"arch"`
	InstallerType string    `gorm:"size:32" json:"installer_type"`
	MinOS         string    `gorm:"size:64" json:"min_os"`
	Published     bool      `gorm:"default:true;index" json:"published"`
	SortOrder     int       `gorm:"default:0" json:"sort_order"`
	LegacyAppID   *string   `gorm:"uniqueIndex;size:64" json:"legacy_app_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// App represents an application (iOS/Android)
type App struct {
	ID            string    `gorm:"primaryKey;size:64" json:"id"`
	Platform      string    `gorm:"index;size:16" json:"platform"`
	BundleID      string    `gorm:"index;size:255" json:"bundle_id"`
	ApplicationID string    `gorm:"index;size:255" json:"application_id"`
	Name          string    `gorm:"size:255" json:"name"`
	IconPath      string    `gorm:"size:255" json:"icon_path"`
	Published     bool      `gorm:"default:true;index" json:"published"` // Whether the app is published and accessible to users
	CreatedAt     time.Time `json:"created_at"`
}

// Release represents a single uploaded build
type Release struct {
	ID            string    `gorm:"primaryKey;size:64" json:"id"`
	AppID         string    `gorm:"index;size:64" json:"app_id"` // Legacy field kept for compatibility.
	VariantID     string    `gorm:"index;size:64" json:"variant_id"`
	Version       string    `gorm:"size:64" json:"version"`
	Build         int64     `gorm:"index" json:"build"`
	Changelog     string    `gorm:"type:text" json:"changelog"`
	MinOS         string    `gorm:"size:64" json:"min_os"`
	SizeBytes     int64     `json:"size_bytes"`
	SHA256        string    `gorm:"size:128" json:"sha256"`
	StoragePath   string    `gorm:"size:512" json:"storage_path"`
	FileName      string    `gorm:"size:255" json:"file_name"`
	FileExt       string    `gorm:"size:32" json:"file_ext"`
	MimeType      string    `gorm:"size:128" json:"mime_type"`
	DownloadCount int64     `gorm:"index" json:"download_count"`
	Channel       string    `gorm:"size:64" json:"channel"`
	CreatedAt     time.Time `json:"created_at"`
}

// AuthToken for upload/admin scopes
type AuthToken struct {
	Token     string     `gorm:"primaryKey;size:128"`
	Label     string     `gorm:"size:128"`
	Scopes    string     `gorm:"size:128"`
	RateLimit int        `gorm:"default:0"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

// Event logs uploads/downloads
type Event struct {
	ID        uint      `gorm:"primaryKey"`
	Ts        time.Time `gorm:"autoCreateTime"`
	Type      string    `gorm:"size:32"`
	AppID     string    `gorm:"size:64"`
	VariantID string    `gorm:"size:64;index"`
	ReleaseID string    `gorm:"size:64"`
	IP        string    `gorm:"size:64"`
	UA        string    `gorm:"size:255"`
	Extra     string    `gorm:"type:text"`
}

// IOSDevice stores device UDID bindings
type IOSDevice struct {
	ID         string     `gorm:"primaryKey;size:64"`
	UDID       string     `gorm:"column:ud_id;uniqueIndex;size:128"`
	DeviceName string     `gorm:"size:128"`
	Model      string     `gorm:"size:128"`
	OSVersion  string     `gorm:"size:64"`
	CreatedAt  time.Time  `json:"created_at"`
	VerifiedAt *time.Time `json:"verified_at"`
	LastIP     string     `gorm:"size:64"`

	// Apple Developer Portal registration status
	AppleRegistered   bool       `gorm:"default:false" json:"apple_registered"`
	AppleRegisteredAt *time.Time `json:"apple_registered_at"`
	AppleDeviceID     string     `gorm:"size:64" json:"apple_device_id"` // Device ID returned by Apple API
}

// DeviceAppBinding records which devices are bound to which apps
type DeviceAppBinding struct {
	ID        string    `gorm:"primaryKey;size:64"`
	DeviceID  string    `gorm:"index;size:64" json:"device_id"`  // References IOSDevice.ID
	UDID      string    `gorm:"index;size:128" json:"udid"`      // Denormalized for easier queries
	AppID     string    `gorm:"index;size:64" json:"app_id"`     // Legacy field kept for compatibility.
	VariantID string    `gorm:"index;size:64" json:"variant_id"` // References Variant.ID
	CreatedAt time.Time `json:"created_at"`
}

// SystemSettings stores global system configuration
// This is a singleton table with only one row (ID = "default")
type SystemSettings struct {
	ID               string    `gorm:"primaryKey;size:64" json:"id"` // Always "default"
	PrimaryDomain    string    `gorm:"size:255" json:"primary_domain"`
	SecondaryDomains string    `gorm:"type:text" json:"secondary_domains"` // JSON array stored as text
	Organization     string    `gorm:"size:255" json:"organization"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Upload storage configuration
	StorageType  string `gorm:"size:32;default:local" json:"storage_type"` // "local" | "s3"
	UploadDomain string `gorm:"size:255" json:"upload_domain"`             // Subdomain for uploads (bypasses CDN limits)

	// S3/R2 configuration
	S3Endpoint  string `gorm:"size:255" json:"-"`
	S3Bucket    string `gorm:"size:255" json:"-"`
	S3AccessKey string `gorm:"size:255" json:"-"`
	S3SecretKey string `gorm:"size:512" json:"-"`
	S3PublicURL string `gorm:"size:255" json:"s3_public_url"` // Public URL prefix for downloads

	// Apple Developer API credentials (App Store Connect API)
	AppleKeyID      string `gorm:"size:64" json:"-"`   // API Key ID (e.g., "ABC123DEF4")
	AppleIssuerID   string `gorm:"size:64" json:"-"`   // Issuer ID (UUID format)
	ApplePrivateKey string `gorm:"type:text" json:"-"` // Private key content (PEM format)
	AppleTeamID     string `gorm:"size:32" json:"-"`   // Team ID
}

// UDIDNonce stores one-time nonces for UDID callback verification
type UDIDNonce struct {
	Nonce     string    `gorm:"primaryKey;size:64"`
	AppID     string    `gorm:"size:64;index"` // Legacy field kept for compatibility.
	VariantID string    `gorm:"size:64;index" json:"variant_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
	Used      bool      `gorm:"default:false;index" json:"used"`
}

// ProvisioningProfile stores iOS provisioning profile information extracted from IPA
type ProvisioningProfile struct {
	ID                   string    `gorm:"primaryKey;size:64" json:"id"`
	ReleaseID            string    `gorm:"uniqueIndex;size:64" json:"release_id"`
	UUID                 string    `gorm:"size:64;index" json:"uuid"`            // Profile UUID
	Name                 string    `gorm:"size:255" json:"name"`                 // Profile name
	TeamID               string    `gorm:"size:32;index" json:"team_id"`         // Team identifier
	TeamName             string    `gorm:"size:255" json:"team_name"`            // Team/Organization name
	AppIDName            string    `gorm:"size:255" json:"app_id_name"`          // App ID Name
	AppIDPrefix          string    `gorm:"size:32" json:"app_id_prefix"`         // App ID prefix (Team ID)
	BundleID             string    `gorm:"size:255;index" json:"bundle_id"`      // Application identifier
	Platform             string    `gorm:"size:32" json:"platform"`              // Platform (iOS, tvOS, etc.)
	ProfileType          string    `gorm:"size:32" json:"profile_type"`          // development, ad-hoc, enterprise, app-store
	ProvisionsAllDevices bool      `json:"provisions_all_devices"`               // true for enterprise profiles
	CreationDate         time.Time `json:"creation_date"`                        // Profile creation date
	ExpirationDate       time.Time `gorm:"index" json:"expiration_date"`         // Profile expiration date
	Certificates         string    `gorm:"type:text" json:"certificates"`        // JSON array of certificate info
	ProvisionedDevices   string    `gorm:"type:text" json:"provisioned_devices"` // JSON array of device UDIDs
	Entitlements         string    `gorm:"type:text" json:"entitlements"`        // JSON object of entitlements
	CreatedAt            time.Time `json:"created_at"`
}
