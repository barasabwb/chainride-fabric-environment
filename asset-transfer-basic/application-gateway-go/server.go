package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
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
var BASE_URL = os.Getenv("BASE_URL")

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
	PricePerKm     int64  `json:"pricePerKm"`
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
	OwnerImage     string `json:"ownerImage"`
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
	PricePerKm     int64  `json:"PricePerKm"`
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
	http.HandleFunc("/api/admin/reject-asset", rejectAssetHandler)
	http.HandleFunc("/api/admin/pending-assets", getPendingAssetsHandler)
	http.HandleFunc("/api/admin/vehicle-reviews", getVehicleReviewsHandler)
	http.HandleFunc("/api/admin/toggle-vehicle", toggleAdminVehicleHandler)
	http.HandleFunc("/api/admin/fleet", getAdminFleetHandler)
	http.HandleFunc("/api/admin/stats", getAdminStatsHandler)
	http.HandleFunc("/api/admin/pending-users", getAdminPendingUsersHandler)
	http.HandleFunc("/api/admin/active-users", getAdminActiveUsersHandler)
	http.HandleFunc("/api/admin/verify-user", verifyUserHandler)
	http.HandleFunc("/api/admin/suspend-user", suspendUserHandler)
	http.HandleFunc("/api/eco", getEcoStatsHandler)
	http.HandleFunc("/api/profile", getProfileHandler)
	http.HandleFunc("/api/update-profile", updateProfileHandler)
	http.HandleFunc("/api/upload-avatar", uploadAvatarHandler)
	http.HandleFunc("/api/my-vehicles", getMyVehiclesHandler)
	http.HandleFunc("/api/host-stats", getHostStatsHandler)

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

	// Safely inject admin suspension mechanics
	db.Exec("ALTER TABLE vehicles ADD COLUMN admin_disabled BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE vehicles ADD COLUMN admin_reason TEXT")
	db.Exec("ALTER TABLE users ADD COLUMN admin_disabled BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE users ADD COLUMN admin_reason TEXT")
	db.Exec("ALTER TABLE vehicles ADD COLUMN is_approved BOOLEAN DEFAULT 0")

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
		mileage REAL,
		fuel_type TEXT,
		battery_level TEXT,
		latitude REAL,
		longitude REAL,
		co2_savings_rate INTEGER,
		price_per_km REAL, -- 🚨 ADDED THIS
		is_approved BOOLEAN DEFAULT 0,
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
		total_cost REAL, -- 🚨 ADDED THIS
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
	INSERT OR IGNORE INTO users (id, name, email, password_hash, is_verified) VALUES 
	('admin', 'Admin User', 'admin@chainride.com', 'adminpass', 1),
	('Anna', 'Anna Smith', 'anna@example.com', 'userpass1', 1),
	('Brian', 'Brian Jones', 'brian@example.com', 'userpass2', 1);
	`
	db.Exec(seedUsers)

	seedVehicles := `
	INSERT OR IGNORE INTO vehicles (id, owner_id, make, model, vehicle_type, car_class, transmission, seats, mileage, fuel_type, battery_level, latitude, longitude, co2_savings_rate, price_per_km, is_approved) VALUES 
	('CAR_001', 'Anna', 'Tesla', 'Model 3', 'Car', 'Sedan', 'Automatic', 5, 14500, 'Electric', '84%', 46.2521, 20.1410, 100, 12.0, 1),
	('SCOOTER_001', 'Brian', 'Xiaomi', 'M365', 'Scooter', 'Micro', 'N/A', 1, 850, 'Electric', '100%', 46.2530, 20.1414, 130, 3.5, 1);
	`
	db.Exec(seedVehicles)

	db.Exec("ALTER TABLE users ADD COLUMN profile_pic_path TEXT") // Non-destructive schema evolution for avatars

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
        var vehicleImagePath, ownerImagePath sql.NullString // 🚨 Added ownerImagePath
        var avgRating float64
        var adminDisabled bool

        // 🚨 UPDATED QUERY: Join with users table to get the host's profile picture and admin status
        var isApproved bool
        query := `
            SELECT v.latitude, v.longitude, v.image_path, v.avg_rating, u.profile_pic_path, v.admin_disabled, v.is_approved 
            FROM vehicles v
            JOIN users u ON v.owner_id = u.id
            WHERE v.id = ?
        `
        err := db.QueryRow(query, asset.ID).Scan(&lat, &lon, &vehicleImagePath, &avgRating, &ownerImagePath, &adminDisabled, &isApproved)
        
        if adminDisabled || !isApproved {
            continue // 🚨 Admin has suspended or not approved this vehicle, block from map
        }

        if err != nil {
            // Fallback if metadata isn't in SQLite yet
            lat, lon = float64(asset.BaseLatMicro)/1000000.0, float64(asset.BaseLonMicro)/1000000.0
            avgRating = 5.0
        }

        // Format URLs
        vImgUrl := ""
        if vehicleImagePath.Valid && vehicleImagePath.String != "" {
            vImgUrl = fmt.Sprintf("%s/%s", BASE_URL, vehicleImagePath.String)
        }

        oImgUrl := "" // 🚨 Format the Host's image URL
        if ownerImagePath.Valid && ownerImagePath.String != "" {
            oImgUrl = fmt.Sprintf("%s/%s", BASE_URL, ownerImagePath.String)
        }

        iconUrl := "https://img.icons8.com/color/48/000000/car-top-view.png"
        if asset.Type == "Scooter" {
            iconUrl = "https://img.icons8.com/color/48/000000/scooter.png"
        } else if asset.Type == "Bike" {
            iconUrl = "https://img.icons8.com/color/48/000000/bicycle.png"
        }

        features = append(features, GeoJSONFeature{
            Type: "Feature",
            Geometry: GeoJSONGeometry{Type: "Point", Coordinates: []float64{lon, lat}},
            Properties: GeoJSONProperties{
                Name:           asset.ID,
                Icon:           iconUrl,
                Owner:          asset.Owner,
                PricePerKm:     asset.PricePerKm,
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
                ImageUrl:       vImgUrl,
                AvgRating:      fmt.Sprintf("%.1f", avgRating),
                OwnerType:      "Private Host",
                OwnerImage:     oImgUrl, // 🚨 PASS THE HOST IMAGE HERE
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

	var isVerified bool
	err = db.QueryRow("SELECT is_verified FROM users WHERE id = ?", ownerID).Scan(&isVerified)
	if err != nil || !isVerified {
		http.Error(w, `{"error": "Verification Required: Please complete identity verification before listing a vehicle."}`, http.StatusForbidden)
		return
	}

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
		PricePerKm:     price,
		CO2SavingsRate: co2Rate,
		BaseLatMicro:   toMicrodegrees(lat),
		BaseLonMicro:   toMicrodegrees(lon),
	}

	// 🚨 DELAYED BLOCKCHAIN MINTING INTERCEPT:
	// We no longer call contract.SubmitTransaction("CreateAsset") here.
	// Instead, we just drop it into SQLite with is_approved = 0.

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
		INSERT INTO vehicles (id, owner_id, make, model, vehicle_type, car_class, transmission, seats, mileage, fuel_type, battery_level, latitude, longitude, co2_savings_rate, price_per_km, is_approved, image_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)
	`
	db.Exec(insertVehicleQuery, payload.ID, ownerID, payload.Make, payload.Model, payload.Type, payload.CarClass, payload.Transmission, payload.Seats, payload.Mileage, payload.FuelType, payload.BatteryLevel, lat, lon, payload.CO2SavingsRate, float64(payload.PricePerKm), imagePath)

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

    // 1. Check Verification
    var isVerified bool
    err := db.QueryRow("SELECT is_verified FROM users WHERE id = ?", effectiveUser).Scan(&isVerified)
    
    // Check if user exists or if they aren't verified
    if err != nil || !isVerified {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusForbidden)
        w.Write([]byte(`{"error": "Verification Required: Please verify your driver's license in Profile Settings to unlock vehicles."}`))
        return
    }

    // 2. Decode Request
    var req RentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VehicleID == "" {
        http.Error(w, `{"error": "Invalid request body or missing vehicleId"}`, 400)
        return
    }

    // 3. Submit Transaction (Use = instead of := here)
    _, err = contract.SubmitTransaction("RentAsset", req.VehicleID, effectiveUser)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(500)
        w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
        return
    }

    w.Header().Set("Content-Type", "application/json")
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

	// ==========================================
	// 🛣️ ORACLE: Simulate Driven Distance
	// ==========================================
	endTime := time.Now().Unix()
	durationMins := (endTime - asset.StartTime) / 60
	if durationMins < 1 { durationMins = 1 }

	multiplier := 0.3 + rand.Float64()*0.9 // 0.3 to 1.2 km/min (realistic urban)
	distanceKm := float64(durationMins) * multiplier
	if distanceKm < 0.5 { distanceKm = 0.5 } // Minimum 0.5 km

	distanceKm = math.Round(distanceKm*100) / 100
	distanceStr := strconv.FormatInt(int64(distanceKm), 10)

	log.Printf("[ORACLE] Trip %s: %d mins, multiplier=%.2f, distance=%.1f km\n", req.VehicleID, durationMins, multiplier, distanceKm)

	// Pass distance to Smart Contract as Oracle argument
	result, err := contract.SubmitTransaction("ReturnAsset", req.VehicleID, effectiveUser, latMicroStr, lonMicroStr, distanceStr)
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
		return
	}

	// 🚨 CATCH THE HASH ID FROM THE BLOCKCHAIN
	blockchainTripID := string(result)

	// ==========================================
	// 🌍 OFF-CHAIN: Dynamic CO2 Calculation
	// ==========================================
	// Read the vehicle's actual emission from SQLite
	var vehicleEmissionRate int64
	var fuelType string
	err = db.QueryRow("SELECT co2_savings_rate, fuel_type FROM vehicles WHERE id = ?", req.VehicleID).Scan(&vehicleEmissionRate, &fuelType)
	if err != nil {
		// Fallback: determine from fuel type on the blockchain asset
		fuelType = asset.FuelType
		switch fuelType {
		case "Electric": vehicleEmissionRate = 0
		case "Hybrid":   vehicleEmissionRate = 90
		case "Diesel":   vehicleEmissionRate = 130
		case "Petrol":   vehicleEmissionRate = 150
		default:         vehicleEmissionRate = 0
		}
	}

	// Baseline: avg single-occupancy car = 192g CO2/km
	baseline := int64(192)
	passengers := int64(1)
	co2Saved := int64(math.Max(0, float64((baseline*passengers - vehicleEmissionRate)) * distanceKm))

	log.Printf("[ECO] Vehicle %s (%s): emission=%dg/km, distance=%.1fkm, co2_saved=%dg\n", req.VehicleID, fuelType, vehicleEmissionRate, distanceKm, co2Saved)

	if asset.StartTime > 0 {
		// Mirror the exact Smart Contract math to cache it in SQLite
    totalCost := (distanceKm * float64(asset.PricePerKm)) + 1.0 // + 1.0 base unlock fee

    insertTripQuery := `
        INSERT INTO trips (id, driver_id, vehicle_id, start_time, end_time, distance_km, co2_saved, total_cost, status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'Completed')
    `
    db.Exec(insertTripQuery, blockchainTripID, effectiveUser, req.VehicleID, time.Unix(asset.StartTime, 0), time.Unix(endTime, 0), distanceKm, co2Saved, totalCost)
	updateEcoQuery := `UPDATE eco_stats SET total_trips = total_trips + 1, total_co2_saved = total_co2_saved + ?, total_distance = total_distance + ? WHERE user_id = ?`
	db.Exec(updateEcoQuery, co2Saved, distanceKm, effectiveUser)

	// 🚗 Increment Mileage on the Vehicle Object
	var currentMileageStr sql.NullString
	err = db.QueryRow("SELECT mileage FROM vehicles WHERE id = ?", req.VehicleID).Scan(&currentMileageStr)
	if err == nil && currentMileageStr.Valid {
		cleanStr := strings.ReplaceAll(currentMileageStr.String, " km", "")
		cleanStr = strings.ReplaceAll(cleanStr, ",", "")
		currentMileage, parseErr := strconv.ParseFloat(cleanStr, 64)
		if parseErr == nil {
			newMileage := currentMileage + distanceKm
			newMileageFormatted := fmt.Sprintf("%.1f", newMileage)
			db.Exec("UPDATE vehicles SET mileage = ? WHERE id = ?", newMileageFormatted, req.VehicleID)
		}
	}

	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"receipt":    blockchainTripID,
		"distanceKm": distanceKm,
		"co2Saved":   co2Saved,
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

// Removed previously mocked smart contract handlers

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
	var totalCO2 sql.NullFloat64

	err := db.QueryRow(`
		SELECT COUNT(*), SUM(co2_saved)
		FROM trips 
		WHERE driver_id = ? AND status = 'Completed'
	`, effectiveUser).Scan(&totalTrips, &totalCO2)
	
	co2 := 0.0
	if totalCO2.Valid {
		co2 = totalCO2.Float64
	}

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"totalTrips": 0, "totalCo2Saved": 0})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalTrips":    totalTrips,
		"totalCo2Saved": co2,
	})
}

