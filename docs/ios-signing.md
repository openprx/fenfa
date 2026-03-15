# iOS Re-signing Guide

Fenfa distributes iOS apps via Ad-Hoc or Enterprise provisioning. For Ad-Hoc distribution, the IPA must be signed with a provisioning profile that includes the target device's UDID. This guide covers how to re-sign IPAs on your server using [zsign](https://github.com/nicklama/zsign).

## Overview

```
Developer builds IPA (dev/app-store signed)
         ↓
Upload to Fenfa (original IPA stored)
         ↓
User visits download page → binds UDID
         ↓
Admin registers UDID in Apple Developer Portal
         ↓
Admin regenerates provisioning profile (includes new UDID)
         ↓
Re-sign IPA with zsign using new profile
         ↓
Upload re-signed IPA to Fenfa → user can install
```

## Prerequisites

- Apple Developer account ($99/year)
- Distribution certificate (.p12) and password
- Ad-Hoc provisioning profile (.mobileprovision) with target UDIDs
- Server with zsign installed

## Install zsign

### From source (Linux/macOS)

```bash
git clone https://github.com/nicklama/zsign.git
cd zsign
g++ *.cpp common/*.cpp -lcrypto -lssl -O2 -o zsign
sudo mv zsign /usr/local/bin/
```

### Alpine Linux (Docker)

```bash
apk add --no-cache g++ openssl-dev make git
git clone https://github.com/nicklama/zsign.git /tmp/zsign
cd /tmp/zsign && g++ *.cpp common/*.cpp -lcrypto -lssl -O2 -o /usr/local/bin/zsign
rm -rf /tmp/zsign
```

### Verify

```bash
zsign --help
```

## Prepare Signing Materials

### 1. Export distribution certificate

From Keychain Access (macOS):

1. Open Keychain Access
2. Find your **Apple Distribution** certificate
3. Right-click → Export → save as `cert.p12`
4. Set a password (you'll need it for zsign)

Or from the command line if you have the .cer and .key files:

```bash
openssl pkcs12 -export -in cert.cer -inkey private.key -out cert.p12
```

### 2. Download provisioning profile

From [Apple Developer Portal](https://developer.apple.com/account/resources/profiles/list):

1. Create or update an **Ad-Hoc** provisioning profile
2. Select the distribution certificate
3. Select all target devices (UDIDs)
4. Download the `.mobileprovision` file

### 3. Organize files

```
/opt/signing/
├── cert.p12                    # Distribution certificate
├── adhoc.mobileprovision       # Ad-Hoc provisioning profile
└── password.txt                # Certificate password (optional)
```

## Re-sign an IPA

### Basic command

```bash
zsign -k cert.p12 -p "certificate_password" -m adhoc.mobileprovision -o signed.ipa original.ipa
```

### Parameters

| Flag | Description |
|------|-------------|
| `-k` | Path to .p12 certificate |
| `-p` | Certificate password |
| `-m` | Path to .mobileprovision |
| `-o` | Output IPA path |
| `-b` | Override bundle ID (optional) |
| `-n` | Override app name (optional) |

### Change bundle ID during re-sign

```bash
zsign -k cert.p12 -p "password" -m adhoc.mobileprovision \
  -b "com.yourcompany.app" \
  -o signed.ipa original.ipa
```

## Workflow with Fenfa

### Manual workflow

1. **Collect UDIDs** — Users bind devices via Fenfa download page
2. **Export UDIDs** — Admin panel → UDID Devices → Export CSV
3. **Register with Apple** — Add UDIDs in Apple Developer Portal (or use Fenfa's Apple API integration for auto-registration)
4. **Regenerate profile** — Download updated Ad-Hoc provisioning profile
5. **Re-sign IPA**:
   ```bash
   zsign -k cert.p12 -p "password" -m adhoc.mobileprovision \
     -o signed.ipa original.ipa
   ```
6. **Upload to Fenfa** — Upload the re-signed IPA via admin panel or API:
   ```bash
   curl -X POST https://your-domain.com/upload \
     -H "X-Auth-Token: your-upload-token" \
     -F "variant_id=var_xxx" \
     -F "version=1.0.0" \
     -F "app_file=@signed.ipa"
   ```

### Script example

```bash
#!/bin/bash
# re-sign and upload to Fenfa
set -e

CERT="/opt/signing/cert.p12"
PASS="your-certificate-password"
PROFILE="/opt/signing/adhoc.mobileprovision"
FENFA_HOST="https://your-domain.com"
FENFA_TOKEN="your-upload-token"
VARIANT_ID="var_xxx"

INPUT_IPA="$1"
VERSION="$2"

if [ -z "$INPUT_IPA" ] || [ -z "$VERSION" ]; then
  echo "Usage: $0 <input.ipa> <version>"
  exit 1
fi

SIGNED_IPA="/tmp/signed_$(date +%s).ipa"

echo "Re-signing $INPUT_IPA..."
zsign -k "$CERT" -p "$PASS" -m "$PROFILE" -o "$SIGNED_IPA" "$INPUT_IPA"

echo "Uploading to Fenfa..."
curl -X POST "$FENFA_HOST/upload" \
  -H "X-Auth-Token: $FENFA_TOKEN" \
  -F "variant_id=$VARIANT_ID" \
  -F "version=$VERSION" \
  -F "app_file=@$SIGNED_IPA"

rm -f "$SIGNED_IPA"
echo "Done."
```

### Using Apple API auto-registration

Fenfa can automatically register devices with Apple Developer Portal. Configure in admin panel → Settings → Apple Developer:

1. Enter **API Key ID**, **Issuer ID**, **Team ID**
2. Upload the **AuthKey .p8** file
3. Click **Test Connection**

Once configured, you can register devices directly from the UDID Devices page — individually or in batch. After registration, regenerate your provisioning profile to include the new devices.

## Enterprise distribution

For Enterprise (in-house) distribution, the provisioning profile covers **all devices** — no UDID registration needed. Simply sign the IPA with your Enterprise certificate and profile:

```bash
zsign -k enterprise_cert.p12 -p "password" -m enterprise.mobileprovision \
  -o signed.ipa original.ipa
```

Upload to Fenfa and any user can install directly without UDID binding.

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "Unable to install" | UDID not in provisioning profile | Register device, regenerate profile, re-sign |
| "Untrusted Developer" | Certificate not trusted on device | Settings → General → Device Management → Trust |
| zsign: "invalid p12" | Wrong password or corrupt file | Re-export certificate from Keychain |
| zsign: "provision not match" | Bundle ID mismatch | Use `-b` flag to override bundle ID |
| Profile expired | Provisioning profile past expiration | Regenerate in Apple Developer Portal |
