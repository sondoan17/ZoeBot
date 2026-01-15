#!/bin/bash
# ZoeBot Docker Installation Script for Ubuntu VPS
# Usage: chmod +x docker-install.sh && ./docker-install.sh

set -e

echo "🐳 ZoeBot Docker Installation Script"
echo "====================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="$HOME/zoebot/zoebot_golang"

# Step 1: Update system
echo -e "${YELLOW}📦 Updating system...${NC}"
sudo apt update && sudo apt upgrade -y

# Step 2: Install Docker
echo -e "${YELLOW}🐳 Installing Docker...${NC}"
if ! command -v docker &> /dev/null; then
    # Install prerequisites
    sudo apt install -y ca-certificates curl gnupg lsb-release

    # Add Docker's official GPG key
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg

    # Add Docker repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    # Install Docker
    sudo apt update
    sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Add current user to docker group
    sudo usermod -aG docker $USER
    
    echo -e "${GREEN}✅ Docker installed${NC}"
    echo -e "${YELLOW}⚠️ Please logout and login again to use Docker without sudo${NC}"
else
    echo -e "${GREEN}✅ Docker already installed: $(docker --version)${NC}"
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
        echo ""
        echo "After editing .env, run:"
        echo "   cd $INSTALL_DIR && docker compose up -d"
        exit 0
    else
        echo -e "${RED}❌ No .env or .env.example found${NC}"
        exit 1
    fi
fi

# Step 5: Build and run with Docker Compose
echo -e "${YELLOW}🔨 Building Docker image...${NC}"
docker compose build

echo -e "${YELLOW}🚀 Starting ZoeBot container...${NC}"
docker compose up -d

# Wait for container to start
sleep 5

# Check if running
if docker compose ps | grep -q "running"; then
    echo -e "${GREEN}✅ ZoeBot is running!${NC}"
    echo ""
    echo "=============================="
    echo -e "${GREEN}🎉 Installation Complete!${NC}"
    echo "=============================="
    echo ""
    echo "Useful commands:"
    echo "  View logs:      docker compose logs -f"
    echo "  Status:         docker compose ps"
    echo "  Restart:        docker compose restart"
    echo "  Stop:           docker compose down"
    echo "  Rebuild:        docker compose up -d --build"
    echo ""
    echo "Health check:     curl http://localhost:8080/health"
else
    echo -e "${RED}❌ ZoeBot failed to start${NC}"
    echo "Check logs: docker compose logs"
    exit 1
fi
