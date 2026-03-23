package chaincode

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

type SmartContract struct {
	contractapi.Contract
}

// ==========================================
// 1. CONSTANTS & DATA STRUCTURES
// ==========================================

const (
	StatusAvailable   = "AVAILABLE"
	StatusPending     = "PENDING"
	StatusBooked      = "BOOKED"
	StatusUnavailable = "UNAVAILABLE"
	RoleUser          = "USER"
	RoleAdmin         = "ADMIN"
)

type UserProfile struct {
	UserID        string  `json:"UserID"`
	Role          string  `json:"Role"`
	TokenBalance  int64   `json:"TokenBalance"`
	LoyaltyPoints int64   `json:"LoyaltyPoints"`
	TrustScore    float64 `json:"TrustScore"` 
	RatingCount   int     `json:"RatingCount"`
}

type Asset struct {
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

type Trip struct {
	TripID         string `json:"TripID"`
	AssetID        string `json:"AssetID"`
	Renter         string `json:"Renter"`
	Owner          string `json:"Owner"`
	StartTime      int64  `json:"StartTime"`
	EndTime        int64  `json:"EndTime"`
	DurationMins   int64  `json:"DurationMins"`
	TotalCost      int64  `json:"TotalCost"`
	CO2Saved       int64  `json:"CO2Saved"`
	PenaltyApplied string `json:"PenaltyApplied"`
	OwnerRated     bool   `json:"OwnerRated"`
	RenterRated    bool   `json:"RenterRated"`
}

type Rating struct {
	RatingID   string  `json:"RatingID"`
	TripID     string  `json:"TripID"`
	Reviewer   string  `json:"Reviewer"`
	TargetUser string  `json:"TargetUser"`
	Stars      float64 `json:"Stars"`
	Timestamp  int64   `json:"Timestamp"`
}

// ==========================================
// 2. INITIALIZATION
// ==========================================

func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	// Give seeded users 1 rating so our weighted average math doesn't divide by zero!
	users := []UserProfile{
		{UserID: "Anna", Role: RoleUser, TokenBalance: 0, LoyaltyPoints: 0, TrustScore: 5.0, RatingCount: 1},
		{UserID: "Brian", Role: RoleUser, TokenBalance: 0, LoyaltyPoints: 0, TrustScore: 5.0, RatingCount: 1},
		{UserID: "appUser", Role: RoleAdmin, TokenBalance: 5000, LoyaltyPoints: 50, TrustScore: 5.0, RatingCount: 1},
		{UserID: "PlatformTreasury", Role: RoleAdmin, TokenBalance: 0, LoyaltyPoints: 0, TrustScore: 5.0, RatingCount: 1},
	}

	for _, user := range users {
		userJSON, _ := json.Marshal(user)
		ctx.GetStub().PutState("USER_"+user.UserID, userJSON)
	}

	assets := []Asset{
		{ID: "CAR_001", Type: "Car", Make: "Tesla", Model: "Model 3", CarClass: "Sedan", Transmission: "Automatic", Seats: 5, Mileage: "14,500 km", FuelType: "Electric", BatteryLevel: "84%", Owner: "Anna", Status: "AVAILABLE", PricePerKm: 12, StartTime: 0, CurrentRenter: "", CO2SavingsRate: 100, BaseLatMicro: 46252100, BaseLonMicro: 20141000},
		{ID: "SCOOTER_001", Type: "Scooter", Make: "Xiaomi", Model: "M365", CarClass: "Micro", Transmission: "N/A", Seats: 1, Mileage: "850 km", FuelType: "Electric", BatteryLevel: "100%", Owner: "Brian", Status: "AVAILABLE", PricePerKm: 4, StartTime: 0, CurrentRenter: "", CO2SavingsRate: 130, BaseLatMicro: 46253000, BaseLonMicro: 20141400},
	}

	for _, asset := range assets {
		assetJSON, _ := json.Marshal(asset)
		ctx.GetStub().PutState("ASSET_"+asset.ID, assetJSON)
	}

	return nil
}

// ==========================================
// 3. CORE FUNCTIONS & GOVERNANCE
// ==========================================

func (s *SmartContract) RegisterUser(ctx contractapi.TransactionContextInterface, userID string, initialBalanceStr string) error {
	initialBalance, err := strconv.ParseInt(initialBalanceStr, 10, 64)
	if err != nil {
		return fmt.Errorf("initial balance must be a valid integer: %v", err)
	}

	existing, err := ctx.GetStub().GetState("USER_" + userID)
	if err != nil { return err }
	if existing != nil { return fmt.Errorf("user already exists") }

	user := UserProfile{
		UserID:        userID,
		Role:          RoleUser,
		TokenBalance:  initialBalance,
		LoyaltyPoints: 0,
		TrustScore:    5.0,
		RatingCount:   1, // Start with 1 rating of 5.0
	}
	userJSON, err := json.Marshal(user)
	if err != nil { return err }
	return ctx.GetStub().PutState("USER_"+userID, userJSON)
}

