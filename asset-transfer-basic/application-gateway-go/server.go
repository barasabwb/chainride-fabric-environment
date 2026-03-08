package main

import (
        "database/sql"
        "encoding/json"
        "fmt"
        "log"
        "net/http"
        "os"
        "io"
        "path/filepath"
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

// Secure Secret Management
func getJWTSecret() []byte {
        secret := os.Getenv("JWT_SECRET")
        if secret == "" {
                log.Println("WARNING: JWT_SECRET env var not set. Using fallback development key.")
                return []byte("super_secret_thesis_key_2026")
        }
        return []byte(secret)
}

// Helper function to extract and verify the JWT from the HTTP request
func verifyJWT(r *http.Request) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	// Extract the token from the "Bearer <token>" format
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, fmt.Errorf("invalid token format")
	}

	claims := &Claims{}
	
	// Parse and validate the token using your existing jwtKey
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
        PricePerMinute int    `json:"pricePerMinute"`
        Type           string `json:"type"`
        CO2SavingsRate int    `json:"co2Rate"`
}

type ChaincodeAsset struct {
        ID             string `json:"ID"`
        Type           string `json:"Type"`
        FuelType       string `json:"FuelType"`
        Owner          string `json:"Owner"`
        Status         string `json:"Status"`
        PricePerMinute int    `json:"PricePerMinute"`
        StartTime      int64  `json:"StartTime"`
        CurrentRenter  string `json:"CurrentRenter"`
        CO2SavingsRate int    `json:"CO2SavingsRate"`
}

type LoginRequest struct {
        UserID   string `json:"userId"`
        Password string `json:"password"`
}
type RentRequest struct {
        VehicleID string `json:"vehicleId"`
}
type ReturnRequest struct {
        VehicleID string `json:"vehicleId"`
}
type TopUpRequest struct {
        Amount int `json:"amount"`
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

	// Enable foreign key enforcement
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		log.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// 1. USERS (Merged with our KYC requirements)
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
	if _, err := db.Exec(createUsersTable); err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}

	// 2. VEHICLES (Using TEXT for Web3 ID Mapping)
	createVehiclesTable := `
	CREATE TABLE IF NOT EXISTS vehicles (
		id TEXT PRIMARY KEY,
		owner_id TEXT NOT NULL,
		make TEXT,
		model TEXT,
		year INTEGER,
		vehicle_type TEXT, 
		seats INTEGER,
		fuel_capacity REAL,
		fuel_level REAL,
		battery_capacity REAL,
		battery_level REAL,
		consumption_per_100km REAL,
		avg_rating REAL DEFAULT 5.0,
		latitude REAL,
		longitude REAL,
		available BOOLEAN DEFAULT 1,
		co2_savings_rate INTEGER,
		FOREIGN KEY(owner_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createVehiclesTable); err != nil {
		log.Fatalf("Error creating vehicles table: %v", err)
	}

	// 3. TRIPS
	createTripsTable := `
	CREATE TABLE IF NOT EXISTS trips (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		driver_id TEXT,
		vehicle_id TEXT,
		start_lat REAL,
		start_lon REAL,
		end_lat REAL,
		end_lon REAL,
		distance_km REAL,
		fuel_used REAL,
		battery_used REAL,
		co2_emitted REAL,
		co2_saved REAL,
		start_time DATETIME,
		end_time DATETIME,
		status TEXT,
		FOREIGN KEY(driver_id) REFERENCES users(id),
		FOREIGN KEY(vehicle_id) REFERENCES vehicles(id)
	);`
	if _, err := db.Exec(createTripsTable); err != nil {
		log.Fatalf("Error creating trips table: %v", err)
	}

	// 4. USER RATINGS
	createUserRatings := `
	CREATE TABLE IF NOT EXISTS user_ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rater_id TEXT,
		rated_user_id TEXT,
		trip_id INTEGER,
		rating INTEGER CHECK(rating BETWEEN 1 AND 5),
		comment TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(rater_id) REFERENCES users(id),
		FOREIGN KEY(rated_user_id) REFERENCES users(id),
		FOREIGN KEY(trip_id) REFERENCES trips(id)
	);`
	if _, err := db.Exec(createUserRatings); err != nil {
		log.Fatalf("Error creating user_ratings: %v", err)
	}

	// 5. VEHICLE RATINGS
	createVehicleRatings := `
	CREATE TABLE IF NOT EXISTS vehicle_ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		vehicle_id TEXT,
		trip_id INTEGER,
		rating INTEGER CHECK(rating BETWEEN 1 AND 5),
		comment TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id),
		FOREIGN KEY(vehicle_id) REFERENCES vehicles(id),
		FOREIGN KEY(trip_id) REFERENCES trips(id)
	);`
	if _, err := db.Exec(createVehicleRatings); err != nil {
		log.Fatalf("Error creating vehicle_ratings: %v", err)
	}

	// 6. ECO STATISTICS
	createEcoStats := `
	CREATE TABLE IF NOT EXISTS eco_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		total_trips INTEGER DEFAULT 0,
		total_distance REAL DEFAULT 0,
		total_co2_saved REAL DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createEcoStats); err != nil {
		log.Fatalf("Error creating eco_stats: %v", err)
	}

	// ---------------- SEED DATA ----------------
	// Important: Users must be inserted BEFORE vehicles due to Foreign Keys!
	seedUsers := `
	INSERT OR IGNORE INTO users (id, name, email, password_hash) VALUES 
	('admin', 'Admin User', 'admin@chainride.com', 'adminpass'),
	('Anna', 'Anna Smith', 'anna@example.com', 'userpass1'),
	('Brian', 'Brian Jones', 'brian@example.com', 'userpass2');
	`
	if _, err := db.Exec(seedUsers); err != nil {
		log.Fatalf("Error seeding users: %v", err)
	}

	seedVehicles := `
	INSERT OR IGNORE INTO vehicles (id, owner_id, make, model, vehicle_type, latitude, longitude, co2_savings_rate) VALUES 
	('CAR_001', 'Anna', 'Tesla', 'Model 3', 'electric', 46.2512, 20.1458, 100),
	('SCOOTER_001', 'Brian', 'Xiaomi', 'M365', 'electric', 46.2530, 20.1410, 130),
	('BIKE_001', 'admin', 'Trek', 'FX1', 'human', 46.2550, 20.1500, 150);
	`
	if _, err := db.Exec(seedVehicles); err != nil {
		log.Fatalf("Error seeding vehicles: %v", err)
	}

	log.Println("✅ SQLite Database initialized successfully with Web3-mapped relational schema.")
}

