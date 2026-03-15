#!/bin/sh
set -e

# Ensure data and upload directories are writable by appuser (uid 10001)
for dir in /data /app/uploads; do
  if [ -d "$dir" ] && [ "$(stat -c %u "$dir" 2>/dev/null || stat -f %u "$dir")" != "10001" ]; then
    chown -R 10001:10001 "$dir" 2>/dev/null || true
  fi
done

# Drop privileges and exec the server
exec su-exec appuser /app/fenfa "$@"