func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, assetJSONString string) error {
	var newAsset Asset
	if err := json.Unmarshal([]byte(assetJSONString), &newAsset); err != nil {
		return fmt.Errorf("failed to parse asset JSON: %v", err)
	}

	ownerJSON, err := ctx.GetStub().GetState("USER_" + newAsset.Owner)
	if err != nil || ownerJSON == nil {
		return fmt.Errorf("owner %s is not registered on the network", newAsset.Owner)
	}

	assetJSON, err := ctx.GetStub().GetState("ASSET_" + newAsset.ID)
	if err != nil { return err }
	if assetJSON != nil { return fmt.Errorf("the asset %s already exists", newAsset.ID) }

	newAsset.Status = StatusPending
	newAsset.CurrentRenter = ""
	newAsset.StartTime = 0

	finalAssetJSON, _ := json.Marshal(newAsset)
	return ctx.GetStub().PutState("ASSET_"+newAsset.ID, finalAssetJSON)
}

func (s *SmartContract) ToggleAssetStatus(ctx contractapi.TransactionContextInterface, id string, callerUserID string) error {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil || assetJSON == nil { return fmt.Errorf("asset %s does not exist", id) }

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Owner != callerUserID { return fmt.Errorf("SECURITY ALERT: User %s does not own asset %s", callerUserID, id) }
	if asset.Status == StatusBooked { return fmt.Errorf("cannot toggle status: asset %s is currently in use", id) }

	if asset.Status == StatusAvailable {
		asset.Status = StatusUnavailable
	} else if asset.Status == StatusUnavailable {
		asset.Status = StatusAvailable
	}

	updatedAssetJSON, _ := json.Marshal(asset)
	return ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON)
}

// 🚨 NEW: Admin-level manual asset suspension
func (s *SmartContract) AdminToggleAssetStatus(ctx contractapi.TransactionContextInterface, adminID string, assetID string) error {
	userJSON, err := ctx.GetStub().GetState("USER_" + adminID)
	if err != nil || userJSON == nil { return fmt.Errorf("admin user %s does not exist", adminID) }
	
	var admin UserProfile
	json.Unmarshal(userJSON, &admin)
	if admin.Role != RoleAdmin { return fmt.Errorf("SECURITY ALERT: User %s lacks ADMIN privileges", adminID) }

	assetJSON, err := ctx.GetStub().GetState("ASSET_" + assetID)
	if err != nil || assetJSON == nil { return fmt.Errorf("asset %s does not exist", assetID) }
	
	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status == StatusBooked {
		return fmt.Errorf("cannot suspend asset: asset %s is currently in use", assetID)
	}

	if asset.Status == StatusAvailable || asset.Status == StatusPending {
		asset.Status = StatusUnavailable
	} else if asset.Status == StatusUnavailable {
		asset.Status = StatusAvailable
	}

	updatedAssetJSON, _ := json.Marshal(asset)
	return ctx.GetStub().PutState("ASSET_"+assetID, updatedAssetJSON)
}

func (s *SmartContract) ApproveAsset(ctx contractapi.TransactionContextInterface, adminID string, assetID string) error {
	userJSON, err := ctx.GetStub().GetState("USER_" + adminID)
	if err != nil || userJSON == nil { return fmt.Errorf("admin user %s does not exist", adminID) }
	
	var admin UserProfile
	json.Unmarshal(userJSON, &admin)
	if admin.Role != RoleAdmin { return fmt.Errorf("SECURITY ALERT: User %s lacks ADMIN privileges", adminID) }

	assetJSON, err := ctx.GetStub().GetState("ASSET_" + assetID)
	if err != nil || assetJSON == nil { return fmt.Errorf("asset %s does not exist", assetID) }
	
	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status != StatusPending { return fmt.Errorf("asset %s is currently %s and cannot be approved", assetID, asset.Status) }

	asset.Status = StatusAvailable
	updatedAssetJSON, _ := json.Marshal(asset)
	return ctx.GetStub().PutState("ASSET_"+assetID, updatedAssetJSON)
}

// =========================================================================================
// 4. THE DECENTRALIZED BANK: RENTAL LOGIC & PAYMENTS
// =========================================================================================

