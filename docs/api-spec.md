# API Specification

## Response Format

All JSON responses use a unified structure:

```json
// Success
{"ok": true, "data": {...}}

// Error
{"ok": false, "error": {"code": "BAD_REQUEST", "message": "..."}}
```

## Authentication

Protected endpoints require `X-Auth-Token` header. Two token scopes:

- **upload** — can upload builds
- **admin** — full admin access (includes upload)

## Public Endpoints

### Product Page

```
GET /products/:slug
```

HTML page with download buttons, QR code, platform detection. Supports `?r=<release_id>` to highlight a specific release.

### Direct Download

```
GET /d/:releaseID
```

Returns binary file (IPA/APK/etc). Supports HTTP Range requests. Increments download counter.

### iOS Manifest

```
GET /ios/:releaseID/manifest.plist
```

Returns `application/xml` plist for `itms-services://` installation. Contains bundle identifier, version, and download URL.

### UDID Binding (iOS)

```
GET  /udid/profile.mobileconfig?variant=<variantID>
POST /udid/callback                                    # Called by iOS automatically
GET  /udid/status?variant=<variantID>                  # Check if device is bound
```

### Health Check

```
GET /healthz
```

Returns `{"ok": true}` if the server is running.

## Upload

### Standard Upload

```
POST /upload
Content-Type: multipart/form-data
X-Auth-Token: <upload_token>
```

| Field | Required | Description |
|-------|----------|-------------|
| `variant_id` | yes | Target variant ID |
| `app_file` | yes | IPA/APK/binary file |
| `version` | no | Version string (e.g. "1.0.0") |
| `build` | no | Build number (integer) |
| `channel` | no | Channel (e.g. "internal", "beta") |
| `min_os` | no | Minimum OS version |
| `changelog` | no | Release notes |

**Response** (201):

```json
{
  "ok": true,
  "data": {
    "app": {"id": "app_xxx", "name": "MyApp", "platform": "ios", "bundle_id": "com.example.app"},
    "release": {"id": "rel_xxx", "version": "1.0.0", "build": 1},
    "urls": {
      "page": "https://dist.example.com/products/my-app",
      "download": "https://dist.example.com/d/rel_xxx",
      "ios_manifest": "https://dist.example.com/ios/rel_xxx/manifest.plist",
      "ios_install": "itms-services://..."
    }
  }
}
```

### Smart Upload

```
POST /admin/api/smart-upload
Content-Type: multipart/form-data
X-Auth-Token: <admin_token>
```

Auto-detects metadata from the package (bundle ID, version, icon, etc). Same fields as standard upload but `version` and `build` are auto-parsed.

## Admin API

All admin endpoints require `X-Auth-Token` with admin scope.

### Products

```
GET    /admin/api/products                              # List products
POST   /admin/api/products                              # Create product
GET    /admin/api/products/:productID                   # Get product with variants
PUT    /admin/api/products/:productID                   # Update product
DELETE /admin/api/products/:productID                   # Delete product (cascades)
```

### Variants

```
POST   /admin/api/products/:productID/variants          # Create variant
PUT    /admin/api/variants/:variantID                    # Update variant
DELETE /admin/api/variants/:variantID                    # Delete variant (cascades)
GET    /admin/api/variants/:variantID/stats              # Variant download stats
```

### Releases

```
DELETE /admin/api/releases/:releaseID                    # Delete release
```

### Publishing

```
PUT /admin/api/apps/:appID/publish                      # Publish app
PUT /admin/api/apps/:appID/unpublish                    # Unpublish app
```

### Events & Devices

```
GET  /admin/api/events                                  # Query events (visit/click/download)
GET  /admin/api/ios_devices                             # List iOS devices
POST /admin/api/devices/:id/register-apple              # Register device with Apple
POST /admin/api/devices/register-apple                  # Batch register devices
```

### Settings

```
GET /admin/api/settings                                 # Get system settings
PUT /admin/api/settings                                 # Update settings
GET /admin/api/upload-config                            # Get upload configuration
GET /admin/api/apple/status                             # Check Apple API status
GET /admin/api/apple/devices                            # List Apple-registered devices
```

### Export

```
GET /admin/exports/releases.csv                         # Export releases
GET /admin/exports/events.csv                           # Export events
GET /admin/exports/ios_devices.csv                      # Export iOS devices
```

## Storage

Files are stored at: `uploads/{product_id}/{variant_id}/{release_id}/filename.ext`

Each release also has a `meta.json` snapshot (local storage only).

## ID Format

IDs use prefix + random hex: `app_7f3xn`, `rel_b1cqa`, `prd_abc123`, `var_def456`.
