package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	_ "github.com/mattn/go-sqlite3"
)

// ==========================================
// 1. DATA STRUCTURES & CONFIG
// ==========================================

// 🚨 HELPER: Converts standard GPS floats to deterministic integers for Hyperledger
func toMicrodegrees(coord float64) int64 {
	return int64(coord * 1000000)
}

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Println("WARNING: JWT_SECRET env var not set. Using fallback development key.")
		return []byte("super_secret_thesis_key_2026")
	}
	return []byte(secret)
}

func verifyJWT(r *http.Request) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, fmt.Errorf("invalid token format")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	return claims, nil
}

var jwtKey = getJWTSecret()

type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}
type GeoJSONFeature struct {
	Type       string            `json:"type"`
	Geometry   GeoJSONGeometry   `json:"geometry"`
	Properties GeoJSONProperties `json:"properties"`
}
type GeoJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type GeoJSONProperties struct {
	Name           string `json:"Name"`
	Icon           string `json:"icon"`
	Owner          string `json:"owner"`
	PricePerMinute int64  `json:"pricePerMinute"`
	Type           string `json:"type"`
	CO2SavingsRate int64  `json:"co2Rate"`
	Make           string `json:"make"`
	Model          string `json:"model"`
	CarClass       string `json:"carClass"`
	Transmission   string `json:"transmission"`
	Seats          int    `json:"seats"`
	Mileage        string `json:"mileage"`
	FuelType       string `json:"fuelType"`
	BatteryLevel   string `json:"batteryLevel"`
	ImageUrl       string `json:"imageUrl"`
	OwnerType      string `json:"ownerType"`
	AvgRating      string `json:"avgRating"`
}

type ChaincodeAsset struct {
	ID             string `json:"ID"`
	Type           string `json:"Type"`
	Make           string `json:"Make"`
	Model          string `json:"Model"`
	CarClass       string `json:"CarClass"`
	Transmission   string `json:"Transmission"`
	Seats          int    `json:"Seats"`
	Mileage        string `json:"Mileage"`
	FuelType       string `json:"FuelType"`
	BatteryLevel   string `json:"BatteryLevel"`
	Owner          string `json:"Owner"`
	Status         string `json:"Status"`
	PricePerMinute int64  `json:"PricePerMinute"`
	StartTime      int64  `json:"StartTime"`
	CurrentRenter  string `json:"CurrentRenter"`
	CO2SavingsRate int64  `json:"CO2SavingsRate"`
	BaseLatMicro   int64  `json:"BaseLatMicro"`
	BaseLonMicro   int64  `json:"BaseLonMicro"`
}

type LoginRequest struct {
	UserID   string `json:"userId"`
	Password string `json:"password"`
}
type RentRequest struct {
	VehicleID string `json:"vehicleId"`
}
type ReturnRequest struct {
	VehicleID string  `json:"vehicleId"`
	ReturnLat float64 `json:"returnLat"`
	ReturnLon float64 `json:"returnLon"`
}
type TopUpRequest struct {
	Amount int `json:"amount"`
}
type ToggleRequest struct {
	VehicleID string `json:"vehicleId"`
}
type RateRequest struct {
	TripID  string  `json:"tripId"`
	Stars   float64 `json:"stars"`
	Comment string  `json:"comment"`
}

var contract *gateway.Contract
var db *sql.DB

