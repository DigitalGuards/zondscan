#!/bin/bash

# Print colored output
print_status() {
    echo -e "\e[1;34m>>> $1\e[0m"
}

print_error() {
    echo -e "\e[1;31m>>> Error: $1\e[0m"
    exit 1
}

# Directory constants
FRONTEND_DIR="quanta-explorer-go/frontend"

# Check if we're in the right directory
if [ ! -d "$FRONTEND_DIR" ]; then
    print_error "Frontend directory not found. Make sure you're in the project root."
fi

# Update from git
print_status "Pulling latest changes from git..."
git pull || print_error "Failed to pull from git"

# Navigate to frontend directory
cd "$FRONTEND_DIR" || print_error "Failed to enter frontend directory"

# Install any new dependencies
print_status "Installing dependencies..."
npm install || print_error "Failed to install dependencies"

# Update browserslist database to avoid warnings
print_status "Updating browserslist database..."
npx browserslist@latest --update-db || print_status "Failed to update browserslist, continuing anyway..."

# Restart the frontend service
print_status "Restarting frontend service..."
pm2 delete frontend 2>/dev/null || true  # Delete if exists, ignore errors
pm2 start "npm run dev" --name "frontend" || print_error "Failed to start frontend"

print_status "Frontend update complete!"
echo -e "\nYou can:"
echo "- View logs: pm2 logs frontend"
echo "- Check status: pm2 status"
echo "- Monitor: pm2 monit"