func (s *SmartContract) RentAsset(ctx contractapi.TransactionContextInterface, id string, newRenter string) error {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil || assetJSON == nil { return fmt.Errorf("asset %s not found", id) }

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status != StatusAvailable { return fmt.Errorf("asset %s is not available", id) }
	if asset.Owner == newRenter { return fmt.Errorf("owners cannot rent their own assets") }

	renterJSON, err := ctx.GetStub().GetState("USER_" + newRenter)
	if err != nil || renterJSON == nil { return fmt.Errorf("renter %s does not exist", newRenter) }

	var renter UserProfile
	json.Unmarshal(renterJSON, &renter)
	
	if renter.TokenBalance <= 0 { return fmt.Errorf("renter %s has insufficient funds", newRenter) }
	
	// 🚨 GOVERNANCE: Algorithmic Ban
	if renter.TrustScore < 2.5 { 
		return fmt.Errorf("NETWORK BAN: Trust Score of %.1f is below the 2.5 minimum required to rent", renter.TrustScore) 
	}

	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	asset.Status = StatusBooked
	asset.CurrentRenter = newRenter
	asset.StartTime = txTimestamp.Seconds

	updatedAssetJSON, _ := json.Marshal(asset)
	err = ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON)
	if err != nil { return err }

	eventPayload := fmt.Sprintf(`{"assetID":"%s", "renter":"%s"}`, id, newRenter)
	ctx.GetStub().SetEvent("AssetRented", []byte(eventPayload))

	return nil
}

