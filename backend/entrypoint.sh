#!/bin/sh
# Runs the server as PUID:PGID (default 1000:1000) so cards.json in the
# ./config bind mount stays owned by, and editable for, the host user.
set -e

if [ "$(id -u)" = "0" ]; then
    PUID="${PUID:-1000}"
    PGID="${PGID:-1000}"
    mkdir -p /cache /config
    # The cache volume may still belong to a previous uid (e.g. 65532).
    if [ "$(stat -c %u /cache)" != "$PUID" ]; then
        chown -R "$PUID:$PGID" /cache
    fi
    if [ "$(stat -c %u /config)" != "$PUID" ]; then
        chown "$PUID:$PGID" /config
    fi
    exec su-exec "$PUID:$PGID" /app/server "$@"
fi

exec /app/server "$@"
