#!/bin/bash

# Helper functions for status and error messages
print_status() {
    echo -e "\033[1;34m[*]\033[0m $1"
}

print_error() {
    echo -e "\033[1;31m[!]\033[0m $1" >&2
    exit 1
}
print_status "Deploying frontend is currently commented out uncomment the function or deploy it manually or with update-frontend.sh"
# Clean PM2 logs and processes
clean_pm2() {
    print_status "Cleaning PM2 logs and processes..."

    # Delete all PM2 logs
    pm2 flush || print_status "No logs to flush"

    # Stop and delete only processes started by this deployment
    for name in handler syncer synchroniser frontend; do
        pm2 delete $name || print_status "No process named $name to delete"
    done

    # Clear PM2 dump file
    pm2 cleardump || print_status "No dump file to clear"
}

# Clean MongoDB database and log files
clean_database_and_logs() {
    print_status "Do you want to clean MongoDB database and log files? (y/n)"
    read -p "Enter choice: " DELETE_DB_LOGS

    if [[ $DELETE_DB_LOGS =~ ^[Yy]$ ]]; then
        print_status "Cleaning MongoDB database and log files..."

        # Drop the MongoDB database
        mongosh --eval "db.getSiblingDB('qrldata-z').dropDatabase()" || print_status "Failed to drop database or database doesn't exist"

        # Delete the log file if it exists
        if [ -f "$BASE_DIR/QRL2MongoDB/logs/zond_sync.log" ]; then
            rm "$BASE_DIR/QRL2MongoDB/logs/zond_sync.log" || print_status "Failed to delete log file"
        else
            print_status "Log file not found, skipping deletion"
        fi
    else
        print_status "Skipping database and log files cleanup"
    fi
}

# Check for required tools
check_dependencies() {
    print_status "Checking dependencies..."

    command -v node >/dev/null 2>&1 || { print_error "Node.js is required but not installed."; }
    command -v npm >/dev/null 2>&1 || { print_error "npm is required but not installed."; }
    command -v go >/dev/null 2>&1 || { print_error "Go is required but not installed."; }
    command -v mongod >/dev/null 2>&1 || { print_error "MongoDB is required but not installed."; }
    command -v nginx >/dev/null 2>&1 || { print_error "Nginx is required but not installed."; }

    # Install PM2 if not present
    if ! command -v pm2 >/dev/null 2>&1; then
        print_status "Installing PM2..."
        npm install -g pm2 || print_error "Failed to install PM2"
    fi
}

# Configure pm2-logrotate so logs can't fill the disk
setup_pm2_logrotate() {
    print_status "Configuring pm2-logrotate (daily, 3-day retention, 50M cap, gzip)..."

    if pm2 list 2>/dev/null | grep -q pm2-logrotate; then
        print_status "pm2-logrotate already installed"
    else
        pm2 install pm2-logrotate || print_error "Failed to install pm2-logrotate"
    fi

    pm2 set pm2-logrotate:retain 3
    pm2 set pm2-logrotate:max_size 50M
    pm2 set pm2-logrotate:compress true
    pm2 set pm2-logrotate:rotateInterval '0 0 * * *'
    pm2 set pm2-logrotate:dateFormat YYYY-MM-DD_HH-mm-ss
    pm2 set pm2-logrotate:rotateModule true
}

# Prompt for node selection
select_node() {
    print_status "Select Zond node to use:"
    PS3="Please choose the node (1-3): "
    options=("Local node (127.0.0.1:8545)" "Testnet Remote node (qrlwallet.com)" "Custom node (enter URL manually)")
    select opt in "${options[@]}"
    do
        case $opt in
            "Local node (127.0.0.1:8545)")
                NODE_URL="http://127.0.0.1:8545"
                break
                ;;
            "Testnet Remote node (qrlwallet.com)")
                NODE_URL="https://qrlwallet.com/api/qrl-rpc/testnet"
                break
                ;;
            "Custom node (enter URL manually)")
                while true; do
                    read -p "Enter custom node URL (e.g., http://192.168.1.100:8545): " CUSTOM_NODE_URL

                    # Basic validation: check if URL starts with http:// or https://
                    if [[ $CUSTOM_NODE_URL =~ ^https?:// ]]; then
                        NODE_URL="$CUSTOM_NODE_URL"
                        print_status "Custom node URL set to: $NODE_URL"
                        break 2
                    else
                        print_error "Invalid URL format. Please use http:// or https:// prefix."
                        read -p "Try again? (y/n): " TRY_AGAIN
                        if [[ ! $TRY_AGAIN =~ ^[Yy]$ ]]; then
                            print_error "Node URL is required. Exiting..."
                        fi
                    fi
                done
                ;;
            *) echo "Invalid option. Please try again.";;
        esac
    done
    print_status "Selected node: $NODE_URL"
    export NODE_URL
}

