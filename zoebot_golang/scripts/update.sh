#!/bin/bash
# ZoeBot Update Script
# Usage: /opt/zoebot/zoebot_golang/scripts/update.sh

set -e

INSTALL_DIR="/opt/zoebot/zoebot_golang"

echo "Updating ZoeBot..."

cd "$INSTALL_DIR"

# Pull latest
git pull

# Rebuild and restart
docker compose up -d --build

sleep 5

if docker compose ps | grep -q "running"; then
    echo "Update complete!"
    docker compose logs --tail=10
else
    echo "Update failed!"
    docker compose logs
    exit 1
fi
