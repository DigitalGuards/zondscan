#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Starting frontend update and deployment...${NC}"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if required commands exist
if ! command_exists npm; then
    echo -e "${RED}Error: npm is not installed${NC}"
    exit 1
fi

if ! command_exists pm2; then
    echo -e "${RED}Error: PM2 is not installed${NC}"
    exit 1
fi

if ! command_exists git; then
    echo -e "${RED}Error: Git is not installed${NC}"
    exit 1
fi

# Update from git
echo -e "${YELLOW}Pulling latest changes from git...${NC}"
git pull
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Git pull failed${NC}"
    exit 1
fi
echo -e "${GREEN}Git pull completed successfully${NC}"

# Navigate to frontend directory
echo -e "${YELLOW}Navigating to frontend directory...${NC}"
cd ExplorerFrontend
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Could not find ExplorerFrontend directory${NC}"
    exit 1
fi

# Clean install dependencies (optional)
read -p "Do you want to clean install dependencies? (y/n): " clean_install
if [[ "$clean_install" =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Clean installing dependencies...${NC}"
    rm -rf node_modules package-lock.json
    npm install
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error: npm install failed${NC}"
        exit 1
    fi
    echo -e "${GREEN}Dependencies installed successfully${NC}"
else
    echo -e "${YELLOW}Skipping clean install of dependencies${NC}"
fi

# Build the frontend
echo -e "${YELLOW}Building frontend...${NC}"
npm run build
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Frontend build failed${NC}"
    exit 1
fi

# Stop and delete existing PM2 process if it exists
echo -e "${YELLOW}Stopping existing PM2 process...${NC}"
pm2 describe frontend > /dev/null
if [ $? -eq 0 ]; then
    echo -e "${YELLOW}Stopping and removing existing frontend process...${NC}"
    pm2 stop frontend
    pm2 delete frontend
fi

# Start frontend with PM2
echo -e "${YELLOW}Starting frontend with PM2...${NC}"
pm2 start npm --name "frontend" -- start
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to start frontend with PM2${NC}"
    exit 1
fi

# Save PM2 configuration
echo -e "${YELLOW}Saving PM2 configuration...${NC}"
pm2 save
if [ $? -ne 0 ]; then
    echo -e "${RED}Warning: Failed to save PM2 configuration${NC}"
fi

echo -e "${GREEN}Frontend update and deployment completed successfully!${NC}"
echo -e "${YELLOW}PM2 process status:${NC}"
pm2 list

# Return to original directory
cd ..
