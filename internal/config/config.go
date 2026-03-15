package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Server struct {
		Port             string   `json:"port"`
		BaseURL          string   `json:"base_url"`          // Deprecated: use PrimaryDomain instead
		PrimaryDomain    string   `json:"primary_domain"`    // Primary domain for Apple communication (UDID callback, manifest)
		SecondaryDomains []string `json:"secondary_domains"` // Secondary domains for distribution
		Organization     string   `json:"organization"`      // Organization name for mobileconfig
		BundleIDPrefix   string   `json:"bundle_id_prefix"`  // Bundle ID prefix (e.g., com.yourcompany.fenfa)
		DataDir          string   `json:"data_dir"`
		DBPath           string   `json:"db_path"`
		DevProxyFront    string   `json:"dev_proxy_front"`
		DevProxyAdmin    string   `json:"dev_proxy_admin"`
	} `json:"server"`
	Auth struct {
		UploadTokens []string `json:"upload_tokens"`
		AdminTokens  []string `json:"admin_tokens"`
	} `json:"auth"`
}

func Default() *Config {
	c := &Config{}
	c.Server.Port = "8000"
	c.Server.PrimaryDomain = "http://localhost:8000"
	c.Server.SecondaryDomains = []string{}
	c.Server.Organization = "Fenfa Distribution"
	c.Server.BundleIDPrefix = "com.fenfa.profile"
	c.Server.DataDir = "data"
	c.Server.DBPath = "data/fenfa.db"
	c.Auth.UploadTokens = []string{"dev-upload-token"}
	c.Auth.AdminTokens = []string{"dev-admin-token"}
	return c
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		// not found -> defaults
		c := Default()
		applyEnv(c)
		return c, nil
	}
	c := Default()
	if err := json.Unmarshal(b, c); err != nil {
		applyEnv(c)
		return c, nil
	}
	// Backward compatibility: if PrimaryDomain is empty but BaseURL is set, use BaseURL
	if c.Server.PrimaryDomain == "" && c.Server.BaseURL != "" {
		c.Server.PrimaryDomain = c.Server.BaseURL
	}
	applyEnv(c)
	return c, nil
}

// applyEnv overrides config values with environment variables if set.
func applyEnv(c *Config) {
	if v := os.Getenv("FENFA_PORT"); v != "" {
		c.Server.Port = v
	}
	if v := os.Getenv("FENFA_DATA_DIR"); v != "" {
		c.Server.DataDir = v
		c.Server.DBPath = v + "/fenfa.db"
	}
	if v := os.Getenv("FENFA_PRIMARY_DOMAIN"); v != "" {
		c.Server.PrimaryDomain = v
	}
	if v := os.Getenv("FENFA_ADMIN_TOKEN"); v != "" {
		c.Auth.AdminTokens = []string{v}
	}
	if v := os.Getenv("FENFA_UPLOAD_TOKEN"); v != "" {
		c.Auth.UploadTokens = []string{v}
	}
}

// GetPrimaryDomain returns the primary domain for Apple communication
func (c *Config) GetPrimaryDomain() string {
	if c.Server.PrimaryDomain != "" {
		return c.Server.PrimaryDomain
	}
	if c.Server.BaseURL != "" {
		return c.Server.BaseURL
	}
	return "http://localhost:8000"
}

// GetOrganization returns the organization name for mobileconfig
func (c *Config) GetOrganization() string {
	if c.Server.Organization != "" {
		return c.Server.Organization
	}
	return "Fenfa Distribution"
}

// GetBundleIDPrefix returns the bundle ID prefix for mobileconfig
func (c *Config) GetBundleIDPrefix() string {
	if c.Server.BundleIDPrefix != "" {
		return c.Server.BundleIDPrefix
	}
	return "com.fenfa.profile"
}