// AUTHENTICATION (JWT)


func loginHandler(w http.ResponseWriter, r *http.Request) {
        enableCors(&w)
        if r.Method == "OPTIONS" { return }
        if r.Method != http.MethodPost {
                http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        var req LoginRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
                http.Error(w, "Invalid request body", 400)
                return
        }

        // Admin Password Check
        if req.UserID == "appUser" {
                // testing password
				
                adminPin := os.Getenv("ADMIN_PIN")
		// fmt.Printf(" DEBUG LOGIN - Received Pass: '%s' | Expected Pass: '%s'\n", req.Password, adminPin)
                if adminPin == "" { adminPin = "admin123" } 

                if req.Password != adminPin {
                        http.Error(w, `{"error": "Forbidden: Invalid Admin Credentials"}`, 403)
                        return
                }
        } else {
                // Ensure Users Exist
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
        if authHeader == "" { return "" }

        parts := strings.Split(authHeader, "Bearer ")
        if len(parts) != 2 { return "" }
        tokenString := strings.TrimSpace(parts[1])

        claims := &Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
                //  Enforce HS256 algorithm
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return jwtKey, nil
        })

        if err != nil || !token.Valid { return "" }
        return claims.UserID
}


// API HANDLERS

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
                if asset.Status != "AVAILABLE" { continue }

                var lat, lon float64
                err := db.QueryRow("SELECT latitude, longitude FROM vehicles WHERE id = ?", asset.ID).Scan(&lat, &lon)
                if err != nil {
                        log.Printf("Warning: Missing GPS data for %s, applying Szeged city center fallback", asset.ID)
                        lat, lon = 46.2530, 20.1414 
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
                                PricePerMinute: asset.PricePerMinute,
                                Type:           asset.Type,
                                CO2SavingsRate: asset.CO2SavingsRate,
                        },
                })
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(GeoJSONFeatureCollection{Type: "FeatureCollection", Features: features})
}

