# O365 Mock Service

A Go-based mock service that mimics Microsoft Graph API endpoints for testing C# applications that query the Microsoft Graph API.

## Features

- **Complete Graph API Simulation**: Mimics common Microsoft Graph API endpoints
- **Flexible Authentication**: Accepts any Bearer token for testing purposes
- **CORS Support**: Enables cross-origin requests for web applications
- **Pagination Support**: Handles `$skip` and `$top` query parameters
- **Multiple Integration Options**: Several ways to redirect your C# app to the mock service
- **Customizable Mock Data**: Easy to modify test data

## Quick Start

### 1. Install Dependencies
```bash
go mod tidy
```

### 2. Run the Service

**macOS/Linux:**
```bash
./start.sh                    # Default: port 8080, 100 users
./start.sh 8090               # Port 8090, 100 users  
./start.sh 8080 500           # Port 8080, 500 users
```

**Windows:**
```cmd
windows\start.cmd             # Default: port 8080, 100 users
windows\start.cmd 8090        # Port 8090, 100 users
windows\start.cmd 8080 500    # Port 8080, 500 users
```

### 3. Test the Service
```bash
# Health check
curl http://localhost:8080/health

# Get users (requires Authorization header)
curl -H "Authorization: Bearer test-token" \
     http://localhost:8080/v1.0/users

# Test pagination behavior
./test-pagination.sh 8080
```

## Available Endpoints

### Public Endpoints (No Authentication Required)
- `GET /health` - Health check
- `GET /` - Service discovery

### Graph API Endpoints (Require Authorization Header)
- `GET /v1.0/users` - List all users
- `GET /v1.0/users/{id}` - Get user by ID or User Principal Name
- `GET /v1.0/me` - Get current user
- `GET /v1.0/groups` - List all groups
- `GET /v1.0/groups/{id}` - Get group by ID

### Query Parameters Supported
- `$skip` - Number of items to skip (pagination)
- `$top` - Number of items to return (pagination, default: 100)
- `$count` - Include total count in response (set to true)

## Configuration

### Command Line Arguments
```bash
# Start with custom port and user count
./start.sh [PORT] [USER_COUNT]

# Examples:
./start.sh 8080 1000          # 1000 users on port 8080
./start.sh 9000 250           # 250 users on port 9000
```

### Environment Variables
- `PORT` - Server port (default: 8080)

### Mock Data
The service generates realistic mock users automatically based on the USER_COUNT parameter. Each user includes:
- Display Name, Email, Job Title, Department
- Office Location, Phone Numbers
- Unique IDs and User Principal Names

## Sample Responses

### Users Response (with Pagination)
```json
{
  "@odata.context": "https://graph.microsoft.com/v1.0/$metadata#users",
  "@odata.count": 250,
  "@odata.nextLink": "http://localhost:8080/v1.0/users?$skip=100",
  "value": [
    {
      "id": "12345678-1234-1234-1234-123456789012",
      "displayName": "John Doe",
      "userPrincipalName": "john.doe@contoso.com",
      "mail": "john.doe@contoso.com",
      "jobTitle": "Software Engineer",
      "department": "Engineering",
      "officeLocation": "Building 1, Floor 2",
      "businessPhones": ["+1-555-0123"],
      "mobilePhone": "+1-555-0124"
    }
  ]
}
```

### Pagination Behavior
The service exactly replicates Microsoft Graph API pagination:
- **Default page size**: 100 users per page
- **Custom page sizes**: Use `$top` parameter (e.g., `$top=25`)
- **Navigation**: Use `$skip` parameter or follow `@odata.nextLink`
- **Total count**: Use `$count=true` to include total count
- **Standard C# pagination works**:
  ```csharp
  var users = await graphClient.Users.Request().GetAsync();
  while (users.NextPageRequest != null) {
      users = await users.NextPageRequest.GetAsync();
  }
  ```

### Single User Response
```json
{
  "id": "12345678-1234-1234-1234-123456789012",
  "displayName": "John Doe",
  "userPrincipalName": "john.doe@contoso.com",
  "mail": "john.doe@contoso.com",
  "jobTitle": "Software Engineer",
  "department": "Engineering",
  "officeLocation": "Building 1, Floor 2"
}
```

## Development

### Building
```bash
go build -o o365mockservice main.go
```

### Running with Custom Configuration
```bash
# Custom port and user count
PORT=9000 go run main.go 500

# Or using the start script
./start.sh 9000 500
```

### Testing
```bash
# Test basic functionality
curl http://localhost:8080/health

# Test pagination behavior
./test-pagination.sh 8080

# Test with different user counts
./start.sh 8080 1000    # 1000 users
./start.sh 8080 5000    # 5000 users
```

### Adding New Endpoints
1. Define response structures in `main.go`
2. Add generateMock* function for test data
3. Create handler function
4. Register route in the router

## Authentication

The mock service uses simplified authentication for testing:
- Accepts any `Bearer` token in the `Authorization` header
- No actual token validation is performed
- This allows easy testing without dealing with real OAuth flows

## CORS

Cross-Origin Resource Sharing (CORS) is enabled by default to support web applications and testing tools.

## Error Handling

The service returns appropriate HTTP status codes and Microsoft Graph API-compatible error responses:

```json
{
  "error": {
    "code": "Request_ResourceNotFound",
    "message": "User not found"
  }
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is for testing purposes. Please ensure compliance with Microsoft Graph API terms of service when using for development and testing.