// 🛑 RETURN ASSET — Distance-Based Billing (Oracle Pattern)
// The Go Server simulates distance and passes it deterministically.
// 🛑 RETURN ASSET — Thesis-Driven Tokenomics & Soft Geofence
func (s *SmartContract) ReturnAsset(ctx contractapi.TransactionContextInterface, id string, returningUserId string, returnLatStr string, returnLonStr string, distanceStr string, platformFeeStr string, co2SavedStr string, parkedDistanceStr string) (string, error) {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil { return "", fmt.Errorf("failed to read from world state: %v", err) }
	if assetJSON == nil { return "", fmt.Errorf("the asset %s does not exist", id) }

	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil { return "", err }

	if asset.CurrentRenter != returningUserId {
		return "", fmt.Errorf("user %s is not the current renter of this vehicle", returningUserId)
	}

	// 🚨 TOKENOMICS ENGINE: Parse Oracle Data
	distanceKm, _ := strconv.ParseInt(distanceStr, 10, 64)
	if distanceKm < 1 { distanceKm = 1 }
	co2SavedGrams, _ := strconv.ParseInt(co2SavedStr, 10, 64)
	parkedDistanceMeters, _ := strconv.ParseInt(parkedDistanceStr, 10, 64)

	renterJSON, _ := ctx.GetStub().GetState("USER_" + returningUserId)
	var renter UserProfile
	json.Unmarshal(renterJSON, &renter)

	ownerJSON, _ := ctx.GetStub().GetState("USER_" + asset.Owner)
	var owner UserProfile
	json.Unmarshal(ownerJSON, &owner)

	// ==========================================
	// 🌟 LOYALTY POINTS CALCULATION
	// ==========================================
	var earnedLoyalty int64 = 2 // Base completion points

	// 1. Eco-Bonus by Vehicle Type
	if asset.Type == "Bike" { earnedLoyalty += 12 } else
	if asset.Type == "Scooter" { earnedLoyalty += 10 } else
	if asset.Type == "Car" {
		if asset.FuelType == "Electric" { earnedLoyalty += 8 } else
		if asset.FuelType == "Hybrid" { earnedLoyalty += 4 }
	}

	// 2. CO2 Bonus (1 point per 100g saved)
	earnedLoyalty += (co2SavedGrams / 100)

	// 3. Trust Bonus
	if renter.TrustScore >= 4.5 { earnedLoyalty += 5 } else
	if renter.TrustScore >= 3.5 { earnedLoyalty += 2 }

	// ==========================================
	// 🛑 PENALTY SYSTEM & TRUST SCORE
	// ==========================================
	penaltyApplied := "None"
	var parkingFine int64 = 0

	if asset.Owner != "appUser" { // Only enforce geofences for P2P vehicles
		if parkedDistanceMeters <= 150 {
			earnedLoyalty += 5
			renter.TrustScore += 0.05 // Good parking reward
		} else if parkedDistanceMeters <= 300 {
			parkingFine = 5
			penaltyApplied = "Minor Parking Violation (150m-300m)"
			renter.TrustScore -= 0.1
		} else if parkedDistanceMeters <= 1000 {
			parkingFine = 15
			penaltyApplied = "Moderate Parking Violation (300m-1km)"
			renter.TrustScore -= 0.2
		} else {
			parkingFine = 40
			penaltyApplied = "Severe Abandonment (>1km)"
			renter.TrustScore -= 0.5
		}
	} else {
		// CR Fleet vehicles are dockless; always reward good returns
		earnedLoyalty += 5
		renter.TrustScore += 0.05
	}

	// Bound Trust Score between 1.0 and 5.0
	if renter.TrustScore > 5.0 { renter.TrustScore = 5.0 }
	if renter.TrustScore < 1.0 { renter.TrustScore = 1.0 }

	// Apply Loyalty Points
	renter.LoyaltyPoints += earnedLoyalty

	// ==========================================
	// 🛣️ BILLING & PAYMENTS
	// ==========================================
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	endTime := txTimestamp.Seconds
	durationSeconds := endTime - asset.StartTime
	durationMins := durationSeconds / 60
	if durationMins < 1 { durationMins = 1 } 

	baseFee := int64(1)
	totalCost := (distanceKm * asset.PricePerKm) + baseFee + parkingFine // Fine added to bill

	if renter.TokenBalance < totalCost {
		return "", fmt.Errorf("insufficient funds to cover trip cost and penalties. Needed: %d CRT", totalCost)
	}

	platformFee, _ := strconv.ParseInt(platformFeeStr, 10, 64)
	if platformFee < 0 { platformFee = 0 }

	// The fine goes to the Treasury, not the host
	ownerPayout := (totalCost - parkingFine) - platformFee
	if ownerPayout < 0 { ownerPayout = 0 }
	totalPlatformRevenue := platformFee + parkingFine

	renter.TokenBalance -= totalCost
	owner.TokenBalance += ownerPayout

	// Send Platform Fees & Parking Fines to the Treasury
	treasuryJSON, _ := ctx.GetStub().GetState("USER_PlatformTreasury")
	if treasuryJSON != nil {
		var treasury UserProfile
		json.Unmarshal(treasuryJSON, &treasury)
		treasury.TokenBalance += totalPlatformRevenue
		updatedTreasury, _ := json.Marshal(treasury)
		ctx.GetStub().PutState("USER_PlatformTreasury", updatedTreasury)
	}

	// 🚨 GENERATE THE BLOCKCHAIN HASH ID
	tripID := "TRIP_" + ctx.GetStub().GetTxID() 
	trip := Trip{
		TripID:         tripID,
		AssetID:        id,
		Renter:         returningUserId,
		Owner:          asset.Owner,
		StartTime:      asset.StartTime,
		EndTime:        endTime,
		DurationMins:   durationMins,
		TotalCost:      totalCost,
		CO2Saved:       co2SavedGrams, // 🚨 Now logs actual Oracle math
		PenaltyApplied: penaltyApplied, // 🚨 Now logs exactly how bad the parking was
		OwnerRated:     false,
		RenterRated:    false,
	}

	asset.CurrentRenter = ""
	asset.Status = StatusAvailable
	asset.StartTime = 0

	updatedAssetJSON, _ := json.Marshal(asset)
	ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON)

	updatedRenterJSON, _ := json.Marshal(renter)
	ctx.GetStub().PutState("USER_"+returningUserId, updatedRenterJSON)

	updatedOwnerJSON, _ := json.Marshal(owner)
	ctx.GetStub().PutState("USER_"+asset.Owner, updatedOwnerJSON)

	tripJSON, _ := json.Marshal(trip)
	ctx.GetStub().PutState(tripID, tripJSON)

	// 🚨 RETURN THE ID TO THE GO SERVER
	return tripID, nil
}

// ==========================================
// 5. REPUTATION, BANKING & READ QUERIES
// ==========================================

