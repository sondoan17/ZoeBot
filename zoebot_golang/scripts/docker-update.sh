#!/bin/bash
# ZoeBot Docker Update Script
# Usage: chmod +x docker-update.sh && ./docker-update.sh

set -e

INSTALL_DIR="$HOME/zoebot/zoebot_golang"

echo "🔄 Updating ZoeBot (Docker)..."

cd "$INSTALL_DIR"

# Pull latest changes (if using git)
if [ -d ".git" ]; then
    echo "📥 Pulling latest changes..."
    git pull
fi

# Rebuild and restart
echo "🔨 Rebuilding Docker image..."
docker compose build --no-cache

echo "🔄 Restarting container..."
docker compose up -d

# Wait and check
sleep 5
if docker compose ps | grep -q "running"; then
    echo "✅ ZoeBot updated and running!"
    docker compose logs --tail=20
else
    echo "❌ Failed to restart. Check logs:"
    docker compose logs
fi