func rentAssetHandler(w http.ResponseWriter, r *http.Request) {
        enableCors(&w)
        if r.Method == "OPTIONS" { return }
        if r.Method != http.MethodPost {
                http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        effectiveUser := getEffectiveUser(r)
        if effectiveUser == "" {
                http.Error(w, `{"error": "Unauthorized. Invalid or missing JWT."}`, 401)
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
        if r.Method == "OPTIONS" { return }
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

        // 1. Execute the Blockchain Transaction FIRST (The source of truth for payment & state)
        result, err := contract.SubmitTransaction("ReturnAsset", req.VehicleID, effectiveUser)
        if err != nil {
                w.WriteHeader(500)
                w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
                return
        }

        // 2. 📊 HYBRID DATA LOGGING: If blockchain succeeds, save the analytics to SQLite
        
        // A. Fetch the vehicle's CO2 rating from SQLite
        var co2Rate float64
        err = db.QueryRow("SELECT co2_savings_rate FROM vehicles WHERE id = ?", req.VehicleID).Scan(&co2Rate)
        if err != nil {
                log.Printf("Warning: Could not find CO2 rate for vehicle %s. Defaulting to 100g/min.", req.VehicleID)
                co2Rate = 100.0 // Fallback
        }

        // For this prototype, we will log a flat estimate (e.g., assuming an average 15-minute ride for demo purposes)
        // In a production app, you would parse the exact duration from the smart contract 'result' receipt.
        estimatedDurationMins := 15.0 
        totalCO2Saved := co2Rate * estimatedDurationMins

        // B. Log the completed trip into the 'trips' table
        insertTripQuery := `
                INSERT INTO trips (driver_id, vehicle_id, co2_saved, end_time, status) 
                VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'COMPLETED')
        `
        _, err = db.Exec(insertTripQuery, effectiveUser, req.VehicleID, totalCO2Saved)
        if err != nil {
                log.Printf("SQLite Error logging trip: %v", err)
        }

        // C. Update the user's lifetime Eco-Stats (With Smart Upsert!)
        updateEcoQuery := `
                UPDATE eco_stats 
                SET total_trips = total_trips + 1, 
                    total_co2_saved = total_co2_saved + ? 
                WHERE user_id = ?
        `
        res, err := db.Exec(updateEcoQuery, totalCO2Saved, effectiveUser)
        if err == nil {
                // Check if the UPDATE actually found a row to change
                rowsAffected, _ := res.RowsAffected()
                if rowsAffected == 0 {
                        // The user didn't have an eco_stats profile yet! Let's create one.
                        insertNewEcoQuery := `INSERT INTO eco_stats (user_id, total_trips, total_co2_saved) VALUES (?, 1, ?)`
                        db.Exec(insertNewEcoQuery, effectiveUser, totalCO2Saved)
                }
        } else {
                log.Printf("SQLite Error updating eco stats: %v", err)
        }

        // 3. Return the successful receipt to the React frontend
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
                "success": true, 
                "receipt": string(result),
        })
}

