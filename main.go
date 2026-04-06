package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Common Microsoft Graph API response structures
type User struct {
	ID                         string   `json:"id"`
	AccountEnabled             bool     `json:"accountEnabled"`
	BusinessPhones             []string `json:"businessPhones"`
	City                       string   `json:"city"`
	Country                    string   `json:"country"`
	CreatedDateTime            string   `json:"createdDateTime"`
	Department                 string   `json:"department"`
	DisplayName                string   `json:"displayName"`
	FaxNumber                  string   `json:"faxNumber"`
	GivenName                  string   `json:"givenName"`
	JobTitle                   string   `json:"jobTitle"`
	LastPasswordChangeDateTime string   `json:"lastPasswordChangeDateTime"`
	Mail                       string   `json:"mail"`
	MobilePhone                string   `json:"mobilePhone"`
	OfficeLocation             string   `json:"officeLocation"`
	Surname                    string   `json:"surname"`
	UserPrincipalName          string   `json:"userPrincipalName"`
}

type Group struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	GroupType   string `json:"groupType"`
	Mail        string `json:"mail"`
}

type GraphResponse struct {
	Context  string      `json:"@odata.context,omitempty"`
	NextLink string      `json:"@odata.nextLink,omitempty"`
	Count    int         `json:"@odata.count,omitempty"`
	Value    interface{} `json:"value"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Configurable user generation
var mockUsers []User
var userDataDir = "user_data"
var usersPerFile = 5000 // Increase to 5000 users per file for better efficiency
var totalUserCount = 0
var maxUsersInMemory = 10000 // Only keep 10k users in memory max

// User index for fast lookups
type UserIndex struct {
	ID                string `json:"id"`
	UserPrincipalName string `json:"userPrincipalName"`
	FileIndex         int    `json:"fileIndex"`
	PositionInFile    int    `json:"positionInFile"`
}

var userIndex map[string]UserIndex // Fast lookup index

// Configuration for user generation
type UserConfig struct {
	UserCount int    `json:"userCount"`
	Domain    string `json:"domain"`
}

// File-based user storage
type UserFile struct {
	StartIndex int    `json:"startIndex"`
	EndIndex   int    `json:"endIndex"`
	Users      []User `json:"users"`
}

// ─── Jamf Pro API data structures ────────────────────────────────────────────

type JamfDepartment struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type JamfComputerGeneral struct {
	Name         string `json:"name"`
	SerialNumber string `json:"serialNumber"`
}

type JamfComputerHardware struct {
	Model string `json:"model"`
}

type JamfComputerUserAndLocation struct {
	Username     string `json:"username"`
	RealName     string `json:"realName"`
	EmailAddress string `json:"emailAddress"`
	Position     string `json:"position"`
	Phone        string `json:"phone"`
	DepartmentId int    `json:"departmentId"`
}

type JamfComputer struct {
	ID              string                      `json:"id"`
	UDID            string                      `json:"udid"`
	General         JamfComputerGeneral         `json:"general"`
	Hardware        JamfComputerHardware        `json:"hardware"`
	UserAndLocation JamfComputerUserAndLocation `json:"userAndLocation"`
}

type JamfPagedResponse struct {
	TotalCount int         `json:"totalCount"`
	Results    interface{} `json:"results"`
}

// Jamf mock data
var mockJamfDepartments []JamfDepartment
var mockJamfComputers []JamfComputer

// Generate a random number
func randomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// Shuffle a slice of strings
func shuffleStrings(slice []string) {
	for i := len(slice) - 1; i > 0; i-- {
		j := randomInt(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// Generate unique username
func generateUniqueUsername(firstName, lastName string, usedNames map[string]bool, attempt int) string {
	var username string
	if attempt == 0 {
		username = fmt.Sprintf("%s.%s", strings.ToLower(firstName), strings.ToLower(lastName))
	} else {
		username = fmt.Sprintf("%s.%s%d", strings.ToLower(firstName), strings.ToLower(lastName), attempt)
	}

	if usedNames[username] {
		return generateUniqueUsername(firstName, lastName, usedNames, attempt+1)
	}

	usedNames[username] = true
	return username
}

// Save users to file
func saveUsersToFile(users []User, startIndex int) error {
	start := time.Now()

	// Ensure directory exists
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return err
	}

	userFile := UserFile{
		StartIndex: startIndex,
		EndIndex:   startIndex + len(users) - 1,
		Users:      users,
	}

	fileIndex := startIndex / usersPerFile
	filename := filepath.Join(userDataDir, fmt.Sprintf("users_%05d.json", fileIndex))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Build index while saving
	if userIndex == nil {
		userIndex = make(map[string]UserIndex)
	}

	for i, user := range users {
		userIndex[user.ID] = UserIndex{
			ID:                user.ID,
			UserPrincipalName: user.UserPrincipalName,
			FileIndex:         fileIndex,
			PositionInFile:    i,
		}
		userIndex[user.UserPrincipalName] = userIndex[user.ID]
	}

	err = json.NewEncoder(file).Encode(userFile)
	duration := time.Since(start)

	if err == nil {
		log.Printf("💾 Saved %d users to file %d in %v", len(users), fileIndex, duration)
	}

	return err
}

// Load users from file by index range - optimized for large datasets
func loadUsersFromFile(startIndex, count int) ([]User, error) {
	loadStart := time.Now()

	var allUsers []User
	filesProcessed := 0

	// Calculate which files we need
	startFileIndex := startIndex / usersPerFile
	endFileIndex := (startIndex + count - 1) / usersPerFile

	for fileIndex := startFileIndex; fileIndex <= endFileIndex; fileIndex++ {
		fileStart := time.Now()
		filename := filepath.Join(userDataDir, fmt.Sprintf("users_%05d.json", fileIndex))

		file, err := os.Open(filename)
		if err != nil {
			continue
		}

		var userFile UserFile
		if err := json.NewDecoder(file).Decode(&userFile); err != nil {
			file.Close()
			continue
		}
		file.Close()
		filesProcessed++

		// Calculate which users from this file we need
		fileStartIdx := 0
		fileEndIdx := len(userFile.Users)

		// Adjust start index for this file
		if startIndex > userFile.StartIndex {
			fileStartIdx = startIndex - userFile.StartIndex
		}

		// Adjust end index for this file
		remainingCount := count - len(allUsers)
		if fileStartIdx+remainingCount < len(userFile.Users) {
			fileEndIdx = fileStartIdx + remainingCount
		}

		if fileStartIdx < len(userFile.Users) && fileEndIdx > fileStartIdx {
			usersFromFile := userFile.Users[fileStartIdx:fileEndIdx]
			allUsers = append(allUsers, usersFromFile...)

			fileLoadTime := time.Since(fileStart)
			log.Printf("📂 Loaded %d users from file %d in %v", len(usersFromFile), fileIndex, fileLoadTime)
		}

		if len(allUsers) >= count {
			break
		}
	}

	totalLoadTime := time.Since(loadStart)
	if len(allUsers) > 0 {
		log.Printf("📊 Total load time: %v for %d users from %d files", totalLoadTime, len(allUsers), filesProcessed)
	}

	return allUsers, nil
}

// Get total user count from files
func getTotalUserCountFromFiles() int {
	files, err := filepath.Glob(filepath.Join(userDataDir, "users_*.json"))
	if err != nil {
		return 0
	}

	maxIndex := -1
	for _, filename := range files {
		file, err := os.Open(filename)
		if err != nil {
			continue
		}

		var userFile UserFile
		if err := json.NewDecoder(file).Decode(&userFile); err == nil {
			if userFile.EndIndex > maxIndex {
				maxIndex = userFile.EndIndex
			}
		}
		file.Close()
	}

	return maxIndex + 1
}

// Generate mock users based on configuration
func generateMockUsers(count int, domain string) []User {
	generationStart := time.Now()

	if domain == "" {
		domain = "contoso.com"
	}

	log.Printf("⏱️ Starting user generation: %d users with domain %s", count, domain)

	// Expanded lists of names for more randomness
	firstNames := []string{
		"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Eve", "Frank",
		"Grace", "Henry", "Ivy", "Jack", "Kate", "Liam", "Mia", "Noah",
		"Olivia", "Paul", "Quinn", "Rachel", "Sam", "Tina", "Uma", "Victor",
		"Wendy", "Xavier", "Yara", "Zack", "Amy", "Ben", "Cara", "David",
		"Emma", "Felix", "Gina", "Hugo", "Iris", "Jake", "Kelly", "Luna",
		"Mike", "Nina", "Oscar", "Penny", "Quincy", "Rose", "Steve", "Tara",
		"Ulrich", "Vera", "Wade", "Xenia", "Yale", "Zoe", "Alex", "Blake",
		"Casey", "Drew", "Ellis", "Finley", "Gray", "Harper", "Indigo", "Jordan",
		"Kai", "Logan", "Morgan", "Peyton", "Riley", "Sage", "Taylor", "Avery",
		"Cameron", "Emery", "Hayden", "Jamie", "Kendall", "Lane", "Micah", "Noel",
		"Parker", "River", "Skyler", "Teagan", "Val", "Winter", "Ashton", "Bryce",
		"Devon", "Ezra", "Finley", "Greer", "Haven", "Ira", "Jules", "Kris",
		"Lee", "Max", "Nova", "Onyx", "Phoenix", "Rowan", "Sage", "Tatum",
		"Uri", "Vega", "Wren", "Xander", "Yuki", "Zen", "Adrian", "Brook",
		"Cedar", "Darian", "Erin", "Fable", "Gale", "Hollis", "Ivory", "Jude",
		"Kit", "Lynn", "Merit", "Nico", "Ocean", "Pax", "Quest", "Rain",
		"Seven", "True", "Unity", "Vale", "Wynn", "Xyla", "Yael", "Zara",
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
		"Cook", "Rogers", "Reed", "Bailey", "Bell", "Cooper", "Richardson", "Cox",
		"Howard", "Ward", "Peterson", "Gray", "James", "Watson", "Brooks", "Kelly",
		"Sanders", "Price", "Bennett", "Wood", "Barnes", "Ross", "Henderson", "Coleman",
		"Jenkins", "Perry", "Powell", "Long", "Patterson", "Hughes", "Flores", "Washington",
		"Butler", "Simmons", "Foster", "Gonzales", "Bryant", "Alexander", "Russell", "Griffin",
		"Diaz", "Hayes", "Myers", "Ford", "Hamilton", "Graham", "Sullivan", "Wallace",
		"Woods", "Cole", "West", "Jordan", "Owens", "Reynolds", "Fisher", "Ellis",
		"Harrison", "Gibson", "Mcdonald", "Cruz", "Marshall", "Ortiz", "Gomez", "Murray",
		"Freeman", "Wells", "Webb", "Simpson", "Stevens", "Tucker", "Porter", "Hunter",
		"Hicks", "Crawford", "Henry", "Boyd", "Mason", "Morales", "Kennedy", "Warren",
		"Dixon", "Ramos", "Reyes", "Burns", "Gordon", "Shaw", "Holmes", "Rice",
		"Robertson", "Hunt", "Black", "Daniels", "Palmer", "Mills", "Nichols", "Grant",
	}

	// Shuffle the name arrays to add more randomness
	shuffleStrings(firstNames)
	shuffleStrings(lastNames)

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

	cities := []string{
		"Seattle", "San Francisco", "New York", "Los Angeles", "Chicago", "Boston",
		"Austin", "Denver", "Portland", "Atlanta", "Miami", "Dallas", "Phoenix",
		"San Diego", "Las Vegas", "Detroit", "Minneapolis", "Tampa", "Orlando",
		"Nashville", "Charlotte", "Pittsburgh", "Cleveland", "Cincinnati",
	}

	countries := []string{
		"United States", "Canada", "United Kingdom", "Germany", "France", "Australia",
		"Netherlands", "Sweden", "Norway", "Denmark", "Finland", "Switzerland",
		"Austria", "Belgium", "Ireland", "Spain", "Italy", "Portugal",
	}

	phoneAreaCodes := []string{
		"206", "415", "212", "310", "312", "617", "512", "303", "503", "404",
		"305", "214", "602", "619", "702", "313", "612", "813", "407", "615",
	}

	// Track used usernames to ensure uniqueness
	usedUsernames := make(map[string]bool)

	// Generate users in batches and save to files
	allUsers := make([]User, 0)
	batchSize := usersPerFile

	// Only keep first batch in memory for 1M+ users
	keepInMemory := count <= maxUsersInMemory

	for batchStart := 0; batchStart < count; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > count {
			batchEnd = count
		}

		batchUsers := make([]User, batchEnd-batchStart)

		for i := 0; i < len(batchUsers); i++ {
			globalIndex := batchStart + i

			// Use random selection instead of sequential
			firstName := firstNames[randomInt(len(firstNames))]
			lastName := lastNames[randomInt(len(lastNames))]

			displayName := fmt.Sprintf("%s %s", firstName, lastName)
			username := generateUniqueUsername(firstName, lastName, usedUsernames, 0)
			userPrincipalName := fmt.Sprintf("%s@%s", username, domain)

			// Generate a UUID-like ID
			userID := fmt.Sprintf("%08d-%04d-%04d-%04d-%012d",
				10000000+globalIndex, 1000+globalIndex%1000, 2000+globalIndex%2000, 3000+globalIndex%3000, 100000000000+globalIndex)

			// Generate phone numbers
			areaCode := phoneAreaCodes[randomInt(len(phoneAreaCodes))]
			businessPhone := fmt.Sprintf("+1 %s %03d %04d", areaCode, 200+(globalIndex%800), 1000+(globalIndex%9000))
			mobilePhone := fmt.Sprintf("+1 %s %03d %04d", areaCode, 200+(globalIndex%800), 2000+(globalIndex%8000))
			faxNumber := fmt.Sprintf("+1 %s %03d %04d", areaCode, 200+(globalIndex%800), 3000+(globalIndex%7000))

			// Generate timestamps
			createdTime := time.Now().AddDate(0, 0, -(globalIndex % 365 * 2))   // Random date within last 2 years
			passwordChangeTime := time.Now().AddDate(0, 0, -(globalIndex % 90)) // Random date within last 90 days

			batchUsers[i] = User{
				ID:                         userID,
				AccountEnabled:             true, // Most users are enabled
				BusinessPhones:             []string{businessPhone},
				City:                       cities[randomInt(len(cities))],
				Country:                    countries[randomInt(len(countries))],
				CreatedDateTime:            createdTime.UTC().Format(time.RFC3339),
				Department:                 departments[randomInt(len(departments))],
				DisplayName:                displayName,
				FaxNumber:                  faxNumber,
				GivenName:                  firstName,
				JobTitle:                   jobTitles[randomInt(len(jobTitles))],
				LastPasswordChangeDateTime: passwordChangeTime.UTC().Format(time.RFC3339),
				Mail:                       userPrincipalName,
				MobilePhone:                mobilePhone,
				OfficeLocation:             fmt.Sprintf("%s, %s", buildings[randomInt(len(buildings))], floors[randomInt(len(floors))]),
				Surname:                    lastName,
				UserPrincipalName:          userPrincipalName,
			}
		}

		// Save this batch to file
		if err := saveUsersToFile(batchUsers, batchStart); err != nil {
			log.Printf("⚠️ Warning: Could not save user batch to file: %v", err)
		}

		// Only keep users in memory if dataset is small enough
		if keepInMemory || batchStart == 0 {
			allUsers = append(allUsers, batchUsers...)
		}

		// Progress update for large datasets
		if count > 50000 && (batchStart+batchSize)%50000 == 0 {
			log.Printf("📈 Progress: %d/%d users generated (%.1f%%)",
				batchStart+batchSize, count, float64(batchStart+batchSize)/float64(count)*100)
		}
	}

	// Update total count and log performance
	totalUserCount = count
	totalTime := time.Since(generationStart)

	log.Printf("✅ User generation completed in %v (%d users, %.0f users/sec)",
		totalTime, count, float64(count)/totalTime.Seconds())

	return allUsers
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

// Generate Jamf mock departments and computers for testing CollectUserInfo
func generateJamfData(count int, dupes bool) {
	deptNames := []string{
		"Engineering", "Product", "Marketing", "Sales", "Human Resources",
		"Finance", "Operations", "Customer Success", "Information Technology", "Security",
	}
	mockJamfDepartments = make([]JamfDepartment, len(deptNames))
	for i, name := range deptNames {
		mockJamfDepartments[i] = JamfDepartment{ID: i + 1, Name: name}
	}

	firstNames := []string{
		"Alice", "Bob", "Carol", "David", "Eve", "Frank", "Grace", "Henry",
		"Ivy", "Jack", "Kate", "Liam", "Mia", "Noah", "Olivia", "Paul",
		"Quinn", "Rachel", "Sam", "Tina", "Uma", "Victor", "Wendy", "Xavier",
		"Yara", "Zack", "Amy", "Ben", "Cara", "Diana",
	}
	lastNames := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
		"Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Wilson",
		"Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee",
		"Perez", "Thompson", "White", "Harris", "Clark", "Ramirez", "Lewis",
		"Robinson", "Walker", "Young",
	}
	models := []string{
		"MacBook Pro 14-inch", "MacBook Pro 16-inch", "MacBook Air M2",
		"Mac mini M2", "iMac 24-inch",
	}
	positions := []string{
		"Engineer", "Manager", "Director", "Analyst", "Designer",
		"Specialist", "Coordinator", "Lead", "Architect", "Consultant",
	}

	// Determine how many records get duplicate serials (~10% when dupes=true)
	dupeCount := 0
	if dupes && count > 1 {
		dupeCount = count / 10
		if dupeCount < 1 {
			dupeCount = 1
		}
	}
	uniqueCount := count - dupeCount

	mockJamfComputers = make([]JamfComputer, count)
	for i := 0; i < count; i++ {
		firstName := firstNames[randomInt(len(firstNames))]
		lastName := lastNames[randomInt(len(lastNames))]
		username := fmt.Sprintf("%s.%s", strings.ToLower(firstName), strings.ToLower(lastName))
		deptID := (i % len(mockJamfDepartments)) + 1

		// Unique serial for most records; dupes borrow an earlier record's serial
		serial := fmt.Sprintf("C%08dXXXX", i+1)
		if dupes && i >= uniqueCount {
			srcIdx := randomInt(uniqueCount)
			serial = mockJamfComputers[srcIdx].General.SerialNumber
		}

		mockJamfComputers[i] = JamfComputer{
			ID:   fmt.Sprintf("%d", i+1),
			UDID: fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", i+1, 0xABCD, 0x1234, 0x5678, i+1),
			General: JamfComputerGeneral{
				Name:         fmt.Sprintf("mac-%s-%03d", strings.ToLower(lastName), i+1),
				SerialNumber: serial,
			},
			Hardware: JamfComputerHardware{
				Model: models[randomInt(len(models))],
			},
			UserAndLocation: JamfComputerUserAndLocation{
				Username:     username,
				RealName:     fmt.Sprintf("%s %s", firstName, lastName),
				EmailAddress: fmt.Sprintf("%s@mock.jamf.local", username),
				Position:     positions[randomInt(len(positions))],
				Phone:        fmt.Sprintf("555-%04d", i+1),
				DepartmentId: deptID,
			},
		}
	}

	if dupes {
		log.Printf("✅ Jamf mock data: %d departments, %d computers (%d with duplicate serials)", len(mockJamfDepartments), count, dupeCount)
	} else {
		log.Printf("✅ Jamf mock data: %d departments, %d computers (all unique serials)", len(mockJamfDepartments), count)
	}
}

// Request logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request details
		log.Printf("📥 [%s] %s %s", r.Method, r.URL.Path, r.URL.RawQuery)
		log.Printf("   Headers: %v", r.Header)
		log.Printf("   Remote Addr: %s", r.RemoteAddr)
		log.Printf("   User Agent: %s", r.UserAgent())

		// Create a custom response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process the request
		next.ServeHTTP(wrapped, r)

		// Log response details
		duration := time.Since(start)
		log.Printf("📤 [%s] %s -> %d (%v)", r.Method, r.URL.Path, wrapped.statusCode, duration)
		log.Printf("   ─────────────────────────────────────────")
	})
}

// Custom response writer to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
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
	requestStart := time.Now()
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

	log.Printf("🔍 API Request: GET /v1.0/users?$skip=%d&$top=%d", skip, top)

	// Log any additional query parameters
	if selectParam := r.URL.Query().Get("$select"); selectParam != "" {
		log.Printf("   $select: %s", selectParam)
	}
	if r.URL.Query().Get("$count") == "true" {
		log.Printf("   $count: true")
	}

	// Get total count from files or memory
	var totalUsers int
	if totalUserCount > 0 {
		totalUsers = totalUserCount
	} else {
		totalUsers = getTotalUserCountFromFiles()
		if totalUsers == 0 {
			totalUsers = len(mockUsers)
		}
	}

	// Check bounds
	if skip >= totalUsers {
		// Return empty result if skip is beyond available data
		response := GraphResponse{
			Context: "https://graph.microsoft.com/v1.0/$metadata#users",
			Value:   []User{},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Calculate actual count to return
	actualCount := top
	if skip+top > totalUsers {
		actualCount = totalUsers - skip
	}

	var users []User
	var err error

	// Try to load from files first, fall back to in-memory if needed
	if totalUserCount > 0 && skip+actualCount > len(mockUsers) {
		users, err = loadUsersFromFile(skip, actualCount)
		if err != nil || len(users) == 0 {
			// Fall back to in-memory users if file loading fails
			end := skip + actualCount
			if end > len(mockUsers) {
				end = len(mockUsers)
			}
			if skip < len(mockUsers) {
				users = mockUsers[skip:end]
			} else {
				users = []User{}
			}
		}
	} else {
		// Use in-memory users
		end := skip + actualCount
		if end > len(mockUsers) {
			end = len(mockUsers)
		}
		if skip < len(mockUsers) {
			users = mockUsers[skip:end]
		} else {
			users = []User{}
		}
	}

	// Build response exactly like Graph API
	response := GraphResponse{
		Context: "https://graph.microsoft.com/v1.0/$metadata#users",
		Value:   users,
	}

	// Add nextLink if there are more results (Graph API behavior)
	if skip+len(users) < totalUsers {
		nextSkip := skip + len(users)

		// Build nextLink URL exactly like Microsoft Graph API
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		// Check for forwarded protocol headers (common in production)
		if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
			scheme = forwarded
		}

		host := r.Host
		if host == "" {
			port := os.Getenv("PORT")
			if port == "" {
				port = "8080"
			}
			host = fmt.Sprintf("localhost:%s", port)
		}

		// Build the nextLink with proper query parameters
		nextLink := fmt.Sprintf("%s://%s/v1.0/users?$skip=%d", scheme, host, nextSkip)

		// Preserve the $top parameter if specified
		if topParam := r.URL.Query().Get("$top"); topParam != "" {
			nextLink += fmt.Sprintf("&$top=%s", topParam)
		}

		// Preserve the $count parameter if specified
		if r.URL.Query().Get("$count") == "true" {
			nextLink += "&$count=true"
		}

		// Preserve any $select parameters
		if selectParam := r.URL.Query().Get("$select"); selectParam != "" {
			nextLink += fmt.Sprintf("&$select=%s", selectParam)
		}

		response.NextLink = nextLink
		log.Printf("🔗 NextLink generated: %s", nextLink)
	}

	// Add count if requested (Graph API behavior)
	if r.URL.Query().Get("$count") == "true" {
		response.Count = totalUsers
	}

	// Log performance metrics
	responseTime := time.Since(requestStart)
	hasNextLink := response.NextLink != ""
	log.Printf("⚡ Response sent in %v (%d users returned, hasNextLink: %t)", responseTime, len(users), hasNextLink)

	json.NewEncoder(w).Encode(response)
}

// Get user by ID
func getUserByID(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	userID := vars["id"]

	log.Printf("🔍 API Request: GET /v1.0/users/%s", userID)

	// First check in-memory users
	for _, user := range mockUsers {
		if user.ID == userID || user.UserPrincipalName == userID {
			responseTime := time.Since(requestStart)
			log.Printf("⚡ User found in memory in %v", responseTime)
			json.NewEncoder(w).Encode(user)
			return
		}
	}

	// Use index for fast file lookup if available
	if userIndex != nil {
		if indexEntry, exists := userIndex[userID]; exists {
			fileIndex := indexEntry.FileIndex
			filename := filepath.Join(userDataDir, fmt.Sprintf("users_%05d.json", fileIndex))

			file, err := os.Open(filename)
			if err == nil {
				var userFile UserFile
				if err := json.NewDecoder(file).Decode(&userFile); err == nil {
					if indexEntry.PositionInFile < len(userFile.Users) {
						user := userFile.Users[indexEntry.PositionInFile]
						if user.ID == userID || user.UserPrincipalName == userID {
							file.Close()
							responseTime := time.Since(requestStart)
							log.Printf("⚡ User found via index in %v (file %d)", responseTime, fileIndex)
							json.NewEncoder(w).Encode(user)
							return
						}
					}
				}
				file.Close()
			}
		}
	}

	// If not found in memory and we have file-based storage, search files
	if totalUserCount > len(mockUsers) {
		files, err := filepath.Glob(filepath.Join(userDataDir, "users_*.json"))
		if err == nil {
			for _, filename := range files {
				file, err := os.Open(filename)
				if err != nil {
					continue
				}

				var userFile UserFile
				if err := json.NewDecoder(file).Decode(&userFile); err == nil {
					for _, user := range userFile.Users {
						if user.ID == userID || user.UserPrincipalName == userID {
							file.Close()
							responseTime := time.Since(requestStart)
							log.Printf("⚡ User found in file in %v", responseTime)
							json.NewEncoder(w).Encode(user)
							return
						}
					}
				}
				file.Close()
			}
		}
	}

	responseTime := time.Since(requestStart)
	log.Printf("⚡ User not found (searched in %v)", responseTime)
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
		totalCount := totalUserCount
		if totalCount == 0 {
			totalCount = getTotalUserCountFromFiles()
			if totalCount == 0 {
				totalCount = len(mockUsers)
			}
		}

		config := UserConfig{
			UserCount: totalCount,
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

		if config.UserCount > 2000000 {
			sendErrorResponse(w, "invalid_request", "User count cannot exceed 2,000,000", http.StatusBadRequest)
			return
		}

		// Clear existing user data
		clearStart := time.Now()
		if err := os.RemoveAll(userDataDir); err != nil {
			log.Printf("⚠️ Warning: Could not clear existing user data: %v", err)
		} else {
			clearTime := time.Since(clearStart)
			log.Printf("🗑️ Cleared existing user data in %v", clearTime)
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
			"userCount": totalUserCount,
			"domain":    domain,
			"inMemory":  len(mockUsers),
			"onDisk":    totalUserCount - len(mockUsers),
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
	if err != nil || count < 1 || count > 2000000 {
		sendErrorResponse(w, "invalid_request", "Count must be a number between 1 and 2,000,000", http.StatusBadRequest)
		return
	}

	// Get domain from query parameter
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		domain = "contoso.com"
	}

	// Clear existing user data
	if err := os.RemoveAll(userDataDir); err != nil {
		log.Printf("⚠️ Warning: Could not clear existing user data: %v", err)
	}

	// Generate users
	mockUsers = generateMockUsers(count, domain)

	log.Printf("🔄 Bulk generated %d mock users with domain %s", count, domain)

	response := map[string]interface{}{
		"message":   fmt.Sprintf("Successfully generated %d users", count),
		"userCount": totalUserCount,
		"domain":    domain,
		"endpoint":  "/v1.0/users",
		"inMemory":  len(mockUsers),
		"onDisk":    totalUserCount - len(mockUsers),
	}
	json.NewEncoder(w).Encode(response)
}

// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()
	w.Header().Set("Content-Type", "application/json")

	totalCount := totalUserCount
	if totalCount == 0 {
		totalCount = getTotalUserCountFromFiles()
		if totalCount == 0 {
			totalCount = len(mockUsers)
		}
	}

	// Calculate file count
	files, _ := filepath.Glob(filepath.Join(userDataDir, "users_*.json"))
	fileCount := len(files)

	response := map[string]interface{}{
		"status":       "healthy",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"service":      "MDM Mock Service",
		"version":      "1.0.0",
		"userCount":    totalCount,
		"inMemory":     len(mockUsers),
		"onDisk":       totalCount - len(mockUsers),
		"usersPerFile": usersPerFile,
		"fileCount":    fileCount,
		"responseTime": time.Since(requestStart).String(),
	}

	log.Printf("⚡ Health check completed in %v", time.Since(requestStart))
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

// ─── Jamf Pro API handlers ────────────────────────────────────────────────────

// POST /v1/auth/token – issue a mock bearer token (accepts any Basic credentials)
func jamfAuthToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		sendErrorResponse(w, "INVALID_TOKEN", "Basic authorization header required", http.StatusUnauthorized)
		return
	}

	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	response := map[string]string{
		"token":   "mock-jamf-token",
		"expires": expires,
	}
	json.NewEncoder(w).Encode(response)
}

// GET /v1/departments/ – return all mock departments
func jamfGetDepartments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := JamfPagedResponse{
		TotalCount: len(mockJamfDepartments),
		Results:    mockJamfDepartments,
	}
	json.NewEncoder(w).Encode(response)
}

// GET /v1/computers-inventory – paginated list of mock computers
func jamfGetComputersInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	page := 0
	pageSize := 500

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page-size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 {
			pageSize = v
		}
	}

	total := len(mockJamfComputers)
	start := page * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response := JamfPagedResponse{
		TotalCount: total,
		Results:    mockJamfComputers[start:end],
	}
	json.NewEncoder(w).Encode(response)
}

func main() {
	mode := flag.String("mode", "", "Service mode: o365 or jamf")
	count := flag.Int("count", 0, "Number of records to generate (users for o365, computers for jamf)")
	dupes := flag.Bool("dupes", false, "Jamf only: generate ~10% duplicate serial numbers")
	flag.Parse()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if *mode == "" {
		fmt.Fprintf(os.Stderr, "MDM Mock Service\n\nUsage:\n")
		fmt.Fprintf(os.Stderr, "  -mode=o365   Microsoft Graph API mock\n")
		fmt.Fprintf(os.Stderr, "  -mode=jamf   Jamf Pro API mock\n")
		fmt.Fprintf(os.Stderr, "  -count=N     Number of records to generate\n")
		fmt.Fprintf(os.Stderr, "  -dupes       (Jamf only) ~10%% of computers share a serial with another\n\n")
		fmt.Fprintf(os.Stderr, "Use start.sh for a friendlier interface.\n")
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	r.Use(corsMiddleware)
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/", rootEndpoint).Methods("GET")

	switch *mode {
	case "o365":
		n := 100
		if *count > 0 {
			n = *count
		}
		domain := "contoso.com"
		if envDomain := os.Getenv("DOMAIN"); envDomain != "" {
			domain = envDomain
		}

		log.Printf("🔄 [O365 mode] Generating %d mock users (domain: %s)...", n, domain)
		initStart := time.Now()
		mockUsers = generateMockUsers(n, domain)
		initTime := time.Since(initStart)
		log.Printf("✅ Generated %d users in %v", n, initTime)

		r.HandleFunc("/config/users", configureUsers).Methods("GET", "POST")
		r.HandleFunc("/generate/users", bulkGenerateUsers).Methods("POST")

		apiRouter := r.PathPrefix("/v1.0").Subrouter()
		apiRouter.Use(authMiddleware)
		apiRouter.HandleFunc("/users", getUsers).Methods("GET")
		apiRouter.HandleFunc("/users/{id}", getUserByID).Methods("GET")
		apiRouter.HandleFunc("/me", getCurrentUser).Methods("GET")
		apiRouter.HandleFunc("/groups", getGroups).Methods("GET")
		apiRouter.HandleFunc("/groups/{id}", getGroupByID).Methods("GET")

		log.Printf("")
		log.Printf("🚀 MDM Mock Service [O365 mode] on :%s", port)
		log.Printf("   Users:  http://localhost:%s/v1.0/users  (%d users, page size 100)", port, n)
		log.Printf("   Groups: http://localhost:%s/v1.0/groups", port)
		log.Printf("   Me:     http://localhost:%s/v1.0/me", port)
		log.Printf("   Config: http://localhost:%s/config/users", port)
		log.Printf("")
		log.Printf("   curl -H 'Authorization: Bearer test' http://localhost:%s/v1.0/users", port)

	case "jamf":
		n := 500
		if *count > 0 {
			n = *count
		}

		generateJamfData(n, *dupes)

		r.HandleFunc("/v1/auth/token", jamfAuthToken).Methods("POST")
		jamfRouter := r.PathPrefix("/v1").Subrouter()
		jamfRouter.Use(authMiddleware)
		jamfRouter.HandleFunc("/departments/", jamfGetDepartments).Methods("GET")
		jamfRouter.HandleFunc("/computers-inventory", jamfGetComputersInventory).Methods("GET")

		dupesNote := "no duplicate serials"
		if *dupes {
			dupesNote = fmt.Sprintf("~%d duplicate serials injected", n/10)
		}
		log.Printf("")
		log.Printf("🚀 MDM Mock Service [Jamf mode] on :%s", port)
		log.Printf("   Computers: %d  (%s)", n, dupesNote)
		log.Printf("   Token:     POST http://localhost:%s/v1/auth/token  (any Basic creds)", port)
		log.Printf("   Depts:     GET  http://localhost:%s/v1/departments/", port)
		log.Printf("   Inventory: GET  http://localhost:%s/v1/computers-inventory", port)
		log.Printf("")
		log.Printf("   >>> Set JamfServer URL to: http://localhost:%s <<<", port)

	default:
		log.Fatalf("❌ Unknown mode %q — use -mode=o365 or -mode=jamf", *mode)
	}

	log.Printf("")
	log.Fatal(http.ListenAndServe(":"+port, r))
}
