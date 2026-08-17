#!/bin/sh
set -eu

DEFAULT_UID="$(id -u nowen)"
DEFAULT_GID="$(id -g nowen)"
PUID="${PUID:-$DEFAULT_UID}"
PGID="${PGID:-$DEFAULT_GID}"

case "$PUID" in
  ""|*[!0-9]*)
    echo "Invalid PUID: $PUID" >&2
    exit 64
    ;;
esac

case "$PGID" in
  ""|*[!0-9]*)
    echo "Invalid PGID: $PGID" >&2
    exit 64
    ;;
esac

# Root mode keeps the historical behavior and does not need ownership remapping.
if [ "$PUID" = "0" ]; then
  exec nowen-video
fi

# Use numeric ownership instead of recreating users/groups at runtime. Host
# PUID/PGID values may collide with existing Alpine system accounts (for
# example GID 10), but numeric ownership and su-exec uid:gid remain valid.
chown -R "$PUID:$PGID" /data /cache 2>/dev/null || true
chown "$PUID:$PGID" /media 2>/dev/null || true

# The compatibility image contains the legacy Python scraper; the production
# image does not. Keep one entrypoint for both images without coupling the
# production image to that optional directory.
if [ -d /app/scripts ]; then
  chown -R "$PUID:$PGID" /app/scripts 2>/dev/null || true
fi

exec su-exec "$PUID:$PGID" nowen-video
