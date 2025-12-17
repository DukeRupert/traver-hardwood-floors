#!/bin/sh
set -e

# Start the Go API in the background
echo "Starting contact API..."
/usr/local/bin/contact-api &

# Give the API a moment to start
sleep 1

# Start Caddy in the foreground
echo "Starting Caddy..."
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
