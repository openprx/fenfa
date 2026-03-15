// Package apple provides a client for Apple's App Store Connect API
package apple

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL     = "https://api.appstoreconnect.apple.com/v1"
	tokenExpiry = 20 * time.Minute
)

// Client provides methods to interact with App Store Connect API
type Client struct {
	keyID      string
	issuerID   string
	privateKey *ecdsa.PrivateKey
	teamID     string
	httpClient *http.Client
}

// Device represents a registered device in Apple Developer Portal
type Device struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name        string `json:"name"`
		UDID        string `json:"udid"`
		Platform    string `json:"platform"`
		Status      string `json:"status"`
		DeviceClass string `json:"deviceClass"`
		Model       string `json:"model"`
		AddedDate   string `json:"addedDate"`
	} `json:"attributes"`
}

// DevicesResponse is the API response for listing devices
type DevicesResponse struct {
	Data  []Device `json:"data"`
	Links struct {
		Self string `json:"self"`
		Next string `json:"next"`
	} `json:"links"`
	Meta struct {
		Paging struct {
			Total int `json:"total"`
			Limit int `json:"limit"`
		} `json:"paging"`
	} `json:"meta"`
}

// DeviceResponse is the API response for a single device
type DeviceResponse struct {
	Data Device `json:"data"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Errors []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Code   string `json:"code"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

func (e *ErrorResponse) Error() string {
	if len(e.Errors) == 0 {
		return "unknown error"
	}
	return fmt.Sprintf("%s: %s", e.Errors[0].Code, e.Errors[0].Detail)
}

// NewClient creates a new Apple API client
func NewClient(keyID, issuerID, privateKeyPEM, teamID string) (*Client, error) {
	if keyID == "" || issuerID == "" || privateKeyPEM == "" {
		return nil, errors.New("missing required credentials")
	}

	privateKey, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Client{
		keyID:      keyID,
		issuerID:   issuerID,
		privateKey: privateKey,
		teamID:     teamID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// parsePrivateKey parses a PEM-encoded ECDSA private key
func parsePrivateKey(pemData string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an ECDSA private key")
	}

	return ecdsaKey, nil
}

// generateJWT creates a JWT token for API authentication
func (c *Client) generateJWT() (string, error) {
	now := time.Now()
	exp := now.Add(tokenExpiry)

	// Header
	header := map[string]string{
		"alg": "ES256",
		"kid": c.keyID,
		"typ": "JWT",
	}

	// Payload
	payload := map[string]interface{}{
		"iss": c.issuerID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"aud": "appstoreconnect-v1",
	}

	// Encode header and payload
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Sign
	signingInput := headerB64 + "." + payloadB64
	signature, err := signES256(c.privateKey, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureB64, nil
}

// signES256 signs data using ECDSA with SHA-256
func signES256(key *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return nil, err
	}

	// Convert r and s to fixed-size byte arrays (32 bytes each for P-256)
	curveBits := key.Curve.Params().BitSize
	keyBytes := (curveBits + 7) / 8

	rBytes := r.Bytes()
	sBytes := s.Bytes()

	signature := make([]byte, 2*keyBytes)
	copy(signature[keyBytes-len(rBytes):keyBytes], rBytes)
	copy(signature[2*keyBytes-len(sBytes):], sBytes)

	return signature, nil
}

// doRequest performs an authenticated API request
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	token, err := c.generateJWT()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && len(errResp.Errors) > 0 {
			return nil, &errResp
		}
		return nil, fmt.Errorf("API error: %s (status %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}

// TestConnection tests the API connection by listing devices
func (c *Client) TestConnection() error {
	_, err := c.doRequest("GET", "/devices?limit=1", nil)
	return err
}

// ListDevices returns all registered devices
func (c *Client) ListDevices() ([]Device, error) {
	var allDevices []Device
	path := "/devices?limit=200"

	for path != "" {
		respBody, err := c.doRequest("GET", path, nil)
		if err != nil {
			return nil, err
		}

		var resp DevicesResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		allDevices = append(allDevices, resp.Data...)

		// Handle pagination
		if resp.Links.Next != "" {
			path = strings.TrimPrefix(resp.Links.Next, baseURL)
		} else {
			path = ""
		}
	}

	return allDevices, nil
}

// RegisterDevice registers a new device with Apple
func (c *Client) RegisterDevice(udid, name string) (*Device, error) {
	if udid == "" {
		return nil, errors.New("UDID is required")
	}
	if name == "" {
		name = "Device " + udid[:8]
	}

	reqBody := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "devices",
			"attributes": map[string]string{
				"name":     name,
				"udid":     udid,
				"platform": "IOS",
			},
		},
	}

	respBody, err := c.doRequest("POST", "/devices", reqBody)
	if err != nil {
		return nil, err
	}

	var resp DeviceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp.Data, nil
}

// FindDeviceByUDID searches for a device by its UDID
func (c *Client) FindDeviceByUDID(udid string) (*Device, error) {
	path := fmt.Sprintf("/devices?filter[udid]=%s", udid)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var resp DevicesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, nil // Not found
	}

	return &resp.Data[0], nil
}
