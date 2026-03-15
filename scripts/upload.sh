#!/bin/bash
# upload.sh — Upload build artifacts to Fenfa distribution platform
# Usage: bash scripts/upload.sh [android|ios|all]
#   No argument defaults to "all" (Android + iOS)
#
# Environment variables:
#   FENFA_HOST    — Upload URL (default: http://localhost:8000)
#   FENFA_TOKEN   — Auth token (default: dev-upload-token)
#   CHANGELOG     — Release notes
#   CHANNEL       — Channel (default: internal)
set -e

cd "$(dirname "$0")/.."

TARGET="${1:-all}"
HOST="${FENFA_HOST:-http://localhost:8000}"
TOKEN="${FENFA_TOKEN:-dev-upload-token}"
CHANNEL="${CHANNEL:-internal}"
ENDPOINT="$HOST/smart-upload"

APK="build/app/outputs/flutter-apk/app-release.apk"
IPA=$(find build/ios/ipa -name "*.ipa" 2>/dev/null | head -1)

upload_file() {
    local file="$1"
    local name=$(basename "$file")
    local size=$(du -h "$file" | cut -f1)

    echo "  Uploading: $name ($size)"

    local args=(-s -w "\n%{http_code}" -X POST "$ENDPOINT")
    args+=(-H "X-Auth-Token: $TOKEN")
    args+=(-F "app_file=@$file")
    [ -n "$CHANNEL" ]   && args+=(-F "channel=$CHANNEL")
    [ -n "$CHANGELOG" ] && args+=(-F "changelog=$CHANGELOG")

    local response
    response=$(curl "${args[@]}")
    local http_code=$(echo "$response" | tail -1)
    local body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "201" ]; then
        local page=$(echo "$body" | grep -o '"page":"[^"]*"' | head -1 | cut -d'"' -f4)
        echo "  Done -> $page"
    else
        echo "  Failed (HTTP $http_code)"
        echo "$body"
        return 1
    fi
}

echo "Fenfa Upload ($ENDPOINT)"
echo ""

UPLOADED=0

# Android
if [ "$TARGET" = "android" ] || [ "$TARGET" = "all" ]; then
    if [ -f "$APK" ]; then
        echo "[Android]"
        upload_file "$APK"
        UPLOADED=$((UPLOADED + 1))
        echo ""
    else
        echo "[Android] APK not found: $APK"
        [ "$TARGET" = "android" ] && exit 1
    fi
fi

# iOS
if [ "$TARGET" = "ios" ] || [ "$TARGET" = "all" ]; then
    if [ -n "$IPA" ] && [ -f "$IPA" ]; then
        echo "[iOS]"
        upload_file "$IPA"
        UPLOADED=$((UPLOADED + 1))
        echo ""
    else
        echo "[iOS] IPA not found"
        [ "$TARGET" = "ios" ] && exit 1
    fi
fi

if [ "$UPLOADED" -eq 0 ]; then
    echo "No build artifacts found. Run your build first."
    exit 1
fi

echo "Upload complete ($UPLOADED file(s))"
