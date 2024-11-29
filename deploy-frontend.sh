#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Starting frontend deployment...${NC}"

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

# Navigate to frontend directory
echo -e "${YELLOW}Navigating to frontend directory...${NC}"
cd quanta-explorer-go/frontend
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Could not find frontend directory${NC}"
    exit 1
fi

# Install dependencies
echo -e "${YELLOW}Installing dependencies...${NC}"
npm install
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: npm install failed${NC}"
    exit 1
fi

# Build the frontend
echo -e "${YELLOW}Building frontend...${NC}"
npm run build
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Frontend build failed${NC}"
    exit 1
fi

# Stop existing PM2 process if it exists
pm2 stop frontend 2>/dev/null
pm2 delete frontend 2>/dev/null

# Start frontend with PM2
echo -e "${YELLOW}Starting frontend with PM2...${NC}"
pm2 start npm --name frontend -- start
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to start frontend with PM2${NC}"
    exit 1
fi

# Save PM2 configuration
pm2 save

echo -e "${GREEN}Frontend deployment completed successfully!${NC}"
echo -e "${YELLOW}PM2 process status:${NC}"
pm2 list

# Return to original directory
cd ../../
