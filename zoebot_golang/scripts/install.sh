#!/bin/bash
# ZoeBot Installation Script for Ubuntu VPS
# Usage: chmod +x install.sh && ./install.sh

set -e

echo "🚀 ZoeBot Installation Script"
echo "=============================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Variables
GO_VERSION="1.22.0"
INSTALL_DIR="$HOME/zoebot/zoebot_golang"
SERVICE_NAME="zoebot"

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${RED}❌ Please do not run as root. Run as normal user.${NC}"
    exit 1
fi

# Step 1: Update system
echo -e "${YELLOW}📦 Updating system...${NC}"
sudo apt update && sudo apt upgrade -y

# Step 2: Install Go
echo -e "${YELLOW}🔧 Installing Go ${GO_VERSION}...${NC}"
if ! command -v go &> /dev/null; then
    wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    rm go${GO_VERSION}.linux-amd64.tar.gz
    
    # Add to PATH
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    export PATH=$PATH:/usr/local/go/bin
    
    echo -e "${GREEN}✅ Go installed: $(go version)${NC}"
else
    echo -e "${GREEN}✅ Go already installed: $(go version)${NC}"
fi

# Step 3: Check if project exists
if [ ! -d "$INSTALL_DIR" ]; then
    echo -e "${RED}❌ Project not found at $INSTALL_DIR${NC}"
    echo "Please upload your project first."
    exit 1
fi

cd "$INSTALL_DIR"

# Step 4: Check .env file
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo -e "${YELLOW}⚠️ Created .env from .env.example${NC}"
        echo -e "${YELLOW}⚠️ Please edit .env with your credentials:${NC}"
        echo "   nano $INSTALL_DIR/.env"
        exit 1
    else
        echo -e "${RED}❌ No .env or .env.example found${NC}"
        exit 1
    fi
fi

# Step 5: Build
echo -e "${YELLOW}🔨 Building ZoeBot...${NC}"
go mod tidy
go build -ldflags="-w -s" -o zoebot ./cmd/zoebot
echo -e "${GREEN}✅ Build successful${NC}"

# Step 6: Setup systemd service
echo -e "${YELLOW}⚙️ Setting up systemd service...${NC}"

# Get current user
CURRENT_USER=$(whoami)

# Create service file
cat > /tmp/zoebot.service << EOF
[Unit]
Description=ZoeBot Discord Bot
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/zoebot
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
EnvironmentFile=${INSTALL_DIR}/.env
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

sudo mv /tmp/zoebot.service /etc/systemd/system/zoebot.service
sudo systemctl daemon-reload
sudo systemctl enable zoebot

echo -e "${GREEN}✅ Systemd service configured${NC}"

# Step 7: Start service
echo -e "${YELLOW}🚀 Starting ZoeBot...${NC}"
sudo systemctl restart zoebot

# Wait a moment
sleep 3

# Check status
if sudo systemctl is-active --quiet zoebot; then
    echo -e "${GREEN}✅ ZoeBot is running!${NC}"
    echo ""
    echo "=============================="
    echo -e "${GREEN}🎉 Installation Complete!${NC}"
    echo "=============================="
    echo ""
    echo "Useful commands:"
    echo "  View logs:    sudo journalctl -u zoebot -f"
    echo "  Status:       sudo systemctl status zoebot"
    echo "  Restart:      sudo systemctl restart zoebot"
    echo "  Stop:         sudo systemctl stop zoebot"
    echo ""
    echo "Health check:   curl http://localhost:8080/health"
else
    echo -e "${RED}❌ ZoeBot failed to start${NC}"
    echo "Check logs: sudo journalctl -u zoebot -n 50"
    exit 1
fi
