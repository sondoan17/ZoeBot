#!/bin/bash
# ZoeBot Update Script
# Usage: chmod +x update.sh && ./update.sh

set -e

INSTALL_DIR="$HOME/zoebot/zoebot_golang"

echo "🔄 Updating ZoeBot..."

cd "$INSTALL_DIR"

# Pull latest changes (if using git)
if [ -d ".git" ]; then
    echo "📥 Pulling latest changes..."
    git pull
fi

# Rebuild
echo "🔨 Rebuilding..."
go mod tidy
go build -ldflags="-w -s" -o zoebot ./cmd/zoebot

# Restart service
echo "🔄 Restarting service..."
sudo systemctl restart zoebot

# Wait and check
sleep 3
if sudo systemctl is-active --quiet zoebot; then
    echo "✅ ZoeBot updated and running!"
else
    echo "❌ Failed to restart. Check logs:"
    sudo journalctl -u zoebot -n 20
fi
