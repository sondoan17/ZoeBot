#!/bin/bash
# ZoeBot Docker Installation Script
# Install location: /opt/zoebot
# Usage: curl -fsSL <url> | bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="/opt/zoebot"
REPO_URL="https://github.com/YOUR_USERNAME/zoebot.git"  # <-- Change this

echo "========================================"
echo "  ZoeBot Docker Installation Script"
echo "========================================"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root${NC}"
    exit 1
fi

# Step 1: Update system
echo -e "${YELLOW}[1/6] Updating system...${NC}"
apt update && apt upgrade -y

# Step 2: Install Docker
echo -e "${YELLOW}[2/6] Installing Docker...${NC}"
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo -e "${GREEN}Docker installed${NC}"
else
    echo -e "${GREEN}Docker already installed${NC}"
fi

# Step 3: Install Git
echo -e "${YELLOW}[3/6] Installing Git...${NC}"
if ! command -v git &> /dev/null; then
    apt install -y git
fi

# Step 4: Clone repository
echo -e "${YELLOW}[4/6] Cloning repository...${NC}"
if [ -d "$INSTALL_DIR" ]; then
    echo "Directory exists, pulling latest..."
    cd "$INSTALL_DIR/zoebot_golang"
    git pull
else
    git clone "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR/zoebot_golang"

# Step 5: Setup .env
echo -e "${YELLOW}[5/6] Setting up environment...${NC}"
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo -e "${YELLOW}"
    echo "========================================"
    echo "  IMPORTANT: Configure .env file"
    echo "========================================"
    echo ""
    echo "Edit the .env file with your credentials:"
    echo "  nano $INSTALL_DIR/zoebot_golang/.env"
    echo ""
    echo "Required variables:"
    echo "  - DISCORD_TOKEN"
    echo "  - RIOT_API_KEY"
    echo "  - CLIPROXY_API_KEY"
    echo "  - CLIPROXY_API_URL"
    echo "  - CLIPROXY_MODEL"
    echo ""
    echo "Optional (for persistence):"
    echo "  - UPSTASH_REDIS_REST_URL"
    echo "  - UPSTASH_REDIS_REST_TOKEN"
    echo ""
    echo -e "${NC}"
    echo "After editing .env, run:"
    echo "  cd $INSTALL_DIR/zoebot_golang && docker compose up -d"
    exit 0
fi

# Step 6: Build and run
echo -e "${YELLOW}[6/6] Building and starting container...${NC}"
docker compose up -d --build

# Wait for container
sleep 5

# Check status
if docker compose ps | grep -q "running"; then
    echo ""
    echo -e "${GREEN}========================================"
    echo "  ZoeBot is running!"
    echo "========================================${NC}"
    echo ""
    echo "Useful commands:"
    echo "  cd $INSTALL_DIR/zoebot_golang"
    echo "  docker compose logs -f      # View logs"
    echo "  docker compose restart      # Restart"
    echo "  docker compose down         # Stop"
    echo "  docker stats zoebot         # Resource usage"
    echo ""
    echo "Health check:"
    curl -s http://localhost:8080/health && echo ""
else
    echo -e "${RED}Failed to start. Check logs:${NC}"
    docker compose logs
    exit 1
fi