func getWalletHandler(w http.ResponseWriter, r *http.Request) {
        enableCors(&w)
        if r.Method == "OPTIONS" { return }

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
        if r.Method == "OPTIONS" { return }
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

        //  Demo Faucet Limit (Max $5000 per request)
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
        if r.Method == "OPTIONS" { return }

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

	// 1. Parse the multipart form (Max upload size: 10MB)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, `{"error": "Failed to parse form data"}`, http.StatusBadRequest)
		return
	}

	// 2. Extract Text Fields from React FormData
	userID := r.FormValue("username") // This acts as the ID for both Web3 and SQLite
	password := r.FormValue("password") // In a real production app, you would hash this!
	email := r.FormValue("email")
	fullName := r.FormValue("fullName")
	phoneNumber := r.FormValue("phoneNumber")
	dob := r.FormValue("dob")
	licenseNum := r.FormValue("licenseNumber")

	if userID == "" || password == "" || email == "" {
		http.Error(w, `{"error": "Missing critical fields (username, password, email)"}`, http.StatusBadRequest)
		return
	}

	// 3. Handle the License Image Upload
	file, handler, err := r.FormFile("licenseImage")
	var imagePath string

	if err == nil {
		defer file.Close()
		// Create a unique filename (e.g., jsmith88_1709999999.png)
		filename := fmt.Sprintf("%s_%d%s", userID, time.Now().Unix(), filepath.Ext(handler.Filename))
		imagePath = filepath.Join("uploads", filename)

		// Save physical file to your VM
		dst, err := os.Create(imagePath)
		if err != nil {
			log.Printf("Error creating file: %v", err)
			http.Error(w, `{"error": "Failed to save image"}`, http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, `{"error": "Failed to write image"}`, http.StatusInternalServerError)
			return
		}
	} else if err != http.ErrMissingFile {
		http.Error(w, `{"error": "Error reading uploaded file"}`, http.StatusBadRequest)
		return
	}

	// 4. Create the Web3 Identity on Hyperledger Fabric
	_, err = contract.SubmitTransaction("RegisterUser", userID, "0")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to register on blockchain: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 5. Save the off-chain data to the new 'users' table
	insertUserQuery := `
		INSERT INTO users (id, name, email, password_hash, phone_number, dob, license_number, license_pic_path) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = db.Exec(insertUserQuery, userID, fullName, email, password, phoneNumber, dob, licenseNum, imagePath)
	if err != nil {
		log.Printf("SQLite Error (Users Table): %v", err)
		http.Error(w, `{"error": "Failed to save user profile"}`, http.StatusInternalServerError)
		return
	}

	// 6. Initialize their Eco-Stats profile (Ready for their first trip!)
	insertEcoQuery := `INSERT INTO eco_stats (user_id) VALUES (?)`
	_, err = db.Exec(insertEcoQuery, userID)
	if err != nil {
		log.Printf("SQLite Error (Eco Stats Table): %v", err)
		// We log the error but don't crash the registration over it
	}

	// 7. Return Success to React
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Welcome to ChainRide! Your profile is under review.",
	})
}

func createAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	claims, err := verifyJWT(r)
	if err != nil {
		http.Error(w, `{"error": "Unauthorized: Invalid or missing token"}`, http.StatusUnauthorized)
		return
	}
	ownerID := claims.UserID

	var req struct {
		ID       string `json:"id"`
		Type     string `json:"assetType"`
		FuelType string `json:"fuelType"`
		Price    int    `json:"price"`
		CO2Rate  int    `json:"co2Rate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	
	_, err = contract.SubmitTransaction("CreateAsset", req.ID, req.Type, req.FuelType, ownerID, fmt.Sprintf("%d", req.Price), fmt.Sprintf("%d", req.CO2Rate))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to mint asset on blockchain: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Vehicle %s minted successfully by %s. Pending Admin Approval.", req.ID, ownerID),
	})
}

func approveAssetHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// 1. Authenticate the user from the JWT Token
	claims, err := verifyJWT(r)
	if err != nil {
		http.Error(w, `{"error": "Unauthorized: Invalid or missing token"}`, http.StatusUnauthorized)
		return
	}
	adminID := claims.UserID // We will pass this to the smart contract

	// 2. Parse the vehicle ID from the React request
	var req struct {
		VehicleID string `json:"vehicleId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// 3. Submit transaction to Hyperledger Fabric: ApproveAsset(adminID, assetID)
	_, err = contract.SubmitTransaction("ApproveAsset", adminID, req.VehicleID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to approve asset: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 4. Return success
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Vehicle %s has been approved and is now AVAILABLE on the map!", req.VehicleID),
	})
}

//  Handles Fetching Pending Vehicles for the Admin
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

	// 3. Filter only PENDING assets
	var pendingAssets []ChaincodeAsset
	for _, asset := range allAssets {
		if asset.Status == "PENDING" {
			pendingAssets = append(pendingAssets, asset)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingAssets)
}

func getEcoStatsHandler(w http.ResponseWriter, r *http.Request) {
        enableCors(&w)
        if r.Method == "OPTIONS" { return }

        // Securely get the user from the JWT token
        effectiveUser := getEffectiveUser(r)
        if effectiveUser == "" {
                http.Error(w, `{"error": "Unauthorized"}`, 401)
                return
        }

        var totalTrips int
        var totalCO2 float64
        
        // Query the SQLite database for this user's specific eco impact
        err := db.QueryRow("SELECT total_trips, total_co2_saved FROM eco_stats WHERE user_id = ?", effectiveUser).Scan(&totalTrips, &totalCO2)
        
        w.Header().Set("Content-Type", "application/json")
        if err != nil {
                // If they have no trips yet, just return 0
                json.NewEncoder(w).Encode(map[string]interface{}{"totalTrips": 0, "totalCo2Saved": 0})
                return
        }

        json.NewEncoder(w).Encode(map[string]interface{}{
                "totalTrips": totalTrips,
                "totalCo2Saved": totalCO2,
        })
}

func enableCors(w *http.ResponseWriter) {
        (*w).Header().Set("Access-Control-Allow-Origin", "*")
        (*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        (*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}
