#!/bin/bash

# Print colored output
print_status() {
    echo -e "\e[1;34m>>> $1\e[0m"
}

print_error() {
    echo -e "\e[1;31m>>> Error: $1\e[0m"
    exit 1
}

# Check for required tools
check_dependencies() {
    print_status "Checking dependencies..."
    command -v node >/dev/null 2>&1 || { print_error "Node.js is required but not installed."; }
    command -v npm >/dev/null 2>&1 || { print_error "npm is required but not installed."; }
}

# Clone or update repository
handle_repository() {
    if [ -d ".git" ]; then
        print_status "Repository already exists. Checking git status..."
        git status
        
        read -p "Would you like to pull the latest changes? (y/n): " PULL_CHANGES
        if [[ $PULL_CHANGES =~ ^[Yy]$ ]]; then
            print_status "Pulling latest changes..."
            git pull || print_error "Failed to pull latest changes"
        else
            print_status "Skipping pull, continuing with existing code..."
        fi
    else
        print_status "Cloning QRL Explorer repository..."
        git clone https://github.com/DigitalGuards/zondexplorer.git || print_error "Failed to clone repository"
        cd zondexplorer || print_error "Failed to enter project directory"
    fi
}

# Setup frontend environment
setup_frontend() {
    print_status "Setting up frontend..."
    cd ExplorerFrontend || print_error "Frontend directory not found"

    # Create .env file
    print_status "Creating .env file..."
    cat > .env << EOL
DATABASE_URL=mongodb://localhost:27017/qrldata?readPreference=primary
NEXT_PUBLIC_DOMAIN_NAME=http://localhost:3000
NEXT_PUBLIC_HANDLER_URL=http://127.0.0.1:8080
EOL

    # Create .env.local file
    print_status "Creating .env.local file..."
    cat > .env.local << EOL
DATABASE_URL=mongodb://localhost:27017/qrldata?readPreference=primary
DOMAIN_NAME=http://localhost:3000
HANDLER_URL=http://127.0.0.1:8080
EOL

    # Install dependencies
    print_status "Installing frontend dependencies..."
    npm install || print_error "Failed to install frontend dependencies"

    # Update browserslist database
    print_status "Updating browserslist database..."
    npx browserslist@latest --update-db || print_status "Browserslist update skipped"
}

# Main deployment function
main() {
    check_dependencies

    # Handle repository
    handle_repository

    # Setup frontend environment
    setup_frontend

    # Ask user for deployment mode
    echo "Please select deployment mode:"
    echo "1) Development (npm run dev)"
    echo "2) Production build (npm run build)"
    echo "3) Production start (npm run start)"
    read -p "Enter your choice (1-3): " DEPLOY_MODE

    case $DEPLOY_MODE in
        1)
            print_status "Starting development server..."
            npm run dev
            ;;
        2)
            print_status "Building for production..."
            npm run build
            ;;
        3)
            print_status "Starting production server..."
            npm run build && npm run start
            ;;
        *)
            print_error "Invalid choice"
            ;;
    esac
}

# Run the deployment
main
