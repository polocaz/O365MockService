# MDM Mock Service

A Go-based mock service that mimics Microsoft Graph API endpoints **and Jamf Pro API endpoints** for testing C# applications.

## Features

- **Microsoft Graph API Simulation**: Mimics common Microsoft Graph API endpoints
- **Jamf Pro API Simulation**: Mimics `/v1/auth/token`, `/v1/departments/`, and `/v1/computers-inventory` for testing `JamfServer::CollectUserInfo`
- **Flexible Authentication**: Accepts any Bearer token for testing purposes
- **CORS Support**: Enables cross-origin requests for web applications
- **Pagination Support**: Handles `$skip`/`$top` (Graph) and `page`/`page-size` (Jamf) query parameters
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

### Graph API Endpoints (Require `Authorization: Bearer <any>`)
- `GET /v1.0/users` - List all users
- `GET /v1.0/users/{id}` - Get user by ID or User Principal Name
- `GET /v1.0/me` - Get current user
- `GET /v1.0/groups` - List all groups
- `GET /v1.0/groups/{id}` - Get group by ID

#### Query Parameters Supported
- `$skip` - Number of items to skip (pagination)
- `$top` - Number of items to return (pagination, default: 100)
- `$count` - Include total count in response (set to true)

### Jamf Pro API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/v1/auth/token` | `Basic` (any credentials) | Get a mock bearer token |
| `GET` | `/v1/departments/` | `Bearer` | List all 10 mock departments |
| `GET` | `/v1/computers-inventory` | `Bearer` | Paginated list of 500 mock computers |

#### Jamf Query Parameters
- `page` - Zero-based page number (default: `0`)
- `page-size` - Records per page (default: `500`)
- `section` - Ignored by the mock (pass `HARDWARE` / `USER_AND_LOCATION` freely)

#### Jamf Computer Record Structure
```json
{
  "id": "1",
  "udid": "00000001-abcd-1234-5678-000000000001",
  "general": {
    "name": "mac-smith-001",
    "serialNumber": "C00000001XXXX"
  },
  "hardware": {
    "model": "MacBook Pro 14-inch"
  },
  "userAndLocation": {
    "username": "alice.smith",
    "realName": "Alice Smith",
    "emailAddress": "alice.smith@mock.jamf.local",
    "position": "Engineer",
    "phone": "555-0001",
    "departmentId": 1
  }
}
```

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

### Jamf Token Response
```json
{
  "token": "mock-jamf-token",
  "expires": "2026-04-03T06:00:00Z"
}
```

### Jamf Departments Response
```json
{
  "totalCount": 10,
  "results": [
    {"id": 1, "name": "Engineering"},
    {"id": 2, "name": "Product"}
  ]
}
```

### Jamf Computers Inventory Response
```json
{
  "totalCount": 500,
  "results": [
    {
      "id": "1",
      "udid": "00000001-abcd-1234-5678-000000000001",
      "general": {"name": "mac-smith-001", "serialNumber": "C00000001XXXX"},
      "hardware": {"model": "MacBook Pro 14-inch"},
      "userAndLocation": {
        "username": "alice.smith",
        "realName": "Alice Smith",
        "emailAddress": "alice.smith@mock.jamf.local",
        "position": "Engineer",
        "phone": "555-0001",
        "departmentId": 1
      }
    }
  ]
}
```

### Graph Users Response (with Pagination)
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

**Graph API (`/v1.0/users`)**
- Default page size: 100 users per page
- Use `$top` and `$skip` to control pagination
- `@odata.nextLink` is included when more pages exist

**Jamf API (`/v1/computers-inventory`)**
- Default page size: 500 computers per page
- Use `page` (zero-based) and `page-size` to control pagination
- Last page is signaled by `results` count being less than `page-size`

### Single User Response (Graph)
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

## Testing Jamf Endpoints

Start the service first:
```bash
./start.sh 8080
```

### 1. Get a mock auth token (mimics JamfServer constructor)
```bash
# Any Base64-encoded "user:password" value works
curl -s -X POST http://localhost:8080/v1/auth/token \
  -H "Authorization: Basic dXNlcjpwYXNz"
```

Expected output:
```json
{"expires":"2026-04-03T06:00:00Z","token":"mock-jamf-token"}
```

### 2. Get departments (mimics ParseDepartments call)
```bash
curl -s http://localhost:8080/v1/departments/ \
  -H "Authorization: Bearer mock-jamf-token" | python3 -m json.tool
```

### 3. Get first page of computers (mimics the CollectUserInfo loop, page 0)
```bash
curl -s "http://localhost:8080/v1/computers-inventory?section=HARDWARE&section=USER_AND_LOCATION&page=0&page-size=500" \
  -H "Authorization: Bearer mock-jamf-token" | python3 -m json.tool
```

### 4. Check pagination termination (page 1 returns empty results)
```bash
curl -s "http://localhost:8080/v1/computers-inventory?page=1&page-size=500" \
  -H "Authorization: Bearer mock-jamf-token" \
  | python3 -c "import sys,json; d=json.load(sys.stdin); print('totalCount:', d['totalCount'], '| page 1 results:', len(d['results']))"
```

Expected: `totalCount: 500 | page 1 results: 0`

### 5. Verify department lookup works end-to-end
```bash
# Fetch departments and extract the map, then check a computer's departmentId resolves correctly
curl -s http://localhost:8080/v1/departments/ -H "Authorization: Bearer mock-jamf-token" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
dept_map = {r['id']: r['name'] for r in d['results']}
print('Department map:', dept_map)
"

curl -s "http://localhost:8080/v1/computers-inventory?page=0&page-size=10" \
  -H "Authorization: Bearer mock-jamf-token" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
for c in d['results'][:3]:
  print(c['general']['serialNumber'], '|', c['userAndLocation']['username'], '| deptId:', c['userAndLocation']['departmentId'])
"
```

### 6. Verify auth is required
```bash
# Should return 401
curl -s -o /dev/null -w "HTTP status: %{http_code}\n" http://localhost:8080/v1/departments/

# Should return 401 (token endpoint requires Basic, not Bearer)
curl -s -o /dev/null -w "HTTP status: %{http_code}\n" \
  -X POST http://localhost:8080/v1/auth/token \
  -H "Authorization: Bearer mock-jamf-token"
```

### 7. Run a quick full-flow check in one pipeline
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/token \
  -H "Authorization: Basic dXNlcjpwYXNz" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

echo "Token: $TOKEN"

DEPT_COUNT=$(curl -s http://localhost:8080/v1/departments/ \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin)['totalCount'])")

echo "Departments: $DEPT_COUNT"

COMPUTER_COUNT=$(curl -s "http://localhost:8080/v1/computers-inventory?page=0&page-size=500" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"total={d['totalCount']} page0={len(d['results'])}\")")

echo "Computers: $COMPUTER_COUNT"
```

Expected output:
```
Token: mock-jamf-token
Departments: 10
Computers: total=500 page0=500
```

### Point JamfServer at the mock
Set `JamfUrl = "http://localhost:8080"` in your MDM settings and use any username/password credentials. The mock accepts all Basic auth values and returns a valid token that will be accepted for subsequent Bearer requests.

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