func getHostStatsHandler(w http.ResponseWriter, r *http.Request) {
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
	var totalEarnings sql.NullFloat64
	var hostRating sql.NullFloat64

	// Total Trips and Earnings
	err := db.QueryRow(`
		SELECT COUNT(*), SUM(t.total_cost)
		FROM trips t
		JOIN vehicles v ON t.vehicle_id = v.id
		WHERE v.owner_id = ? AND t.status = 'Completed'
	`, effectiveUser).Scan(&totalTrips, &totalEarnings)
	if err != nil {
		log.Printf("DB Query Error for stats: %v\n", err)
	}

	// Host Rating
	err = db.QueryRow(`
		SELECT AVG(rating)
		FROM user_ratings
		WHERE rated_user_id = ?
	`, effectiveUser).Scan(&hostRating)
	if err != nil {
		log.Printf("DB Query Error for rating: %v\n", err)
	}

	earnings := 0.0
	if totalEarnings.Valid {
		earnings = totalEarnings.Float64
	}

	rating := 5.0
	if hostRating.Valid {
		rating = math.Round(hostRating.Float64*10) / 10
	} else {
		// Fallback to reputation_score from users table
		var repScore sql.NullFloat64
		db.QueryRow("SELECT reputation_score FROM users WHERE id = ?", effectiveUser).Scan(&repScore)
		if repScore.Valid && repScore.Float64 > 0 {
			rating = math.Round(repScore.Float64*10) / 10
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalTrips":    totalTrips,
		"totalEarnings": earnings,
		"hostRating":    rating,
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
		effectiveUser = r.URL.Query().Get("id")
	}

	// 🚨 SELECT now includes t.distance_km and t.total_cost
	query := `
		SELECT t.id, v.make, v.model, v.id, v.owner_id, v.vehicle_type, v.image_path,
		       t.start_time, t.end_time, t.distance_km, t.co2_saved, t.total_cost, t.status,
		       ur.rating, ur.comment,
		       vr.rating, vr.comment
		FROM trips t
		JOIN vehicles v ON t.vehicle_id = v.id
		LEFT JOIN user_ratings ur ON ur.trip_id = t.id AND ur.rater_id = ?
		LEFT JOIN vehicle_ratings vr ON vr.trip_id = t.id AND vr.user_id = ?
		WHERE t.driver_id = ?
		ORDER BY t.end_time DESC
	`
	rows, err := db.Query(query, effectiveUser, effectiveUser, effectiveUser)
	if err != nil {
		log.Printf("DB Query Error: %v\n", err)
		http.Error(w, `{"error": "Failed to fetch trips"}`, 500)
		return
	}
	defer rows.Close()

	type ReactTrip struct {
		TripID        string  `json:"TripID"`
		AssetID       string  `json:"AssetID"`
		Owner         string  `json:"Owner"`
		EndTime       int64   `json:"EndTime"`
		DurationMins  int64   `json:"DurationMins"`
		DistanceKm    float64 `json:"DistanceKm"` // 🚨 Added distance to pass to UI
		TotalCost     float64 `json:"TotalCost"`
		CO2Saved      float64 `json:"CO2Saved"`
		Status        string  `json:"Status"`
		RenterRated   bool    `json:"RenterRated"`
		VehicleRated  bool    `json:"VehicleRated"`
		Rating        int     `json:"Rating"`        
		VehicleRating int     `json:"VehicleRating"` 
		ImagePath     string  `json:"ImagePath"`
		Comment       string  `json:"Comment"`
	}

	var trips []ReactTrip
	for rows.Next() {
		var id, vMake, vModel, vId, vOwner, vType, status string
		var startTime, endTime time.Time
		var distanceKm, co2Saved, totalCost sql.NullFloat64 // 🚨 Catch the real math
		var imgPath, uComment, vComment sql.NullString
		var uRating, vRating sql.NullInt32

		err := rows.Scan(&id, &vMake, &vModel, &vId, &vOwner, &vType, &imgPath, &startTime, &endTime, &distanceKm, &co2Saved, &totalCost, &status, &uRating, &uComment, &vRating, &vComment)
		if err != nil {
			log.Printf("Row Scan Error: %v\n", err)
			continue
		}

		durationMins := int64(endTime.Sub(startTime).Minutes())
		if durationMins < 1 { durationMins = 1 }

		imgUrl := ""
		if imgPath.Valid && imgPath.String != "" {
			imgUrl = fmt.Sprintf("%s/%s", BASE_URL, imgPath.String)
		}

		trips = append(trips, ReactTrip{
			TripID:        id,
			AssetID:       fmt.Sprintf("%s %s (%s)", vMake, vModel, vId),
			Owner:         vOwner,
			EndTime:       endTime.Unix(),
			DurationMins:  durationMins,
			DistanceKm:    distanceKm.Float64,
			TotalCost:     totalCost.Float64, // 🚨 Pulling exact cost from DB, no more hardcoded math!
			CO2Saved:      co2Saved.Float64,
			Status:        status,
			RenterRated:   uRating.Valid,
			VehicleRated:  vRating.Valid,
			Rating:        int(uRating.Int32),
			VehicleRating: int(vRating.Int32),
			ImagePath:     imgUrl,
			Comment:       uComment.String,
		})
	}

	if trips == nil { trips = []ReactTrip{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trips)
}

// 🌟 SUBMIT TRIP RATING TO THE BLOCKCHAIN
func rateTripHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	user := getEffectiveUser(r)
	var req RateRequest
	json.NewDecoder(r.Body).Decode(&req)

	// 1. Send to Blockchain
	starsStr := fmt.Sprintf("%.1f", req.Stars)
	_, err := contract.SubmitTransaction("RateTrip", req.TripID, user, starsStr)
	if err != nil {
		log.Printf("Blockchain Error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// 2. Lookup the Host's ID for the DB Cache
	var ownerID string
	db.QueryRow("SELECT owner_id FROM trips t JOIN vehicles v ON t.vehicle_id = v.id WHERE t.id = ?", req.TripID).Scan(&ownerID)

	// 3. Save locally with strict error checking
	_, err = db.Exec("INSERT INTO user_ratings (rater_id, rated_user_id, trip_id, rating, comment) VALUES (?, ?, ?, ?, ?)", user, ownerID, req.TripID, req.Stars, req.Comment)
	if err != nil {
		log.Printf("DB Error Saving Host Rating: %v", err)
		http.Error(w, "DB Error", 500)
		return
	}
	
	w.Write([]byte(`{"success": true}`))
}

// 🚗 SUBMIT VEHICLE RATING (100% Off-Chain SQLite)
type RateVehicleRequest struct {
	TripID    string  `json:"tripId"`
	VehicleID string  `json:"vehicleId"`
	Stars     float64 `json:"stars"`
	Comment   string  `json:"comment"`
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
	_, err := db.Exec("INSERT INTO vehicle_ratings (user_id, vehicle_id, trip_id, rating, comment) VALUES (?, ?, ?, ?, ?)", effectiveUser, req.VehicleID, req.TripID, req.Stars, req.Comment)
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

// ==========================================
// 👤 NEW PROFILE APIS 
// ==========================================

type ProfileResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	PhoneNumber    string `json:"phoneNumber"`
	IsVerified     bool   `json:"isVerified"`
	LicensePicPath string `json:"licensePicPath"`
	ProfilePicPath string `json:"profilePicPath"`
}

func getProfileHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var p ProfileResponse
	var picPath sql.NullString
	var phone sql.NullString
	var profilePic sql.NullString

	err := db.QueryRow("SELECT id, name, email, phone_number, is_verified, license_pic_path, profile_pic_path FROM users WHERE id = ?", effectiveUser).
		Scan(&p.ID, &p.Name, &p.Email, &phone, &p.IsVerified, &picPath, &profilePic)
		
	if err != nil {
		http.Error(w, `{"error": "Profile not found"}`, 404)
		return
	}
	
	p.PhoneNumber = phone.String
	p.LicensePicPath = picPath.String
	p.ProfilePicPath = profilePic.String

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func updateProfileHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, 400)
		return
	}

	_, err := db.Exec("UPDATE users SET name = ?, email = ?, phone_number = ? WHERE id = ?", req.Name, req.Email, req.Phone, effectiveUser)
	if err != nil {
		http.Error(w, `{"error": "Failed to update profile"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func uploadAvatarHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error": "Failed to parse form data"}`, http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("avatarImage")
	if err != nil {
		http.Error(w, `{"error": "Failed to retrieve image"}`, 400)
		return
	}
	defer file.Close()

	os.MkdirAll("uploads", os.ModePerm)
	filename := fmt.Sprintf("avatar_%s_%d%s", effectiveUser, time.Now().Unix(), filepath.Ext(handler.Filename))
	imagePath := filepath.Join("uploads", filename)

	dst, err := os.Create(imagePath)
	if err != nil {
		http.Error(w, `{"error": "Failed to save file"}`, 500)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	_, err = db.Exec("UPDATE users SET profile_pic_path = ? WHERE id = ?", imagePath, effectiveUser)
	if err != nil {
		http.Error(w, `{"error": "Failed to update profile"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "profilePicPath": imagePath})
}


// ==========================================
// 🚗 MY VEHICLES API (UPDATED)
// ==========================================
func getMyVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser == "" {
		http.Error(w, `{"error": "Unauthorized"}`, 401)
		return
	}

	// 🚨 SELECT now includes admin override and pending approval
	rows, err := db.Query(`
		SELECT id, make, model, vehicle_type, car_class, transmission, seats, mileage, battery_level, co2_savings_rate, price_per_km, image_path, admin_disabled, admin_reason, is_approved 
		FROM vehicles WHERE owner_id = ? ORDER BY id DESC`, effectiveUser)

	if err != nil {
		log.Printf("DB Query Error: %v\n", err)
		http.Error(w, `{"error": "Failed to query vehicles"}`, 500)
		return
	}
	defer rows.Close()

	var vehicles []map[string]interface{}
	for rows.Next() {
		var id, vmake, model, vType, vClass, trans, mileage, battery, imagePath, adminReason sql.NullString
		var seats, co2 sql.NullInt64
		var pricePerKm sql.NullFloat64 // 🚨 Catch the real price
		var adminDisabled, isApproved bool

		if err := rows.Scan(&id, &vmake, &model, &vType, &vClass, &trans, &seats, &mileage, &battery, &co2, &pricePerKm, &imagePath, &adminDisabled, &adminReason, &isApproved); err != nil {
			log.Printf("Row Scan Error: %v\n", err)
			continue
		}

		imgUrl := "https://images.unsplash.com/photo-1503376712351-1f2ecf63f0bb?auto=format&fit=crop&w=800&q=80"
		if imagePath.Valid && imagePath.String != "" {
			imgUrl = fmt.Sprintf("%s/%s", BASE_URL, imagePath.String)
		}

		// 🚨 Query exact total_cost directly from the trips table!
		var totalTrips int
		var totalEarnings sql.NullFloat64

		db.QueryRow(`
			SELECT COUNT(*), SUM(total_cost) 
			FROM trips 
			WHERE vehicle_id = ? AND status = 'Completed'
		`, id.String).Scan(&totalTrips, &totalEarnings)

		earnings := 0.0
		if totalEarnings.Valid {
			earnings = totalEarnings.Float64
		}

		// Use the real price the user entered, fallback to 3.0
		price := 3.0
		if pricePerKm.Valid {
			price = pricePerKm.Float64
		}

		chaincodeStatus := "Available"
		assetBytes, err := contract.EvaluateTransaction("ReadAsset", id.String)
		if err == nil {
			var asset ChaincodeAsset
			json.Unmarshal(assetBytes, &asset)
			if asset.Status == "AVAILABLE" { chaincodeStatus = "Available" }
			if asset.Status == "IN_USE" { chaincodeStatus = "In Use" }
			if asset.Status == "UNAVAILABLE" { chaincodeStatus = "Unavailable" }
		}

		if !isApproved {
			chaincodeStatus = "Pending Approval"
		} else if adminDisabled {
			chaincodeStatus = "Disabled by Admin"
		}

		vehicles = append(vehicles, map[string]interface{}{
			"id":           id.String,
			"type":         vType.String,
			"title":        vmake.String + " " + model.String,
			"class":        vClass.String,
			"location":     "Stored Location",
			"price":        price,
			"battery":      battery.String,
			"trips":        totalTrips,
			"earnings":     earnings,
			"status":       chaincodeStatus,
			"adminReason":  adminReason.String,
			"transmission": trans.String,
			"seats":        seats.Int64,
			"mileage":      mileage.String,
			"co2Rate":      co2.Int64,
			"imageUrl":     imgUrl,
		})
	}

	if vehicles == nil { vehicles = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vehicles)
}

// ==========================================
// 🛡️ ADMIN OVERRIDES (Suspensions)
// ==========================================

func toggleAdminVehicleHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden: Requires Admin privileges"}`, 403)
		return
	}

	var req struct {
		VehicleID string `json:"vehicleId"`
		Disabled  bool   `json:"disabled"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid payload format"}`, 400)
		return
	}

	// Update the vehicle's off-chain suspension parameters
	_, err := db.Exec("UPDATE vehicles SET admin_disabled = ?, admin_reason = ? WHERE id = ?", req.Disabled, req.Reason, req.VehicleID)
	if err != nil {
		log.Printf("DB Error suspending vehicle: %v", err)
		http.Error(w, `{"error": "Failed to update asset suspension state"}`, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"disabled": req.Disabled,
		"reason":   req.Reason,
	})
}

func getAdminFleetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden: Requires Admin privileges"}`, 403)
		return
	}

	rows, err := db.Query(`
		SELECT id, owner_id, make, model, vehicle_type, car_class, transmission, seats, mileage, battery_level, co2_savings_rate, price_per_km, image_path, admin_disabled, admin_reason, avg_rating, latitude, longitude 
		FROM vehicles WHERE is_approved = 1 ORDER BY id DESC`)

	if err != nil {
		log.Printf("DB Query Error: %v\n", err)
		http.Error(w, `{"error": "Failed to query fleet"}`, 500)
		return
	}
	defer rows.Close()

	var fleet []map[string]interface{}
	for rows.Next() {
		var id, owner_id, vmake, model, vType, vClass, trans, mileage, battery, imagePath, adminReason sql.NullString
		var seats, co2 sql.NullInt64
		var pricePerKm, avgRating, lat, lon sql.NullFloat64
		var adminDisabled bool

		if err := rows.Scan(&id, &owner_id, &vmake, &model, &vType, &vClass, &trans, &seats, &mileage, &battery, &co2, &pricePerKm, &imagePath, &adminDisabled, &adminReason, &avgRating, &lat, &lon); err != nil {
			log.Printf("Row Scan Error: %v\n", err)
			continue
		}

		status := "AVAILABLE"
		if adminDisabled {
			status = "DELISTED"
		}

		var totalTrips int
		db.QueryRow(`SELECT COUNT(*) FROM trips WHERE vehicle_id = ? AND status = 'Completed'`, id.String).Scan(&totalTrips)

		fleet = append(fleet, map[string]interface{}{
			"id":           id.String,
			"owner":        owner_id.String,
			"type":         vType.String,
			"make":         vmake.String,
			"model":        model.String,
			"carClass":     vClass.String,
			"transmission": trans.String,
			"seats":        seats.Int64,
			"mileage":      mileage.String,
			"fuelType":     "Electric",
			"powerLevel":   battery.String,
			"lat":          lat.Float64,
			"lon":          lon.Float64,
			"co2Rate":      co2.Int64,
			"price":        pricePerKm.Float64,
			"rating":       avgRating.Float64,
			"status":       status,
			"adminReason":  adminReason.String,
			"trips":        totalTrips,
		})
	}

	if fleet == nil { fleet = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fleet)
}

