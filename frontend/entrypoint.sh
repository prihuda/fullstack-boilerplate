#!/bin/sh
set -e

# Replace __VITE_*__ placeholders in built JS files with runtime env vars.
# Add more VITE_ variables here as needed.

JS_DIR="/usr/share/nginx/html/assets"

if [ -d "$JS_DIR" ]; then
    for var in VITE_API_URL; do
        eval value=\$$var
        if [ -n "$value" ]; then
            sed -i "s|__${var}__|${value}|g" "$JS_DIR"/*.js
        fi
    done
fi

exec nginx -g 'daemon off;'
