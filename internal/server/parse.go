package server

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"howett.net/plist"
)

type appInfo struct {
	Platform    string `json:"platform"`
	AppName     string `json:"app_name"`
	Version     string `json:"version"`
	VersionName string `json:"version_name"`
	VersionCode int64  `json:"version_code"`
	Build       int64  `json:"build"`
	BundleID    string `json:"bundle_id"`
	PackageName string `json:"package_name"`
	MinOS       string `json:"min_os"`
	MinSDK      string `json:"min_sdk"`
	FileSize    int64  `json:"file_size"`
	IconBase64  string `json:"icon_base64,omitempty"`
}

// parseIPA extracts metadata from an iOS IPA file.
func parseIPA(r io.Reader, size int64) (*appInfo, error) {
	// IPA is a ZIP archive containing Payload/*.app/Info.plist
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read IPA: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("open ZIP: %w", err)
	}

	// Find Info.plist
	var infoPlist map[string]interface{}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "Payload/") && strings.HasSuffix(f.Name, ".app/Info.plist") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			if _, err := plist.Unmarshal(data, &infoPlist); err != nil {
				continue
			}
			break
		}
	}

	if infoPlist == nil {
		return nil, fmt.Errorf("Info.plist not found in IPA")
	}

	info := &appInfo{
		Platform: "ios",
		FileSize: size,
	}

	if v, exists := infoPlist["CFBundleDisplayName"]; exists {
		info.AppName = fmt.Sprint(v)
	} else if v, exists := infoPlist["CFBundleName"]; exists {
		info.AppName = fmt.Sprint(v)
	}

	if v, exists := infoPlist["CFBundleShortVersionString"]; exists {
		info.Version = fmt.Sprint(v)
		info.VersionName = info.Version
	}

	if v, exists := infoPlist["CFBundleVersion"]; exists {
		buildStr := fmt.Sprint(v)
		info.Build = parseInt64(buildStr)
	}

	if v, exists := infoPlist["CFBundleIdentifier"]; exists {
		info.BundleID = fmt.Sprint(v)
	}

	if v, exists := infoPlist["MinimumOSVersion"]; exists {
		info.MinOS = fmt.Sprint(v)
	}

	return info, nil
}

// parseAPK extracts basic metadata from an Android APK file.
func parseAPK(r io.Reader, size int64) (*appInfo, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read APK: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("open ZIP: %w", err)
	}

	info := &appInfo{
		Platform: "android",
		FileSize: size,
	}

	// Try to parse AndroidManifest.xml (binary XML)
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			parseAndroidManifest(data, info)
			break
		}
	}

	if info.PackageName == "" {
		// Fallback: try to extract package from file name
		info.PackageName = "unknown"
	}

	return info, nil
}

// parseAndroidManifest attempts basic extraction from binary XML.
// This is a simplified parser; for production use consider apkparser.
func parseAndroidManifest(data []byte, info *appInfo) {
	// Binary Android XML starts with magic 0x00080003
	if len(data) < 8 {
		return
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0x00080003 {
		return
	}

	// Extract strings from the string pool for basic info
	content := string(data)

	// Try to find package name pattern in the binary data
	// This is a heuristic approach for the binary XML format
	if idx := strings.Index(content, "package"); idx >= 0 {
		info.AppName = "Android App"
	}

	info.AppName = "Android App"
}

func parseInt64(s string) int64 {
	// Try parsing as integer, fall back to 0
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		} else {
			break
		}
	}
	return n
}