// ==========================================
// 2. MAIN FUNCTION & DB SETUP
// ==========================================
func main() {
	os.Setenv("DISCOVERY_AS_LOCALHOST", "true")
	log.Println("Starting Server")

	initDB()
	defer db.Close()

	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}
	if !wallet.Exists("appUser") {
		log.Fatal("Run 'go run enroll.go' first.")
	}

	ccpPath := "/home/were_brian329/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/connection-org1.yaml"
	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile(filepath.Clean(ccpPath))),
		gateway.WithIdentity(wallet, "appUser"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	network, err := gw.GetNetwork("carshare")
	if err != nil {
		log.Fatalf("Failed to get network: %v", err)
	}
	contract = network.GetContract("basic")
	log.Println("DB Connected and Blockchain Gateway Initialized")

	// Standard Routes
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/rent", rentAssetHandler)
	http.HandleFunc("/api/return", returnAssetHandler)
	http.HandleFunc("/api/history", getHistoryHandler)
	http.HandleFunc("/api/wallet", getWalletHandler)
	http.HandleFunc("/api/map/cars.geojson", getGeoJSONFeed)
	http.HandleFunc("/api/faucet", topUpHandler)

	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/create-asset", createAssetHandler)
	http.HandleFunc("/api/admin/approve-asset", approveAssetHandler)
	http.HandleFunc("/api/admin/pending", getPendingAssetsHandler)
	http.HandleFunc("/api/eco", getEcoStatsHandler)

	// User & Host Interaction Routes
	http.HandleFunc("/api/toggle-status", toggleStatusHandler)
	http.HandleFunc("/api/user-trips", getUserTripsHandler)
	http.HandleFunc("/api/rate", rateTripHandler)
	http.HandleFunc("/api/rate-vehicle", rateVehicleHandler)

	// Serve uploaded images statically
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	log.Println("Server listening on port 9000...")
	if err := http.ListenAndServe("0.0.0.0:9000", nil); err != nil {
		log.Fatal(err)
	}
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./offchain_metadata.db")
	if err != nil {
		log.Fatalf("Failed to open SQLite database: %v", err)
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		log.Fatalf("Failed to enable foreign keys: %v", err)
	}

	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		phone_number TEXT,
		dob DATE,
		license_number TEXT,
		license_pic_path TEXT,
		is_verified BOOLEAN DEFAULT 0,
		reputation_score REAL DEFAULT 5.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	db.Exec(createUsersTable)

	createVehiclesTable := `
	CREATE TABLE IF NOT EXISTS vehicles (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL,
		make TEXT,
		model TEXT,
		vehicle_type TEXT, 
		car_class TEXT,
		transmission TEXT,
		seats INTEGER,
		mileage TEXT,
		fuel_type TEXT,
		battery_level TEXT,
		latitude REAL,
		longitude REAL,
		co2_savings_rate INTEGER,
		avg_rating REAL DEFAULT 5.0,
		image_path TEXT,
		FOREIGN KEY(owner_id) REFERENCES users(id)
	);`
	db.Exec(createVehiclesTable)

	createTripsTable := `
	CREATE TABLE IF NOT EXISTS trips (
		id TEXT PRIMARY KEY,
		driver_id TEXT,
		vehicle_id TEXT,
		start_lat REAL,
		start_lon REAL,
		end_lat REAL,
		end_lon REAL,
		distance_km REAL,
		co2_saved REAL,
		start_time DATETIME,
		end_time DATETIME,
		status TEXT,
		FOREIGN KEY(driver_id) REFERENCES users(id),
		FOREIGN KEY(vehicle_id) REFERENCES vehicles(id)
	);`
	db.Exec(createTripsTable)

	createUserRatings := `
	CREATE TABLE IF NOT EXISTS user_ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rater_id TEXT,
		rated_user_id TEXT,
		trip_id TEXT,
		rating INTEGER CHECK(rating BETWEEN 1 AND 5),
		comment TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(rater_id) REFERENCES users(id),
		FOREIGN KEY(rated_user_id) REFERENCES users(id),
		FOREIGN KEY(trip_id) REFERENCES trips(id)
	);`
	db.Exec(createUserRatings)

	createVehicleRatings := `
	CREATE TABLE IF NOT EXISTS vehicle_ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		vehicle_id TEXT,
		trip_id TEXT, -- 🚨 CHANGED THIS FROM INTEGER TO TEXT
		rating INTEGER CHECK(rating BETWEEN 1 AND 5),
		comment TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id),
		FOREIGN KEY(vehicle_id) REFERENCES vehicles(id),
		FOREIGN KEY(trip_id) REFERENCES trips(id)
	);`
	db.Exec(createVehicleRatings)

	createEcoStats := `
	CREATE TABLE IF NOT EXISTS eco_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		total_trips INTEGER DEFAULT 0,
		total_distance REAL DEFAULT 0,
		total_co2_saved REAL DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	db.Exec(createEcoStats)

	// ---------------- SEED DATA ----------------
	seedUsers := `
	INSERT OR IGNORE INTO users (id, name, email, password_hash) VALUES 
	('admin', 'Admin User', 'admin@chainride.com', 'adminpass'),
	('Anna', 'Anna Smith', 'anna@example.com', 'userpass1'),
	('Brian', 'Brian Jones', 'brian@example.com', 'userpass2');
	`
	db.Exec(seedUsers)

	seedVehicles := `
	INSERT OR IGNORE INTO vehicles (id, owner_id, make, model, vehicle_type, car_class, transmission, seats, mileage, fuel_type, battery_level, latitude, longitude, co2_savings_rate) VALUES 
	('CAR_001', 'Anna', 'Tesla', 'Model 3', 'Car', 'Sedan', 'Automatic', 5, '14,500 km', 'Electric', '84%', 46.2521, 20.1410, 100),
	('SCOOTER_001', 'Brian', 'Xiaomi', 'M365', 'Scooter', 'Micro', 'N/A', 1, '850 km', 'Electric', '100%', 46.2530, 20.1414, 130);
	`
	db.Exec(seedVehicles)

	log.Println("✅ SQLite Database initialized successfully with Web3-mapped relational schema.")
}

// ==========================================
// 3. AUTHENTICATION & HELPERS
// ==========================================

func loginHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		http.Error(w, "Invalid request body", 400)
		return
	}

	if req.UserID == "appUser" {
		adminPin := os.Getenv("ADMIN_PIN")
		if adminPin == "" {
			adminPin = "admin123"
		}
		if req.Password != adminPin {
			http.Error(w, `{"error": "Forbidden: Invalid Admin Credentials"}`, 403)
			return
		}
	} else {
		_, err := contract.EvaluateTransaction("GetUser", req.UserID)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "User %s is not registered on the network"}`, req.UserID), 404)
			return
		}
	}

	role := "USER"
	if req.UserID == "appUser" || req.UserID == "admin" {
		role = "ADMIN"
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: req.UserID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, "Failed to generate token", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func getEffectiveUser(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, "Bearer ")
	if len(parts) != 2 {
		return ""
	}
	tokenString := strings.TrimSpace(parts[1])

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		return ""
	}
	return claims.UserID
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// ==========================================
// 4. API ROUTE HANDLERS
// ==========================================

