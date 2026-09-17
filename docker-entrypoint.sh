#!/bin/sh
set -e

# Start the Go API in the background. Nothing but JSON may reach stdout, so
# there are no progress echoes here — the API logs "server starting" itself and
# Caddy logs its own startup.
/usr/local/bin/contact-api &

# Give the API a moment to start
sleep 1

# Start Caddy in the foreground
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
