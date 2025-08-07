#!/bin/bash

# Start the O365 Mock Service
# Usage: ./start.sh [PORT] [USER_COUNT]
# Examples:
#   ./start.sh                    # Default: port 8080, 100 users
#   ./start.sh 8090              # Port 8090, 100 users  
#   ./start.sh 8080 500          # Port 8080, 500 users

PORT=${1:-8080}
USER_COUNT=${2:-100}

echo "Starting O365 Mock Service..."
echo "Port: $PORT"
echo "Mock users: $USER_COUNT"
echo ""
echo "Health check will be available at: http://localhost:$PORT/health"
echo "Graph API endpoints at: http://localhost:$PORT/v1.0"
echo ""
echo "Example test commands:"
echo "curl http://localhost:$PORT/health"
echo "curl -H \"Authorization: Bearer test-token\" http://localhost:$PORT/v1.0/users"
echo "curl -H \"Authorization: Bearer test-token\" 'http://localhost:$PORT/v1.0/users?\$top=10'"
echo "curl -H \"Authorization: Bearer test-token\" 'http://localhost:$PORT/v1.0/users?\$skip=100&\$top=50'"
echo ""
echo "Press Ctrl+C to stop the service"
echo ""

PORT=$PORT go run main.go $USER_COUNT
