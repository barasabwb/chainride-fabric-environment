package main

import (
        "database/sql"
        "encoding/json"
        "fmt"
        "log"
        "net/http"
        "os"
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
        Password string `json:"password"` // 🛡️ Used only for Admin logins
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

        createTableQuery := `
        CREATE TABLE IF NOT EXISTS vehicle_metadata (
                id TEXT PRIMARY KEY,
                latitude REAL,
                longitude REAL
        );`
        if _, err := db.Exec(createTableQuery); err != nil {
                log.Fatalf("Fatal Error creating SQLite table: %v", err)
        }

        seedQuery := `
        INSERT OR IGNORE INTO vehicle_metadata (id, latitude, longitude) VALUES 
        ('CAR_001', 46.2512, 20.1458),
        ('SCOOTER_001', 46.2530, 20.1410),
        ('BIKE_001', 46.2550, 20.1500);
        `
        if _, err := db.Exec(seedQuery); err != nil {
                log.Fatalf("Fatal Error seeding SQLite data: %v", err)
        }
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
		// fmt.Printf("🚨 DEBUG LOGIN - Received Pass: '%s' | Expected Pass: '%s'\n", req.Password, adminPin)
                if adminPin == "" { adminPin = "admin123" } // Fallback for prototype

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
                // 🛡️ SECURITY: Enforce HS256 algorithm
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
                err := db.QueryRow("SELECT latitude, longitude FROM vehicle_metadata WHERE id = ?", asset.ID).Scan(&lat, &lon)
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

        result, err := contract.SubmitTransaction("ReturnAsset", req.VehicleID, effectiveUser)
        if err != nil {
                w.WriteHeader(500)
                w.Write([]byte(fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error())))
                return
        }

        json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "receipt": string(result)})
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

        // 🛡️ SECURITY: Demo Faucet Limit (Max $5000 per request)
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

	var req struct {
		UserID         string `json:"userId"`
		Password       string `json:"password"`
		InitialBalance int    `json:"initialBalance"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	
	_, err := contract.SubmitTransaction("RegisterUser", req.UserID, fmt.Sprintf("%d", req.InitialBalance))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to register on blockchain: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Return success back to React
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Wallet created for %s with %d tokens", req.UserID, req.InitialBalance),
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

func enableCors(w *http.ResponseWriter) {
        (*w).Header().Set("Access-Control-Allow-Origin", "*")
        (*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        (*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}
