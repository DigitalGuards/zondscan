#!/bin/bash

# Print colored output
print_status() {
    echo -e "\e[1;34m>>> $1\e[0m"
}

print_error() {
    echo -e "\e[1;31m>>> Error: $1\e[0m"
    exit 1
}

# Clean PM2 logs and processes
clean_pm2() {
    print_status "Cleaning PM2 logs and processes..."
    
    # Delete all PM2 logs
    pm2 flush || print_status "No logs to flush"
    
    # Stop and delete all processes
    pm2 delete all || print_status "No processes to delete"
    
    # Clear PM2 dump file
    pm2 cleardump || print_status "No dump file to clear"
}

# Check for required tools
check_dependencies() {
    print_status "Checking dependencies..."

    command -v node >/dev/null 2>&1 || { print_error "Node.js is required but not installed."; }
    command -v npm >/dev/null 2>&1 || { print_error "npm is required but not installed."; }
    command -v go >/dev/null 2>&1 || { print_error "Go is required but not installed."; }
    command -v mongod >/dev/null 2>&1 || { print_error "MongoDB is required but not installed."; }

    # Install PM2 if not present
    if ! command -v pm2 >/dev/null 2>&1; then
        print_status "Installing PM2..."
        npm install -g pm2 || print_error "Failed to install PM2"
    fi
}

# Check if MongoDB is running
check_mongodb() {
    if ! nc -z localhost 27017; then
        print_error "MongoDB is not running on localhost:27017."
    fi
}

# Check if Zond node is accessible
check_zond_node() {
    RESPONSE=$(curl --silent --fail -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","id":1,"method":"net_listening","params":[]}' \
        http://127.0.0.1:8545)

    if [[ $? -ne 0 || -z "$RESPONSE" ]]; then
        print_error "Zond node is not accessible at http://127.0.0.1:8545."
    fi
}

# Check if port is available
check_port() {
    PORT=$1
    if lsof -i:$PORT -t >/dev/null; then
        print_error "Port $PORT is already in use."
    fi
}

# Clone the repository
clone_repo() {
    print_status "Cloning QRL Explorer repository..."
    git clone https://github.com/moscowchill/qrl-explorer-pos.git || print_error "Failed to clone repository"
    cd qrl-explorer-pos || print_error "Failed to enter project directory"
    export BASE_DIR=$(pwd)
}

# Setup server environment
setup_server() {
    print_status "Setting up server..."
    cd "$BASE_DIR/quanta-explorer-go/server" || print_error "Server directory not found"

    # Build the server
    print_status "Building server..."
    go build main.go || print_error "Failed to build server"

    # Start server with PM2, specifying the working directory and APP_ENV
    print_status "Starting server with PM2..."
    APP_ENV=development pm2 start ./main --name "handler" --cwd "$BASE_DIR/quanta-explorer-go/server" || print_error "Failed to start server"
}

# Setup frontend environment
setup_frontend() {
    print_status "Setting up frontend..."
    cd "$BASE_DIR/quanta-explorer-go/frontend" || print_error "Frontend directory not found"

    # Create .env file
    cat > .env << EOL
DATABASE_URL=mongodb://localhost:27017/qrldata?readPreference=primary
NEXTAUTH_URL=127.0.0.1
NEXT_PUBLIC_DOMAIN_NAME=http://localhost:3000
NEXT_PUBLIC_HANDLER_URL=http://127.0.0.1:8080
EOL

    # Create .env.local file
    cat > .env.local << EOL
DATABASE_URL=mongodb://localhost:27017/qrldata?readPreference=primary
NEXTAUTH_SECRET=development_secret
ADMIN_PUBLIC_ADDRESS=development_address
DOMAIN_NAME=http://localhost:3000
HANDLER_URL=http://127.0.0.1:8080
EOL

    # Install dependencies
    print_status "Installing frontend dependencies..."
    npm install || print_error "Failed to install frontend dependencies"

    # Update browserslist database
    print_status "Updating browserslist database..."
    npx browserslist@latest --update-db || print_error "Failed to update browserslist"

    # Start frontend in development mode with PM2
    print_status "Starting frontend in development mode..."
    cd "$BASE_DIR/quanta-explorer-go/frontend" && pm2 start "npm run dev" --name "frontend" || print_error "Failed to start frontend"
}

# Setup blockchain synchronizer
setup_synchronizer() {
    print_status "Setting up blockchain synchronizer..."
    cd "$BASE_DIR/QRLtoMongoDB-PoS" || print_error "Synchronizer directory not found"

    # Create .env file
    cat > .env << EOL
MONGOURI=mongodb://localhost:27017
NODE_URL=http://127.0.0.1:8545
EOL

    # Build and start synchronizer
    print_status "Building synchronizer..."
    go build main.go || print_error "Failed to build synchronizer"

    # Start synchronizer with PM2, specifying the working directory
    print_status "Starting synchronizer with PM2..."
    pm2 start ./main --name "synchroniser" --cwd "$BASE_DIR/QRLtoMongoDB-PoS" || print_error "Failed to start synchronizer"
}

# Save PM2 processes
save_pm2() {
    print_status "Saving PM2 processes..."
    pm2 save || print_error "Failed to save PM2 processes"
    print_status "Generating PM2 startup script..."
    pm2 startup systemd -u $USER --hp $HOME || print_error "Failed to generate PM2 startup script"
}

# Main deployment function
main() {
    print_status "Starting QRL Explorer deployment..."

    # Clean PM2 logs and processes before starting
    clean_pm2

    # Check for required tools
    check_dependencies

    # Check if MongoDB and Zond node are running
    check_mongodb
    check_zond_node

    # Check if required ports are available
    check_port 3000
    check_port 8080

    # Create a clean working directory
    rm -rf qrl-explorer-pos

    # Clone and setup
    clone_repo
    setup_server        # Start the server before building the frontend
    setup_frontend
    setup_synchronizer
    save_pm2

    print_status "Deployment complete! Services are starting up..."
    echo -e "\nAccess points:"
    echo "- Frontend: http://localhost:3000"
    echo "- Server API: http://localhost:8080"
    echo -e "\nMake sure you have:"
    echo "1. MongoDB running on localhost:27017"
    echo "2. Zond node accessible at http://127.0.0.1:8545"
    echo -e "\nTo monitor services:"
    echo "pm2 status"
    echo -e "\nTo view logs:"
    echo "pm2 logs"
    echo -e "\nTo clear logs:"
    echo "pm2 flush"
    echo -e "\nTo stop all services:"
    echo "pm2 stop all"
}

# Run the deployment
main