# Check if Zond node is accessible
check_zond_node() {
    RESPONSE=$(curl --silent --fail -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","id":1,"method":"net_listening","params":[]}' \
        $NODE_URL)

    if [[ $? -ne 0 || -z "$RESPONSE" ]]; then
        print_error "Zond node is not accessible at $NODE_URL"
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
    if git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
        print_status "Repository already exists. Checking git status..."
        git status
    else
        print_status "Cloning QRL Explorer repository..."
        git clone https://github.com/DigitalGuards/zondscan.git || print_error "Failed to clone repository"
        cd zondscan || print_error "Failed to enter project directory"
    fi

    export BASE_DIR=$(pwd)
}

# Prompt for branch selection (checkout + optional pull)
select_branch() {
    cd "$BASE_DIR" || print_error "BASE_DIR not set or invalid"

    print_status "Fetching latest refs from origin..."
    git fetch --all --prune 2>/dev/null || print_status "Fetch failed, continuing with cached refs"

    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    print_status "Current branch: $CURRENT_BRANCH"

    # Build deduped branch list from local + origin (skip the bare 'origin' HEAD pointer)
    mapfile -t BRANCHES < <(git for-each-ref --format='%(refname:short)' refs/heads refs/remotes/origin \
        | grep -vE '^origin(/HEAD)?$' \
        | sed 's|^origin/||' \
        | sort -u)

    print_status "Select branch to deploy:"
    PS3="Please choose a branch: "
    options=("Keep current ($CURRENT_BRANCH)" "${BRANCHES[@]}" "Custom (enter name manually)")
    select opt in "${options[@]}"
    do
        case $opt in
            "")
                echo "Invalid option. Please try again."
                ;;
            "Keep current ($CURRENT_BRANCH)")
                SELECTED_BRANCH="$CURRENT_BRANCH"
                break
                ;;
            "Custom (enter name manually)")
                read -p "Enter branch name: " SELECTED_BRANCH
                [ -z "$SELECTED_BRANCH" ] && print_error "Branch name required"
                break
                ;;
            *)
                SELECTED_BRANCH="$opt"
                break
                ;;
        esac
    done

    print_status "Selected branch: $SELECTED_BRANCH"

    # If switching branches and tree is dirty, offer to stash
    if [ "$SELECTED_BRANCH" != "$CURRENT_BRANCH" ] && [ -n "$(git status --porcelain)" ]; then
        print_status "Working tree has uncommitted changes:"
        git status --short
        read -p "Stash and continue? (y/n): " STASH_CHANGES
        if [[ $STASH_CHANGES =~ ^[Yy]$ ]]; then
            git stash push -m "deploy.sh auto-stash $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
                || print_error "Failed to stash changes"
        else
            print_error "Aborting, clean working tree first or stash manually"
        fi
    fi

    # Checkout target branch, track origin if no local branch exists yet
    if [ "$SELECTED_BRANCH" != "$CURRENT_BRANCH" ]; then
        if git show-ref --verify --quiet "refs/heads/$SELECTED_BRANCH"; then
            git checkout "$SELECTED_BRANCH" || print_error "Failed to checkout $SELECTED_BRANCH"
        elif git show-ref --verify --quiet "refs/remotes/origin/$SELECTED_BRANCH"; then
            git checkout -b "$SELECTED_BRANCH" "origin/$SELECTED_BRANCH" \
                || print_error "Failed to checkout origin/$SELECTED_BRANCH"
        else
            print_error "Branch '$SELECTED_BRANCH' not found locally or on origin"
        fi
    fi

    # Pull latest if branch tracks a remote
    if git rev-parse --abbrev-ref --symbolic-full-name '@{u}' >/dev/null 2>&1; then
        read -p "Pull latest changes for $SELECTED_BRANCH? (y/n): " PULL_LATEST
        if [[ $PULL_LATEST =~ ^[Yy]$ ]]; then
            git pull --ff-only || print_error "Failed to pull (non-fast-forward, resolve manually)"
        fi
    else
        print_status "Branch has no upstream, skipping pull"
    fi

    print_status "On branch $(git rev-parse --abbrev-ref HEAD) at $(git rev-parse --short HEAD)"
}