// 🛡️ UPGRADED: Secure, Trip-Based Weighted Reputation System
func (s *SmartContract) RateTrip(ctx contractapi.TransactionContextInterface, tripID string, callerUserID string, stars float64) error {
	if stars < 1.0 { stars = 1.0 } else if stars > 5.0 { stars = 5.0 }

	tripJSON, err := ctx.GetStub().GetState(tripID)
	if err != nil || tripJSON == nil { 
		return fmt.Errorf("trip %s not found on the ledger", tripID) 
	}

	var trip Trip
	if err := json.Unmarshal(tripJSON, &trip); err != nil { return err }

	var targetUserID string
	isOwnerRatingRenter := callerUserID == trip.Owner

	if isOwnerRatingRenter {
		if trip.OwnerRated { return fmt.Errorf("owner has already rated this trip") }
		targetUserID = trip.Renter
		trip.OwnerRated = true
	} else if callerUserID == trip.Renter {
		if trip.RenterRated { return fmt.Errorf("renter has already rated this trip") }
		targetUserID = trip.Owner
		trip.RenterRated = true
	} else {
		return fmt.Errorf("SECURITY ALERT: User %s was not a participant in trip %s", callerUserID, tripID)
	}

	userJSON, err := ctx.GetStub().GetState("USER_" + targetUserID)
	if err != nil || userJSON == nil { return fmt.Errorf("target user %s not found", targetUserID) }

	var user UserProfile
	if err := json.Unmarshal(userJSON, &user); err != nil { return err }

	// 🧮 WEIGHTED AVERAGE MATH
	totalScore := user.TrustScore * float64(user.RatingCount)
	totalScore += stars
	user.RatingCount++
	user.TrustScore = totalScore / float64(user.RatingCount)

	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	ratingID := "RATING_" + ctx.GetStub().GetTxID()
	newRating := Rating{
		RatingID:   ratingID,
		TripID:     tripID,
		Reviewer:   callerUserID,
		TargetUser: targetUserID,
		Stars:      stars,
		Timestamp:  txTimestamp.Seconds,
	}

	updatedUser, _ := json.Marshal(user)
	ctx.GetStub().PutState("USER_"+targetUserID, updatedUser)

	updatedTrip, _ := json.Marshal(trip)
	ctx.GetStub().PutState(tripID, updatedTrip)

	ratingBytes, _ := json.Marshal(newRating)
	ctx.GetStub().PutState(ratingID, ratingBytes)

	return nil
}

func (s *SmartContract) TopUpWallet(ctx contractapi.TransactionContextInterface, userID string, amountStr string) error {
	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil || amount <= 0 {
		return fmt.Errorf("invalid amount")
	}

	userJSON, err := ctx.GetStub().GetState("USER_" + userID)
	if err != nil || userJSON == nil {
		return fmt.Errorf("user %s does not exist", userID)
	}

	var user UserProfile
	if err := json.Unmarshal(userJSON, &user); err != nil {
		return err
	}
	user.TokenBalance += amount

	updatedUserJSON, _ := json.Marshal(user)
	return ctx.GetStub().PutState("USER_"+userID, updatedUserJSON)
}

func (s *SmartContract) ReadAsset(ctx contractapi.TransactionContextInterface, id string) (*Asset, error) {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil || assetJSON == nil {
		return nil, fmt.Errorf("asset %s does not exist", id)
	}
	var asset Asset
	if err := json.Unmarshal(assetJSON, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *SmartContract) GetAllAssets(ctx contractapi.TransactionContextInterface) ([]*Asset, error) {
	resultsIterator, err := ctx.GetStub().GetStateByRange("ASSET_", "ASSET_\uffff")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var assets []*Asset
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var asset Asset
		if err := json.Unmarshal(queryResponse.Value, &asset); err != nil {
			return nil, err
		}
		assets = append(assets, &asset)
	}
	return assets, nil
}

func (s *SmartContract) GetUser(ctx contractapi.TransactionContextInterface, userID string) (*UserProfile, error) {
	userJSON, err := ctx.GetStub().GetState("USER_" + userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if userJSON == nil {
		return nil, fmt.Errorf("the user %s does not exist", userID)
	}

	var user UserProfile
	if err := json.Unmarshal(userJSON, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

type TxTimestamp struct {
	Seconds int64 `json:"seconds"`
}

type HistoryQueryResult struct {
	TxId      string      `json:"TxId"`
	Timestamp TxTimestamp `json:"Timestamp"`
	Value     Asset       `json:"Value"`
}

func (s *SmartContract) GetAssetHistory(ctx contractapi.TransactionContextInterface, id string) ([]HistoryQueryResult, error) {
	resultsIterator, err := ctx.GetStub().GetHistoryForKey("ASSET_" + id)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var records []HistoryQueryResult
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var asset Asset
		if len(response.Value) > 0 {
			if err := json.Unmarshal(response.Value, &asset); err != nil {
				return nil, err
			}
		}

		record := HistoryQueryResult{
			TxId:  response.TxId,
			Value: asset,
		}

		if response.Timestamp != nil {
			record.Timestamp.Seconds = response.Timestamp.Seconds
		}

		records = append(records, record)
	}

	return records, nil
}