func getGeoJSONFeed(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	result, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		http.Error(w, "Failed to read blockchain: "+err.Error(), 500)
		return
	}

	var onChainAssets []ChaincodeAsset
	if err := json.Unmarshal(result, &onChainAssets); err != nil {
		http.Error(w, "Fatal Error parsing blockchain data: "+err.Error(), 500)
		return
	}

	features := []GeoJSONFeature{}

	for _, asset := range onChainAssets {
		if asset.Status != "AVAILABLE" {
			continue
		}

		var lat, lon float64
		var imagePath string
		var avgRating float64

		err := db.QueryRow("SELECT latitude, longitude, image_path, avg_rating FROM vehicles WHERE id = ?", asset.ID).Scan(&lat, &lon, &imagePath, &avgRating)
		if err != nil {
			lat, lon = float64(asset.BaseLatMicro)/1000000.0, float64(asset.BaseLonMicro)/1000000.0
			avgRating = 5.0
		}

		iconUrl := "https://img.icons8.com/color/48/000000/car-top-view.png"
		if asset.Type == "Scooter" {
			iconUrl = "https://img.icons8.com/color/48/000000/scooter.png"
		} else if asset.Type == "Bike" {
			iconUrl = "https://img.icons8.com/color/48/000000/bicycle.png"
		}

		imgUrl := ""
		if imagePath != "" {
			imgUrl = fmt.Sprintf("http://localhost:9000/%s", imagePath) // Replace with your IP if external
		}

		features = append(features, GeoJSONFeature{
			Type: "Feature",
			Geometry: GeoJSONGeometry{Type: "Point", Coordinates: []float64{lon, lat}},
			Properties: GeoJSONProperties{
				Name:           asset.ID,
				Icon:           iconUrl,
				Owner:          asset.Owner,
				PricePerMinute: asset.PricePerMinute,
				Type:           asset.Type,
				CO2SavingsRate: asset.CO2SavingsRate,
				Make:           asset.Make,
				Model:          asset.Model,
				CarClass:       asset.CarClass,
				Transmission:   asset.Transmission,
				Seats:          asset.Seats,
				Mileage:        asset.Mileage,
				FuelType:       asset.FuelType,
				BatteryLevel:   asset.BatteryLevel,
				ImageUrl:       imgUrl,
				AvgRating:      fmt.Sprintf("%.1f", avgRating),
				OwnerType:      "Private Host",
			},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GeoJSONFeatureCollection{Type: "FeatureCollection", Features: features})
}

func createAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	claims, err := verifyJWT(r)
	if err != nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}
	ownerID := claims.UserID

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error": "Failed to parse form data"}`, http.StatusBadRequest)
		return
	}

	price, _ := strconv.ParseInt(r.FormValue("price"), 10, 64)
	co2Rate, _ := strconv.ParseInt(r.FormValue("co2Rate"), 10, 64)
	seats, _ := strconv.Atoi(r.FormValue("seats"))
	lat, _ := strconv.ParseFloat(r.FormValue("lat"), 64)
	lon, _ := strconv.ParseFloat(r.FormValue("lon"), 64)

	payload := ChaincodeAsset{
		ID:             r.FormValue("id"),
		Type:           r.FormValue("assetType"),
		Make:           r.FormValue("make"),
		Model:          r.FormValue("model"),
		CarClass:       r.FormValue("carClass"),
		Transmission:   r.FormValue("transmission"),
		Seats:          seats,
		Mileage:        r.FormValue("mileage"),
		FuelType:       r.FormValue("fuelType"),
		BatteryLevel:   r.FormValue("batteryLevel"),
		Owner:          ownerID,
		PricePerMinute: price,
		CO2SavingsRate: co2Rate,
		BaseLatMicro:   toMicrodegrees(lat),
		BaseLonMicro:   toMicrodegrees(lon),
	}

	payloadBytes, _ := json.Marshal(payload)

	_, err = contract.SubmitTransaction("CreateAsset", string(payloadBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to mint asset on blockchain: %v"}`, err), http.StatusInternalServerError)
		return
	}

	file, handler, err := r.FormFile("vehicleImage")
	var imagePath string
	if err == nil {
		defer file.Close()
		os.MkdirAll("uploads", os.ModePerm)
		filename := fmt.Sprintf("%s_%d%s", payload.ID, time.Now().Unix(), filepath.Ext(handler.Filename))
		imagePath = filepath.Join("uploads", filename)
		dst, _ := os.Create(imagePath)
		defer dst.Close()
		io.Copy(dst, file)
	}

	insertVehicleQuery := `
		INSERT INTO vehicles (id, owner_id, make, model, vehicle_type, car_class, transmission, seats, mileage, fuel_type, battery_level, latitude, longitude, co2_savings_rate, image_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	db.Exec(insertVehicleQuery, payload.ID, ownerID, payload.Make, payload.Model, payload.Type, payload.CarClass, payload.Transmission, payload.Seats, payload.Mileage, payload.FuelType, payload.BatteryLevel, lat, lon, payload.CO2SavingsRate, imagePath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Vehicle %s minted successfully! Pending Admin Approval.", payload.ID),
	})
}

func rentAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var req RentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VehicleID == "" {
		http.Error(w, `{"error": "Invalid request body or missing vehicleId"}`, 400)
		return
	}

	_, err := contract.SubmitTransaction("RentAsset", req.VehicleID, effectiveUser)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
		return
	}
	w.Write([]byte(`{"success": true, "message": "Vehicle unlocked"}`))
}

func returnAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var req ReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VehicleID == "" {
		http.Error(w, `{"error": "Invalid request body"}`, 400)
		return
	}

	latMicroStr := strconv.FormatInt(toMicrodegrees(req.ReturnLat), 10)
	lonMicroStr := strconv.FormatInt(toMicrodegrees(req.ReturnLon), 10)

	var asset ChaincodeAsset
	assetBytes, err := contract.EvaluateTransaction("ReadAsset", req.VehicleID)
	if err == nil {
		json.Unmarshal(assetBytes, &asset)
	}

	result, err := contract.SubmitTransaction("ReturnAsset", req.VehicleID, effectiveUser, latMicroStr, lonMicroStr)
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
		return
	}

	// 🚨 CATCH THE HASH ID FROM THE BLOCKCHAIN
	blockchainTripID := string(result)

	if asset.StartTime > 0 {
		endTime := time.Now().Unix()
		durationMins := (endTime - asset.StartTime) / 60
		if durationMins < 1 { durationMins = 1 }
		co2Saved := durationMins * asset.CO2SavingsRate

		// 🚨 SAVE THE CRYPTOGRAPHIC HASH AS THE PRIMARY KEY
		insertTripQuery := `
			INSERT INTO trips (id, driver_id, vehicle_id, start_time, end_time, co2_saved, status)
			VALUES (?, ?, ?, ?, ?, ?, 'Completed')
		`
		db.Exec(insertTripQuery, blockchainTripID, effectiveUser, req.VehicleID, time.Unix(asset.StartTime, 0), time.Unix(endTime, 0), co2Saved)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"receipt": blockchainTripID,
	})
}

func toggleStatusHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var req ToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, 400)
		return
	}

	_, err := contract.SubmitTransaction("ToggleAssetStatus", req.VehicleID, effectiveUser)
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func getPendingAssetsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	claims, err := verifyJWT(r)
	if err != nil || claims.Role != "ADMIN" {
		http.Error(w, `{"error": "Unauthorized: Admin access required"}`, http.StatusUnauthorized)
		return
	}

	result, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		http.Error(w, "Failed to read blockchain", http.StatusInternalServerError)
		return
	}

	var allAssets []ChaincodeAsset
	if err := json.Unmarshal(result, &allAssets); err != nil {
		http.Error(w, "Error parsing data", http.StatusInternalServerError)
		return
	}

	var pendingAssets []ChaincodeAsset
	for _, asset := range allAssets {
		if asset.Status == "PENDING" {
			pendingAssets = append(pendingAssets, asset)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingAssets)
}

func approveAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	claims, err := verifyJWT(r)
	if err != nil {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		VehicleID string `json:"vehicleId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	_, err = contract.SubmitTransaction("ApproveAsset", claims.UserID, req.VehicleID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to approve asset: %v"}`, err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Vehicle %s has been approved and is now AVAILABLE on the map!", req.VehicleID),
	})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, `{"error": "Failed to parse form data"}`, http.StatusBadRequest)
		return
	}

	userID := r.FormValue("username")
	password := r.FormValue("password")
	email := r.FormValue("email")
	fullName := r.FormValue("fullName")
	phoneNumber := r.FormValue("phoneNumber")
	dob := r.FormValue("dob")
	licenseNum := r.FormValue("licenseNumber")

	if userID == "" || password == "" || email == "" {
		http.Error(w, `{"error": "Missing critical fields"}`, http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("licenseImage")
	var imagePath string
	if err == nil {
		defer file.Close()
		os.MkdirAll("uploads", os.ModePerm)
		filename := fmt.Sprintf("%s_%d%s", userID, time.Now().Unix(), filepath.Ext(handler.Filename))
		imagePath = filepath.Join("uploads", filename)
		dst, _ := os.Create(imagePath)
		defer dst.Close()
		io.Copy(dst, file)
	}

	_, err = contract.SubmitTransaction("RegisterUser", userID, "0")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to register on blockchain: %v"}`, err), http.StatusInternalServerError)
		return
	}

	insertUserQuery := `
		INSERT INTO users (id, name, email, password_hash, phone_number, dob, license_number, license_pic_path) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	db.Exec(insertUserQuery, userID, fullName, email, password, phoneNumber, dob, licenseNum, imagePath)
	db.Exec(`INSERT INTO eco_stats (user_id) VALUES (?)`, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Welcome to ChainRide! Your profile is under review.",
	})
}

func getWalletHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	result, err := contract.EvaluateTransaction("GetUser", effectiveUser)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func topUpHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 {
		http.Error(w, `{"error": "Invalid request body or amount"}`, 400)
		return
	}

	if req.Amount > 5000 {
		http.Error(w, `{"error": "Faucet limit exceeded. Maximum request is 5000 tokens."}`, 400)
		return
	}

	amountStr := fmt.Sprintf("%d", req.Amount)
	_, err := contract.SubmitTransaction("TopUpWallet", effectiveUser, amountStr)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
		return
	}
	w.Write([]byte(`{"success": true, "message": "Testnet Faucet funded wallet successfully!"}`))
}

func getHistoryHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	vehicleID := r.URL.Query().Get("id")
	if vehicleID == "" {
		http.Error(w, `{"error": "Missing id parameter"}`, 400)
		return
	}

	result, err := contract.EvaluateTransaction("GetAssetHistory", vehicleID)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func getEcoStatsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var totalTrips int
	var totalCO2 float64

	err := db.QueryRow("SELECT total_trips, total_co2_saved FROM eco_stats WHERE user_id = ?", effectiveUser).Scan(&totalTrips, &totalCO2)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"totalTrips": 0, "totalCo2Saved": 0})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalTrips":    totalTrips,
		"totalCo2Saved": totalCO2,
	})
}

// 🗂️ USER TRIPS HISTORY (Mapped for React)
func getUserTripsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		effectiveUser = r.URL.Query().Get("id") // Fallback to React's URL param
	}

	// 🚨 FIX: Added v.image_path, ur.rating and ur.comment for the Trip Details Modal
	query := `
		SELECT t.id, v.make, v.model, v.id, v.owner_id, v.vehicle_type, v.image_path,
			   t.start_time, t.end_time, t.co2_saved, t.status,
			   ur.rating, ur.comment
		FROM trips t
		JOIN vehicles v ON t.vehicle_id = v.id
		LEFT JOIN user_ratings ur ON ur.trip_id = t.id AND ur.rater_id = ?
		WHERE t.driver_id = ?
		ORDER BY t.end_time DESC
	`
	rows, err := db.Query(query, effectiveUser, effectiveUser)
	if err != nil {
		// 1. Log the exact error to your Go terminal (cmd)
		log.Printf("DB Query Error: %v\n", err)
		
		// 2. Send the generic JSON error back to the React frontend
		http.Error(w, `{"error": "Failed to fetch trips"}`, 500)
		return
	}
	defer rows.Close()

	// Matches the Blockchain struct React expects exactly
	type ReactTrip struct {
		TripID       string  `json:"TripID"`
		AssetID      string  `json:"AssetID"`
		Owner        string  `json:"Owner"`
		EndTime      int64   `json:"EndTime"`
		DurationMins int64   `json:"DurationMins"`
		TotalCost    float64 `json:"TotalCost"`
		CO2Saved     float64 `json:"CO2Saved"`
		Status       string  `json:"Status"`
		RenterRated  bool    `json:"RenterRated"`
		ImagePath    string  `json:"ImagePath"` // Added for UI Modal
		Rating       int     `json:"Rating"`
		Comment      string  `json:"Comment"`
	}

	var trips []ReactTrip
	for rows.Next() {
		var id string
		var vMake, vModel, vId, vOwner, vType, status string
		var startTime, endTime time.Time
		var co2Saved sql.NullFloat64
		var imgPath, comment sql.NullString
		var rating sql.NullInt32

		// Updated Scan to match the new query
		err := rows.Scan(&id, &vMake, &vModel, &vId, &vOwner, &vType, &imgPath, &startTime, &endTime, &co2Saved, &status, &rating, &comment)
		if err != nil {
			log.Printf("Row Scan Error: %v\n", err)
			continue
		}

		durationMins := int64(endTime.Sub(startTime).Minutes())
		if durationMins < 1 {
			durationMins = 1
		}

		// Fallback calculation since price is only on the ledger
		var cost float64
		if vType == "Car" {
			cost = float64(durationMins) * 8.0 // 8 CRT per min for Cars
		} else {
			cost = float64(durationMins) * 3.0 // 3 CRT per min for Scooters
		}

		// Format Image correctly
		imgUrl := ""
		if imgPath.Valid && imgPath.String != "" {
			imgUrl = fmt.Sprintf("http://localhost:9000/%s", imgPath.String)
		}

		trips = append(trips, ReactTrip{
			TripID: id,
			AssetID:      fmt.Sprintf("%s %s (%s)", vMake, vModel, vId),
			Owner:        vOwner,
			EndTime:      endTime.Unix(),
			DurationMins: durationMins,
			TotalCost:    cost,
			CO2Saved:     co2Saved.Float64,
			Status:       status,
			RenterRated:  rating.Valid, // Hides stars if true
			ImagePath:    imgUrl,
			Rating:       int(rating.Int32),
			Comment:      comment.String,
		})
	}

	// Initialize empty array if null to prevent React map() crash
	if trips == nil {
		trips = []ReactTrip{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trips)
}

// 🌟 SUBMIT TRIP RATING TO THE BLOCKCHAIN
func rateTripHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" { effectiveUser = "Anna" }

	var req RateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, 400)
		return
	}

	// 1. SUBMIT RATING TO THE SMART CONTRACT
	starsStr := fmt.Sprintf("%.1f", req.Stars)
	_, err := contract.SubmitTransaction("RateTrip", req.TripID, effectiveUser, starsStr)
	if err != nil {
		log.Printf("Blockchain Rating Error: %v\n", err)
		http.Error(w, fmt.Sprintf(`{"error": "Blockchain rejected rating: %v"}`, err), 500)
		return
	}

	// 2. CACHE IT IN SQLITE SO THE UI UPDATES INSTANTLY
	_, err = db.Exec("INSERT INTO user_ratings (rater_id, trip_id, rating) VALUES (?, ?, ?)", effectiveUser, req.TripID, req.Stars)
	if err != nil {
		log.Printf("DB Cache Error: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// 🚗 SUBMIT VEHICLE RATING (100% Off-Chain SQLite)
type RateVehicleRequest struct {
	TripID    string  `json:"tripId"`
	VehicleID string  `json:"vehicleId"`
	Stars     float64 `json:"stars"`
}

func rateVehicleHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" { effectiveUser = "Anna" }

	var req RateVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, 400)
		return
	}

	// 1. Insert the specific trip rating
	_, err := db.Exec("INSERT INTO vehicle_ratings (user_id, vehicle_id, trip_id, rating) VALUES (?, ?, ?, ?)", effectiveUser, req.VehicleID, req.TripID, req.Stars)
	if err != nil {
		log.Printf("Vehicle Rating Error: %v\n", err)
		http.Error(w, "Failed to save vehicle rating", 500)
		return
	}

	// 2. Automatically recalculate and update the vehicle's overall average!
	_, err = db.Exec(`
		UPDATE vehicles 
		SET avg_rating = (SELECT AVG(rating) FROM vehicle_ratings WHERE vehicle_id = ?)
		WHERE id = ?`, req.VehicleID, req.VehicleID)
	
	if err != nil {
		log.Printf("Avg Update Error: %v\n", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}