# Setup server environment
setup_backendapi() {
    print_status "Setting up server..."
    cd "$BASE_DIR/backendAPI" || print_error "Server directory not found"

    # Create .env file only if it doesn't exist
    if [ -f ".env" ]; then
        print_status ".env file already exists, skipping creation"
    else
        print_status "Creating .env file..."
        cat > .env << EOL
GIN_MODE=release
MONGOURI=mongodb://localhost:27017/qrldata-z?readPreference=primary
HTTP_PORT=:8081
NODE_URL=$NODE_URL
EOL
    fi

    # Build the server
    print_status "Building server..."
    go build -o backendAPI main.go || print_error "Failed to build server"

    # Start server with PM2, specifying the working directory and APP_ENV
    print_status "Starting server with PM2..."
    pm2 start ./backendAPI --name "handler" --cwd "$BASE_DIR/backendAPI" || print_error "Failed to start server"
}

# Setup nginx configuration
setup_nginx() {
    print_status "Setting up Nginx configuration..."

    read -p "Enter your domain name (e.g., zondscan.com): " DOMAIN_NAME
    read -p "Enter SSL certificate path (or press Enter to skip SSL setup): " SSL_CERT

    if [ -z "$SSL_CERT" ]; then
        print_status "Skipping SSL setup. Creating HTTP-only configuration..."

        cat > /etc/nginx/sites-available/$DOMAIN_NAME << EOL
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN_NAME www.$DOMAIN_NAME;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOL
    else
        read -p "Enter SSL certificate key path: " SSL_KEY

        cat > /etc/nginx/sites-available/$DOMAIN_NAME << EOL
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN_NAME www.$DOMAIN_NAME;
    return 301 https://\$server_name\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name $DOMAIN_NAME www.$DOMAIN_NAME;

    ssl_certificate $SSL_CERT;
    ssl_certificate_key $SSL_KEY;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384';

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOL
    fi

    # Enable the site
    ln -sf /etc/nginx/sites-available/$DOMAIN_NAME /etc/nginx/sites-enabled/$DOMAIN_NAME

    # Enable gzip compression in main nginx.conf if not already enabled
    if ! grep -q "gzip_vary on" /etc/nginx/nginx.conf; then
        print_status "Enabling gzip compression..."
        sed -i 's/# gzip_vary on;/gzip_vary on;/g' /etc/nginx/nginx.conf
        sed -i 's/# gzip_proxied any;/gzip_proxied any;/g' /etc/nginx/nginx.conf
        sed -i 's/# gzip_comp_level 6;/gzip_comp_level 6;/g' /etc/nginx/nginx.conf
        sed -i 's/# gzip_buffers 16 8k;/gzip_buffers 16 8k;/g' /etc/nginx/nginx.conf
        sed -i 's/# gzip_http_version 1.1;/gzip_http_version 1.1;/g' /etc/nginx/nginx.conf
        sed -i 's/# gzip_types text\/plain text\/css application\/json application\/javascript text\/xml application\/xml application\/xml+rss text\/javascript;/gzip_types text\/plain text\/css application\/json application\/javascript text\/xml application\/xml application\/xml+rss text\/javascript;/g' /etc/nginx/nginx.conf
    fi

    # Test and reload nginx
    nginx -t || print_error "Nginx configuration test failed"
    systemctl reload nginx || print_error "Failed to reload Nginx"

    print_status "Nginx configured successfully for $DOMAIN_NAME"
}

