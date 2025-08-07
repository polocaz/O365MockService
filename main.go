package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Common Microsoft Graph API response structures
type User struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
	JobTitle          string `json:"jobTitle"`
	Department        string `json:"department"`
	OfficeLocation    string `json:"officeLocation"`
}

type Group struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	GroupType   string `json:"groupType"`
	Mail        string `json:"mail"`
}

type GraphResponse struct {
	Context   string      `json:"@odata.context,omitempty"`
	NextLink  string      `json:"@odata.nextLink,omitempty"`
	Count     int         `json:"@odata.count,omitempty"`
	Value     interface{} `json:"value"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Configurable user generation
var mockUsers []User

// Configuration for user generation
type UserConfig struct {
	UserCount int    `json:"userCount"`
	Domain    string `json:"domain"`
}

// Generate mock users based on configuration
func generateMockUsers(count int, domain string) []User {
	if domain == "" {
		domain = "contoso.com"
	}
	
	// Sample data for generating realistic users
	firstNames := []string{
		"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Eve", "Frank",
		"Grace", "Henry", "Ivy", "Jack", "Kate", "Liam", "Mia", "Noah",
		"Olivia", "Paul", "Quinn", "Rachel", "Sam", "Tina", "Uma", "Victor",
		"Wendy", "Xavier", "Yara", "Zack", "Amy", "Ben", "Cara", "David",
		"Emma", "Felix", "Gina", "Hugo", "Iris", "Jake", "Kelly", "Luna",
		"Mike", "Nina", "Oscar", "Penny", "Quincy", "Rose", "Steve", "Tara",
		"Ulrich", "Vera", "Wade", "Xenia", "Yale", "Zoe", "Alex", "Blake",
		"Casey", "Drew", "Ellis", "Finley", "Gray", "Harper", "Indigo", "Jordan",
	}
	
	lastNames := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
		"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas",
		"Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White",
		"Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker", "Young",
		"Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores",
		"Green", "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
		"Carter", "Roberts", "Gomez", "Phillips", "Evans", "Turner", "Diaz", "Parker",
		"Cruz", "Edwards", "Collins", "Reyes", "Stewart", "Morris", "Morales", "Murphy",
	}
	
	jobTitles := []string{
		"Software Engineer", "Senior Software Engineer", "Product Manager", "Senior Product Manager",
		"DevOps Engineer", "Data Scientist", "UX Designer", "UI Designer", "Business Analyst",
		"Project Manager", "Scrum Master", "Technical Lead", "Engineering Manager", "Director",
		"VP Engineering", "CTO", "CEO", "Marketing Manager", "Sales Manager", "HR Manager",
		"Finance Manager", "Operations Manager", "Customer Success Manager", "Support Engineer",
		"QA Engineer", "Security Engineer", "Cloud Architect", "Frontend Developer", "Backend Developer",
		"Full Stack Developer", "Mobile Developer", "Database Administrator", "System Administrator",
	}
	
	departments := []string{
		"Engineering", "Product", "Marketing", "Sales", "Human Resources", "Finance",
		"Operations", "Customer Success", "Support", "Security", "IT", "Legal",
		"Research & Development", "Quality Assurance", "Data Science", "Design",
	}
	
	buildings := []string{
		"Building 1", "Building 2", "Building 3", "Main Campus", "West Campus", "East Campus",
		"Downtown Office", "Remote", "Headquarters", "Branch Office",
	}
	
	floors := []string{
		"Floor 1", "Floor 2", "Floor 3", "Floor 4", "Floor 5", "Ground Floor", "Basement",
	}
	
	users := make([]User, count)
	
	for i := 0; i < count; i++ {
		firstName := firstNames[i%len(firstNames)]
		lastName := lastNames[i%len(lastNames)]
		
		// Create unique variations for large numbers
		if i >= len(firstNames) {
			firstName = fmt.Sprintf("%s%d", firstName, (i/len(firstNames))+1)
		}
		
		displayName := fmt.Sprintf("%s %s", firstName, lastName)
		username := fmt.Sprintf("%s.%s", strings.ToLower(firstName), strings.ToLower(lastName))
		userPrincipalName := fmt.Sprintf("%s@%s", username, domain)
		
		// Generate a UUID-like ID
		userID := fmt.Sprintf("%08d-%04d-%04d-%04d-%012d", 
			10000000+i, 1000+i%1000, 2000+i%2000, 3000+i%3000, 100000000000+i)
		
		users[i] = User{
			ID:                userID,
			DisplayName:       displayName,
			UserPrincipalName: userPrincipalName,
			Mail:              userPrincipalName,
			JobTitle:          jobTitles[i%len(jobTitles)],
			Department:        departments[i%len(departments)],
			OfficeLocation:    fmt.Sprintf("%s, %s", buildings[i%len(buildings)], floors[i%len(floors)]),
		}
	}
	
	return users
}

var mockGroups = []Group{
	{
		ID:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		DisplayName: "Engineering Team",
		Description: "All engineering staff",
		GroupType:   "Security",
		Mail:        "engineering@contoso.com",
	},
	{
		ID:          "ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj",
		DisplayName: "Product Team",
		Description: "Product management and design",
		GroupType:   "Security",
		Mail:        "product@contoso.com",
	},
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Authentication middleware (simplified for testing)
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendErrorResponse(w, "unauthorized", "Access token is missing", http.StatusUnauthorized)
			return
		}

		// For testing purposes, accept any Bearer token
		// In a real scenario, you'd validate the token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			sendErrorResponse(w, "unauthorized", "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Send error response
func sendErrorResponse(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	errorResp := ErrorResponse{}
	errorResp.Error.Code = code
	errorResp.Error.Message = message
	
	json.NewEncoder(w).Encode(errorResp)
}

// Get all users with proper Graph API pagination
func getUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Parse pagination parameters - Graph API style
	skip := 0
	top := 100 // Default page size like real Graph API
	
	if skipParam := r.URL.Query().Get("$skip"); skipParam != "" {
		if s, err := strconv.Atoi(skipParam); err == nil && s >= 0 {
			skip = s
		}
	}
	
	if topParam := r.URL.Query().Get("$top"); topParam != "" {
		if t, err := strconv.Atoi(topParam); err == nil && t > 0 && t <= 999 {
			top = t
		}
	}
	
	// Calculate slice bounds
	totalUsers := len(mockUsers)
	start := skip
	end := skip + top
	
	if start >= totalUsers {
		// Return empty result if skip is beyond available data
		response := GraphResponse{
			Context: "https://graph.microsoft.com/v1.0/$metadata#users",
			Value:   []User{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	if end > totalUsers {
		end = totalUsers
	}
	
	users := mockUsers[start:end]
	
	// Build response exactly like Graph API
	response := GraphResponse{
		Context: "https://graph.microsoft.com/v1.0/$metadata#users",
		Value:   users,
	}
	
	// Add nextLink if there are more results (Graph API behavior)
	if end < totalUsers {
		nextSkip := end
		// Build nextLink URL exactly like Microsoft Graph API
		baseURL := r.URL.Scheme
		if baseURL == "" {
			baseURL = "http" // Default for local testing
		}
		host := r.Host
		if host == "" {
			host = fmt.Sprintf("localhost:%s", os.Getenv("PORT"))
			if os.Getenv("PORT") == "" {
				host = "localhost:8080"
			}
		}
		
		nextLink := fmt.Sprintf("%s://%s/v1.0/users?$skip=%d", baseURL, host, nextSkip)
		if topParam := r.URL.Query().Get("$top"); topParam != "" {
			nextLink += fmt.Sprintf("&$top=%s", topParam)
		}
		
		response.NextLink = nextLink
	}
	
	// Add count if requested (Graph API behavior)
	if r.URL.Query().Get("$count") == "true" {
		response.Count = totalUsers
	}
	
	json.NewEncoder(w).Encode(response)
}

// Get user by ID
func getUserByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	vars := mux.Vars(r)
	userID := vars["id"]
	
	for _, user := range mockUsers {
		if user.ID == userID || user.UserPrincipalName == userID {
			json.NewEncoder(w).Encode(user)
			return
		}
	}
	
	sendErrorResponse(w, "Request_ResourceNotFound", "User not found", http.StatusNotFound)
}

// Get current user (me endpoint)
func getCurrentUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Return the first user as the "current" user
	if len(mockUsers) > 0 {
		json.NewEncoder(w).Encode(mockUsers[0])
	} else {
		sendErrorResponse(w, "Request_ResourceNotFound", "Current user not found", http.StatusNotFound)
	}
}

// Get all groups
func getGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	response := GraphResponse{
		Context: "https://graph.microsoft.com/v1.0/$metadata#groups",
		Count:   len(mockGroups),
		Value:   mockGroups,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Get group by ID
func getGroupByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	vars := mux.Vars(r)
	groupID := vars["id"]
	
	for _, group := range mockGroups {
		if group.ID == groupID {
			json.NewEncoder(w).Encode(group)
			return
		}
	}
	
	sendErrorResponse(w, "Request_ResourceNotFound", "Group not found", http.StatusNotFound)
}

// Configuration endpoint to set user count
func configureUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method == "GET" {
		// Return current configuration
		config := UserConfig{
			UserCount: len(mockUsers),
			Domain:    "contoso.com", // You can make this configurable too
		}
		json.NewEncoder(w).Encode(config)
		return
	}
	
	if r.Method == "POST" {
		var config UserConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			sendErrorResponse(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
			return
		}
		
		// Validate user count
		if config.UserCount < 1 {
			sendErrorResponse(w, "invalid_request", "User count must be at least 1", http.StatusBadRequest)
			return
		}
		
		if config.UserCount > 10000 {
			sendErrorResponse(w, "invalid_request", "User count cannot exceed 10,000", http.StatusBadRequest)
			return
		}
		
		// Generate new users
		domain := config.Domain
		if domain == "" {
			domain = "contoso.com"
		}
		
		mockUsers = generateMockUsers(config.UserCount, domain)
		
		log.Printf("🔄 Generated %d mock users with domain %s", config.UserCount, domain)
		
		response := map[string]interface{}{
			"message":   fmt.Sprintf("Successfully generated %d users", config.UserCount),
			"userCount": len(mockUsers),
			"domain":    domain,
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	sendErrorResponse(w, "method_not_allowed", "Only GET and POST methods are allowed", http.StatusMethodNotAllowed)
}

// Bulk generate users endpoint (for quick testing)
func bulkGenerateUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Get count from query parameter
	countStr := r.URL.Query().Get("count")
	if countStr == "" {
		countStr = "100" // Default to 100 users
	}
	
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 10000 {
		sendErrorResponse(w, "invalid_request", "Count must be a number between 1 and 10,000", http.StatusBadRequest)
		return
	}
	
	// Get domain from query parameter
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		domain = "contoso.com"
	}
	
	// Generate users
	mockUsers = generateMockUsers(count, domain)
	
	log.Printf("🔄 Bulk generated %d mock users with domain %s", count, domain)
	
	response := map[string]interface{}{
		"message":   fmt.Sprintf("Successfully generated %d users", count),
		"userCount": len(mockUsers),
		"domain":    domain,
		"endpoint":  "/v1.0/users",
	}
	json.NewEncoder(w).Encode(response)
}

// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "O365 Mock Service",
		"version":   "1.0.0",
		"userCount": len(mockUsers),
	}
	json.NewEncoder(w).Encode(response)
}

// Root endpoint that mimics Graph API discovery
func rootEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"@odata.context": "https://graph.microsoft.com/v1.0/$metadata",
		"value": map[string]string{
			"name":        "Microsoft Graph Mock API",
			"description": "Mock implementation of Microsoft Graph API for testing",
			"version":     "v1.0",
		},
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get user count from command line argument or environment variable
	userCount := 100 // Default to 100 users (like a typical test scenario)
	domain := "contoso.com"
	
	// Check command line arguments first
	if len(os.Args) > 1 {
		if count, err := strconv.Atoi(os.Args[1]); err == nil && count > 0 && count <= 50000 {
			userCount = count
		} else {
			log.Printf("❌ Invalid user count: %s. Using default: %d", os.Args[1], userCount)
		}
	} else {
		// Fallback to environment variable
		if userCountStr := os.Getenv("USER_COUNT"); userCountStr != "" {
			if count, err := strconv.Atoi(userCountStr); err == nil && count > 0 && count <= 50000 {
				userCount = count
			}
		}
	}
	
	// Get domain from environment variable
	if envDomain := os.Getenv("DOMAIN"); envDomain != "" {
		domain = envDomain
	}

	// Initialize mock users
	log.Printf("🔄 Generating %d mock users for domain %s...", userCount, domain)
	mockUsers = generateMockUsers(userCount, domain)
	log.Printf("✅ Generated %d mock users successfully", len(mockUsers))

	// Create router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(corsMiddleware)

	// Public endpoints (no auth required)
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/", rootEndpoint).Methods("GET")
	
	// Configuration endpoints (no auth required for easy testing)
	r.HandleFunc("/config/users", configureUsers).Methods("GET", "POST")
	r.HandleFunc("/generate/users", bulkGenerateUsers).Methods("POST")

	// Microsoft Graph API endpoints (with auth)
	apiRouter := r.PathPrefix("/v1.0").Subrouter()
	apiRouter.Use(authMiddleware)

	// User endpoints - Graph API compliant
	apiRouter.HandleFunc("/users", getUsers).Methods("GET")
	apiRouter.HandleFunc("/users/{id}", getUserByID).Methods("GET")
	apiRouter.HandleFunc("/me", getCurrentUser).Methods("GET")

	// Group endpoints (keeping minimal for completeness)
	apiRouter.HandleFunc("/groups", getGroups).Methods("GET")
	apiRouter.HandleFunc("/groups/{id}", getGroupByID).Methods("GET")

	// Start server
	log.Printf("")
	log.Printf("🚀 O365 Mock Service starting on port %s", port)
	log.Printf("� Total mock users: %d (domain: %s)", len(mockUsers), domain)
	log.Printf("� Default page size: 100 users (like real Graph API)")
	log.Printf("")
	log.Printf("🔗 Endpoints:")
	log.Printf("   Health: http://localhost:%s/health", port)
	log.Printf("   Users:  http://localhost:%s/v1.0/users", port)
	log.Printf("   Config: http://localhost:%s/config/users", port)
	log.Printf("")
	log.Printf("� Examples (Graph API pagination):")
	log.Printf("   # Get first page (100 users)")
	log.Printf("   curl -H 'Authorization: Bearer test' http://localhost:%s/v1.0/users", port)
	log.Printf("")
	log.Printf("   # Get specific page size")
	log.Printf("   curl -H 'Authorization: Bearer test' 'http://localhost:%s/v1.0/users?\\$top=50'", port)
	log.Printf("")
	log.Printf("   # Get specific page (skip first 100)")
	log.Printf("   curl -H 'Authorization: Bearer test' 'http://localhost:%s/v1.0/users?\\$skip=100'", port)
	log.Printf("")
	log.Printf("   # Get with count")
	log.Printf("   curl -H 'Authorization: Bearer test' 'http://localhost:%s/v1.0/users?\\$count=true'", port)
	log.Printf("")
	log.Printf("🎯 To generate different user counts:")
	log.Printf("   go run main.go 1000    # Generate 1000 users")
	log.Printf("   go run main.go 5000    # Generate 5000 users")
	
	log.Fatal(http.ListenAndServe(":"+port, r))
}
