#!/bin/sh
set -eu

# Run fenfa container with volumes mounted
# Env:
#   IMAGE        - image reference (default: fenfa/fenfa:latest)
#   NAME         - container name (default: fenfa)
#   PORT         - host port to map to 8000 (default: 8000)
#   BIND         - bind address (default: 127.0.0.1)
#   DATA_DIR     - host path for database directory (default: "$PWD/data")
#   UPLOADS_DIR  - host path for uploads directory (default: "$PWD/uploads")
#   CONFIG_PATH  - host path to config.json (default: "$PWD/config.json")

IMAGE=${IMAGE:-fenfa/fenfa:latest}
NAME=${NAME:-fenfa}
PORT=${PORT:-8000}
BIND=${BIND:-127.0.0.1}
DATA_DIR=${DATA_DIR:-"$PWD/data"}
UPLOADS_DIR=${UPLOADS_DIR:-"$PWD/uploads"}
CONFIG_PATH=${CONFIG_PATH:-"$PWD/config.json"}

mkdir -p "$DATA_DIR" "$UPLOADS_DIR"

# If config.json does not exist, create a minimal one
if [ ! -f "$CONFIG_PATH" ]; then
  cat > "$CONFIG_PATH" <<EOF
{
  "server": {
    "port": "8000",
    "data_dir": "/data",
    "db_path": "/data/fenfa.db"
  },
  "auth": {
    "admin_tokens": ["change-me"],
    "upload_tokens": ["change-me"]
  }
}
EOF
  echo "Created default config at $CONFIG_PATH — please edit tokens before use"
fi

# Stop and remove existing container if running
docker stop "$NAME" 2>/dev/null || true
docker rm "$NAME" 2>/dev/null || true

exec docker run -d \
  --name "$NAME" \
  --restart=unless-stopped \
  -p "$BIND:$PORT:8000" \
  -v "$DATA_DIR:/data" \
  -v "$UPLOADS_DIR:/app/uploads" \
  -v "$CONFIG_PATH:/app/config.json:ro" \
  "$IMAGE"