# Setup frontend environment
setup_frontend() {
    print_status "Setting up frontend..."
    cd "$BASE_DIR/ExplorerFrontend" || print_error "Frontend directory not found"

    # Determine the public URL for client-side API calls
    if [ -n "$DOMAIN_NAME" ]; then
        if [ -n "$SSL_CERT" ]; then
            PUBLIC_URL="https://$DOMAIN_NAME"
        else
            PUBLIC_URL="http://$DOMAIN_NAME"
        fi
    else
        PUBLIC_URL="http://localhost:3000"
    fi

    # Create .env file only if it doesn't exist
    if [ -f ".env" ]; then
        print_status ".env file already exists, skipping creation"
    else
        print_status "Creating .env file..."
        cat > .env << EOL
DATABASE_URL=mongodb://localhost:27017/qrldata-z?readPreference=primary
DOMAIN_NAME=$PUBLIC_URL
HANDLER_URL=http://127.0.0.1:8082
NEXT_PUBLIC_HANDLER_URL=$PUBLIC_URL/api
EOL
    fi

    # Install dependencies
    print_status "Installing frontend dependencies..."
    npm install || print_error "Failed to install frontend dependencies"

    # Update browserslist database
    print_status "Updating browserslist database..."
    npx update-browserslist-db@latest || print_status "Failed to update browserslist, continuing..."

    # Build production frontend
    print_status "Building production frontend..."
    npm run build || print_error "Failed to build frontend"

    # Start frontend in production mode with PM2
    print_status "Starting frontend in production mode..."
    cd "$BASE_DIR/ExplorerFrontend" && pm2 start "npm start" --name "frontend" || print_error "Failed to start frontend"
}

# Setup blockchain synchronizer
setup_synchronizer() {
    print_status "Setting up blockchain synchronizer..."
    cd "$BASE_DIR/QRL2MongoDB" || print_error "Synchronizer directory not found"

    # Create .env file only if it doesn't exist
    if [ -f ".env" ]; then
        print_status ".env file already exists, skipping creation"
    else
        print_status "Creating .env file..."
        cat > .env << EOL
MONGOURI=mongodb://localhost:27017
NODE_URL=$NODE_URL
BEACONCHAIN_API=http://91.99.92.138:3500
EOL
    fi

    # Build synchronizer
    print_status "Building synchronizer..."
    go build -o zsyncer main.go || print_error "Failed to build synchronizer"

    # Make the binary executable
    chmod +x ./zsyncer

    # Start synchronizer with PM2, explicitly setting environment variables
    print_status "Starting synchronizer with PM2..."
     pm2 start ./zsyncer --name "synchroniser" --cwd "$BASE_DIR/QRL2MongoDB" || print_error "Failed to start synchronizer"
}

# Save PM2 processes
save_pm2() {
    print_status "Saving PM2 processes..."
    pm2 save || print_error "Failed to save PM2 processes"
}

# Main deployment function
main() {
    print_status "Starting QRL Explorer deployment..."

    # Set BASE_DIR early for use by cleanup functions
    export BASE_DIR=$(pwd)

    # Clean PM2 logs and processes before starting
    clean_pm2

    # Clean database and log files
    clean_database_and_logs

    # Check for required tools
    check_dependencies

    # Ensure log rotation is in place before any processes start
    setup_pm2_logrotate

    # Prompt for node selection
    select_node

    # Check if MongoDB and Zond node are running
    # check_mongodb
    check_zond_node

    # Check if required ports are available
    check_port 3000
    check_port 8081

    # Clone and setup
    clone_repo

    # Pick which branch to deploy (after repo exists, before any builds)
    select_branch

    # Ask if user wants to setup nginx
    read -p "Do you want to setup Nginx reverse proxy? (y/n): " SETUP_NGINX
    if [[ $SETUP_NGINX =~ ^[Yy]$ ]]; then
        setup_nginx
    fi

    setup_frontend
    setup_synchronizer
    echo "Waiting for synchronizer to initialize..."
    for i in {10..1}; do
        echo -ne "\rStarting backend in $i seconds..."
        sleep 1
    done
    echo -e "\rSynchronizer initialized, starting backend..."
    setup_backendapi
    save_pm2

    print_status "Deployment complete! Services are starting up..."
    echo -e "\nAccess points:"
    echo "- Frontend: http://localhost:3000"
    echo "- Server API: http://localhost:8081"
    echo -e "\nMake sure you have:"
    echo "1. MongoDB running on localhost:27017"
    echo "2. Zond node accessible at $NODE_URL"
    if [[ $SETUP_NGINX =~ ^[Yy]$ ]]; then
        echo "3. Nginx is configured and running"
        echo "4. DNS is pointing to this server"
    fi
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