// ==========================================
// 📈 ADMIN DASHBOARD STATS
// ==========================================
func getAdminStatsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	effectiveUser := getEffectiveUser(r)
	if effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden: Requires Admin privileges"}`, 403)
		return
	}

	var totalUsers, activeVehicles, totalTrips int
	var co2Saved, grossRevenue sql.NullFloat64

	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	db.QueryRow("SELECT COUNT(*) FROM vehicles").Scan(&activeVehicles)
	db.QueryRow("SELECT COUNT(*) FROM trips").Scan(&totalTrips)
	db.QueryRow("SELECT SUM(co2_saved), SUM(total_cost) FROM trips").Scan(&co2Saved, &grossRevenue)

	revenue := 0.0
	if grossRevenue.Valid {
		revenue = grossRevenue.Float64
	}
	
	co2 := 0.0
	if co2Saved.Valid {
		co2 = co2Saved.Float64
	}

	// 1 CRT fee per trip
	platformFees := float64(totalTrips) * 1.0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"totalUsers": totalUsers,
		"activeVehicles": activeVehicles,
		"totalTrips": totalTrips,
		"co2Saved": co2,
		"grossRevenue": revenue,
		"platformFees": platformFees,
	})
}

// ==========================================
// 🧑‍💼 ADMIN USER MANAGEMENT
// ==========================================

func getAdminPendingUsersHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden: Requires Admin privileges"}`, 403)
		return
	}

	rows, err := db.Query(`SELECT id, name, email, dob, license_number, license_pic_path FROM users WHERE is_verified = 0 AND license_pic_path IS NOT NULL`)
	if err != nil {
		http.Error(w, `{"error": "Failed to query pending users"}`, 500)
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, name, email, dob, licenseNum, licensePic sql.NullString
		if err := rows.Scan(&id, &name, &email, &dob, &licenseNum, &licensePic); err != nil { continue }
		imgUrl := ""
		if licensePic.Valid && licensePic.String != "" {
			imgUrl = fmt.Sprintf("%s/%s", BASE_URL, licensePic.String)
		}
		users = append(users, map[string]interface{}{
			"id": id.String, "name": name.String, "email": email.String, "dob": dob.String, 
			"license_number": licenseNum.String, "license_pic_path": imgUrl,
		})
	}
	if users == nil { users = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func getAdminActiveUsersHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden: Requires Admin privileges"}`, 403)
		return
	}

	rows, err := db.Query(`
		SELECT id, email, reputation_score, admin_disabled, 
		       (SELECT IFNULL(SUM(co2_saved), 0) FROM trips WHERE driver_id = users.id) as loyalty
		FROM users 
		WHERE is_verified = 1`)
	if err != nil {
		http.Error(w, `{"error": "Failed to query active users"}`, 500)
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, email sql.NullString
		var repScore, loyalty sql.NullFloat64
		var adminDisabled bool
		if err := rows.Scan(&id, &email, &repScore, &adminDisabled, &loyalty); err != nil { continue }

		role := "USER"
		if id.String == "admin" || id.String == "appUser" { role = "ADMIN" }

		status := "ACTIVE"
		if adminDisabled { status = "SUSPENDED" }

		users = append(users, map[string]interface{}{
			"id": id.String, "email": email.String, "trustScore": repScore.Float64,
			"status": status, "loyalty": int(loyalty.Float64), "role": role, "balance": 0,
		})
	}
	if users == nil { users = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func verifyUserHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden"}`, 403)
		return
	}

	var req struct {
		UserID   string `json:"userId"`
		Approved bool   `json:"approved"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, 400)
		return
	}

	if req.Approved {
		db.Exec("UPDATE users SET is_verified = 1 WHERE id = ?", req.UserID)
	} else {
		db.Exec("UPDATE users SET is_verified = 0, license_pic_path = NULL WHERE id = ?", req.UserID)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func suspendUserHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden"}`, 403)
		return
	}

	var req struct {
		UserID   string `json:"userId"`
		Disabled bool   `json:"disabled"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, 400)
		return
	}

	db.Exec("UPDATE users SET admin_disabled = ?, admin_reason = ? WHERE id = ?", req.Disabled, req.Reason, req.UserID)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ==========================================
// 🚗 ADMIN VEHICLE APPROVAL & REVIEWS 
// ==========================================

func getPendingAssetsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden: Requires Admin privileges"}`, 403)
		return
	}

	rows, err := db.Query(`SELECT id, owner_id, vehicle_type, make, model, fuel_type, price_per_km, co2_savings_rate, image_path FROM vehicles WHERE is_approved = 0`)
	if err != nil {
		http.Error(w, `{"error": "Failed to query pending vehicles"}`, 500)
		return
	}
	defer rows.Close()

	var vehicles []map[string]interface{}
	for rows.Next() {
		var id, owner_id, vType, make, model, fuelType, imagePath sql.NullString
		var pricePerKm float64
		var co2Rate int
		
		if err := rows.Scan(&id, &owner_id, &vType, &make, &model, &fuelType, &pricePerKm, &co2Rate, &imagePath); err != nil { continue }
		imgUrl := ""
		if imagePath.Valid && imagePath.String != "" { imgUrl = fmt.Sprintf("%s/%s", BASE_URL, imagePath.String) }
		
		vehicles = append(vehicles, map[string]interface{}{
			"ID": id.String, "Owner": owner_id.String, "Type": vType.String, "Make": make.String, "Model": model.String,
			"FuelType": fuelType.String, "PricePerMinute": pricePerKm, "CO2SavingsRate": co2Rate, "imageUrl": imgUrl,
		})
	}
	if vehicles == nil { vehicles = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vehicles)
}

func approveAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden"}`, 403)
		return
	}

	var req struct { VehicleID string `json:"vehicleId"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, 400)
		return
	}

	var payload ChaincodeAsset
	var id, owner_id, vmake, model, vType, carClass, transmission, fuelType, batteryLevel sql.NullString
	var lat, lon, pricePerKm sql.NullFloat64
	var seats, co2Rate sql.NullInt64
	var mileage sql.NullFloat64 
	
	err := db.QueryRow(`SELECT id, owner_id, make, model, vehicle_type, car_class, transmission, seats, mileage, fuel_type, battery_level, latitude, longitude, co2_savings_rate, price_per_km FROM vehicles WHERE id = ?`, req.VehicleID).Scan(
		&id, &owner_id, &vmake, &model, &vType, &carClass, &transmission, &seats, &mileage, &fuelType, &batteryLevel, &lat, &lon, &co2Rate, &pricePerKm,
	)
	if err != nil {
		http.Error(w, `{"error": "Asset not found in SQLite"}`, 404)
		return
	}
	payload.ID = id.String
	payload.Owner = owner_id.String
	payload.Make = vmake.String
	payload.Model = model.String
	payload.Type = vType.String
	payload.CarClass = carClass.String
	payload.Transmission = transmission.String
	payload.Seats = int(seats.Int64)
	payload.Mileage = fmt.Sprintf("%.2f", mileage.Float64)
	payload.FuelType = fuelType.String
	payload.BatteryLevel = batteryLevel.String
	payload.CO2SavingsRate = co2Rate.Int64
	payload.PricePerKm = int64(pricePerKm.Float64)
	payload.BaseLatMicro = toMicrodegrees(lat.Float64)
	payload.BaseLonMicro = toMicrodegrees(lon.Float64)

	payloadBytes, _ := json.Marshal(payload)
	_, mintErr := contract.SubmitTransaction("CreateAsset", string(payloadBytes))
	if mintErr != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to mint asset on blockchain: %v"}`, mintErr), 500)
		return
	}

	db.Exec("UPDATE vehicles SET is_approved = 1 WHERE id = ?", req.VehicleID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func rejectAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }
	if effectiveUser := getEffectiveUser(r); effectiveUser != "admin" && effectiveUser != "appUser" {
		http.Error(w, `{"error": "Forbidden"}`, 403)
		return
	}

	var req struct { VehicleID string `json:"vehicleId"`; Reason string `json:"reason"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid payload"}`, 400)
		return
	}

	db.Exec("DELETE FROM vehicles WHERE id = ?", req.VehicleID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func getVehicleReviewsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	vehicleId := r.URL.Query().Get("vehicleId")
	rows, err := db.Query(`SELECT r.user_id, r.rating, r.comment, r.created_at FROM vehicle_ratings r WHERE r.vehicle_id = ? ORDER BY r.created_at DESC`, vehicleId)
	if err != nil {
		http.Error(w, `{"error": "Failed to fetch reviews"}`, 500)
		return
	}
	defer rows.Close()

	var reviews []map[string]interface{}
	for rows.Next() {
		var reviewer, comment, date sql.NullString
		var rating float64
		if err := rows.Scan(&reviewer, &rating, &comment, &date); err != nil { continue }
		
		idCount := len(reviews) + 1
		reviews = append(reviews, map[string]interface{}{
			"id": idCount, "reviewer": reviewer.String, "rating": rating, "comment": comment.String, "date": date.String,
		})
	}
	if reviews == nil { reviews = []map[string]interface{}{} }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reviews)
}
