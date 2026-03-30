package chaincode

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract exposes the ledger operations used by the ChainRide API.
type SmartContract struct {
	contractapi.Contract
}

// -----------------------------------------------------------------------------
// 1. Constants and core data types
// -----------------------------------------------------------------------------

const (
	StatusAvailable   = "AVAILABLE"
	StatusPending     = "PENDING"
	StatusBooked      = "BOOKED"
	StatusUnavailable = "UNAVAILABLE"
	RoleUser          = "USER"
	RoleAdmin         = "ADMIN"
)

// UserProfile stores the on-ledger account state for a rider, host, or admin.
type UserProfile struct {
	UserID        string  `json:"UserID"`
	Role          string  `json:"Role"`
	TokenBalance  int64   `json:"TokenBalance"`
	LoyaltyPoints int64   `json:"LoyaltyPoints"`
	TrustScore    float64 `json:"TrustScore"`
	RatingCount   int     `json:"RatingCount"`
}

// Asset represents a vehicle listing that can be booked through ChainRide.
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

// Trip captures the final rental summary written when a ride ends.
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

// Rating stores one side of the post-trip feedback exchange.
type Rating struct {
	RatingID   string  `json:"RatingID"`
	TripID     string  `json:"TripID"`
	Reviewer   string  `json:"Reviewer"`
	TargetUser string  `json:"TargetUser"`
	Stars      float64 `json:"Stars"`
	Timestamp  int64   `json:"Timestamp"`
}

// -----------------------------------------------------------------------------
// 2. Ledger initialization
// -----------------------------------------------------------------------------

func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	// Seed the default users with a baseline rating count so weighted trust
	// calculations work without special-case handling.
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

// -----------------------------------------------------------------------------
// 3. Core account and asset operations
// -----------------------------------------------------------------------------

func (s *SmartContract) RegisterUser(ctx contractapi.TransactionContextInterface, userID string, initialBalanceStr string) error {
	initialBalance, err := strconv.ParseInt(initialBalanceStr, 10, 64)
	if err != nil {
		return fmt.Errorf("initial balance must be a valid integer: %v", err)
	}

	existing, err := ctx.GetStub().GetState("USER_" + userID)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("An account with this user ID already exists")
	}

	user := UserProfile{
		UserID:        userID,
		Role:          RoleUser,
		TokenBalance:  initialBalance,
		LoyaltyPoints: 0,
		TrustScore:    5.0,
		RatingCount:   1, // Start with one baseline rating for trust score math.
	}
	userJSON, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState("USER_"+userID, userJSON)
}

func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, assetJSONString string) error {
	var newAsset Asset
	if err := json.Unmarshal([]byte(assetJSONString), &newAsset); err != nil {
		return fmt.Errorf("We could not read the vehicle details: %v", err)
	}

	ownerJSON, err := ctx.GetStub().GetState("USER_" + newAsset.Owner)
	if err != nil || ownerJSON == nil {
		return fmt.Errorf("Owner %s does not have an active account", newAsset.Owner)
	}

	assetJSON, err := ctx.GetStub().GetState("ASSET_" + newAsset.ID)
	if err != nil {
		return err
	}
	if assetJSON != nil {
		return fmt.Errorf("Vehicle %s already exists", newAsset.ID)
	}

	newAsset.Status = StatusPending
	newAsset.CurrentRenter = ""
	newAsset.StartTime = 0

	finalAssetJSON, _ := json.Marshal(newAsset)
	return ctx.GetStub().PutState("ASSET_"+newAsset.ID, finalAssetJSON)
}

func (s *SmartContract) ToggleAssetStatus(ctx contractapi.TransactionContextInterface, id string, callerUserID string) error {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil || assetJSON == nil {
		return fmt.Errorf("Vehicle %s could not be found", id)
	}

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Owner != callerUserID {
		return fmt.Errorf("You can only manage your own vehicles")
	}
	if asset.Status == StatusBooked {
		return fmt.Errorf("Vehicle %s is currently in use and cannot be updated", id)
	}

	if asset.Status == StatusAvailable {
		asset.Status = StatusUnavailable
	} else if asset.Status == StatusUnavailable {
		asset.Status = StatusAvailable
	}

	updatedAssetJSON, _ := json.Marshal(asset)
	return ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON)
}

// AdminToggleAssetStatus lets an administrator pause or restore a listing
// without deleting the underlying asset record.
func (s *SmartContract) AdminToggleAssetStatus(ctx contractapi.TransactionContextInterface, adminID string, assetID string) error {
	userJSON, err := ctx.GetStub().GetState("USER_" + adminID)
	if err != nil || userJSON == nil {
		return fmt.Errorf("The admin account could not be found")
	}

	var admin UserProfile
	json.Unmarshal(userJSON, &admin)
	if admin.Role != RoleAdmin {
		return fmt.Errorf("This action requires an administrator account")
	}

	assetJSON, err := ctx.GetStub().GetState("ASSET_" + assetID)
	if err != nil || assetJSON == nil {
		return fmt.Errorf("Vehicle %s could not be found", assetID)
	}

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status == StatusBooked {
		return fmt.Errorf("Vehicle %s is currently in use and cannot be paused", assetID)
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
	if err != nil || userJSON == nil {
		return fmt.Errorf("The admin account could not be found")
	}

	var admin UserProfile
	json.Unmarshal(userJSON, &admin)
	if admin.Role != RoleAdmin {
		return fmt.Errorf("This action requires an administrator account")
	}

	assetJSON, err := ctx.GetStub().GetState("ASSET_" + assetID)
	if err != nil || assetJSON == nil {
		return fmt.Errorf("Vehicle %s could not be found", assetID)
	}

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status != StatusPending {
		return fmt.Errorf("Vehicle %s is currently %s and cannot be approved yet", assetID, asset.Status)
	}

	asset.Status = StatusAvailable
	updatedAssetJSON, _ := json.Marshal(asset)
	return ctx.GetStub().PutState("ASSET_"+assetID, updatedAssetJSON)
}

// -----------------------------------------------------------------------------
// 4. Rental settlement and balance updates
// -----------------------------------------------------------------------------

func (s *SmartContract) RentAsset(ctx contractapi.TransactionContextInterface, id string, newRenter string) error {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil || assetJSON == nil {
		return fmt.Errorf("Vehicle %s could not be found", id)
	}

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status != StatusAvailable {
		return fmt.Errorf("Vehicle %s is not available right now", id)
	}
	if asset.Owner == newRenter {
		return fmt.Errorf("You cannot book your own vehicle")
	}

	renterJSON, err := ctx.GetStub().GetState("USER_" + newRenter)
	if err != nil || renterJSON == nil {
		return fmt.Errorf("Account %s could not be found", newRenter)
	}

	var renter UserProfile
	json.Unmarshal(renterJSON, &renter)

	if renter.TokenBalance <= 0 {
		return fmt.Errorf("This account does not have enough balance to start a trip")
	}

	// Enforce the minimum trust score before a rental can start.
	if renter.TrustScore < 2.5 {
		return fmt.Errorf("This account's trust score of %.1f is below the minimum required to start a trip", renter.TrustScore)
	}

	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	asset.Status = StatusBooked
	asset.CurrentRenter = newRenter
	asset.StartTime = txTimestamp.Seconds

	updatedAssetJSON, _ := json.Marshal(asset)
	err = ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON)
	if err != nil {
		return err
	}

	eventPayload := fmt.Sprintf(`{"assetID":"%s", "renter":"%s"}`, id, newRenter)
	ctx.GetStub().SetEvent("AssetRented", []byte(eventPayload))

	return nil
}

// ReturnAsset closes a trip using ride metrics calculated by the Go API.
// The server supplies distance, fees, emissions saved, and parking distance so
// the chaincode can settle the trip deterministically.
func (s *SmartContract) ReturnAsset(ctx contractapi.TransactionContextInterface, id string, returningUserId string, returnLatStr string, returnLonStr string, distanceStr string, platformFeeStr string, co2SavedStr string, parkedDistanceStr string) (string, error) {
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
	if err != nil {
		return "", fmt.Errorf("We could not load this vehicle right now")
	}
	if assetJSON == nil {
		return "", fmt.Errorf("Vehicle %s could not be found", id)
	}

	var asset Asset
	err = json.Unmarshal(assetJSON, &asset)
	if err != nil {
		return "", err
	}

	if asset.CurrentRenter != returningUserId {
		return "", fmt.Errorf("Only the active renter can end this trip")
	}

	// Parse the ride metrics supplied by the API layer.
	distanceKm, _ := strconv.ParseInt(distanceStr, 10, 64)
	if distanceKm < 1 {
		distanceKm = 1
	}
	co2SavedGrams, _ := strconv.ParseInt(co2SavedStr, 10, 64)
	parkedDistanceMeters, _ := strconv.ParseInt(parkedDistanceStr, 10, 64)

	renterJSON, _ := ctx.GetStub().GetState("USER_" + returningUserId)
	var renter UserProfile
	json.Unmarshal(renterJSON, &renter)

	ownerJSON, _ := ctx.GetStub().GetState("USER_" + asset.Owner)
	var owner UserProfile
	json.Unmarshal(ownerJSON, &owner)

	// Loyalty rewards combine trip completion, eco impact, and trust bonuses.
	var earnedLoyalty int64 = 2 // Base completion points

	// Base reward by asset type.
	if asset.Type == "Bike" {
		earnedLoyalty += 12
	} else if asset.Type == "Scooter" {
		earnedLoyalty += 10
	} else if asset.Type == "Car" {
		if asset.FuelType == "Electric" {
			earnedLoyalty += 8
		} else if asset.FuelType == "Hybrid" {
			earnedLoyalty += 4
		}
	}

	// Add one point for every 100 grams of emissions avoided.
	earnedLoyalty += (co2SavedGrams / 100)

	// Reward riders with consistently strong trust scores.
	if renter.TrustScore >= 4.5 {
		earnedLoyalty += 5
	} else if renter.TrustScore >= 3.5 {
		earnedLoyalty += 2
	}

	// Apply parking rules and trust adjustments based on where the vehicle
	// was returned.
	penaltyApplied := "None"
	var parkingFine int64 = 0

	if asset.Owner != "appUser" { // Only peer-to-peer vehicles use the parking geofence.
		if parkedDistanceMeters <= 150 {
			earnedLoyalty += 5
			renter.TrustScore += 0.05 // Reward careful returns close to the base location.
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
		// Fleet vehicles do not use the peer-to-peer geofence rules.
		earnedLoyalty += 5
		renter.TrustScore += 0.05
	}

	// Keep the trust score within the supported range.
	if renter.TrustScore > 5.0 {
		renter.TrustScore = 5.0
	}
	if renter.TrustScore < 1.0 {
		renter.TrustScore = 1.0
	}

	// Store the loyalty points earned for this trip.
	renter.LoyaltyPoints += earnedLoyalty

	// Calculate the final charge and split the proceeds between the owner and
	// the platform treasury.
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	endTime := txTimestamp.Seconds
	durationSeconds := endTime - asset.StartTime
	durationMins := durationSeconds / 60
	if durationMins < 1 {
		durationMins = 1
	}

	baseFee := int64(1)
	totalCost := (distanceKm * asset.PricePerKm) + baseFee + parkingFine // Include any parking fine in the final charge.

	platformFee, _ := strconv.ParseInt(platformFeeStr, 10, 64)
	if platformFee < 0 {
		platformFee = 0
	}

	ownerPayout := (totalCost - parkingFine) - platformFee
	if ownerPayout < 0 {
		ownerPayout = 0
	}
	totalPlatformRevenue := platformFee + parkingFine

	// Allow negative balances so trips can still close even when the rider ends
	// the trip in debt.
	renter.TokenBalance -= totalCost

	// Debt lowers trust and is reflected in the penalty summary.
	if renter.TokenBalance < 0 {
		renter.TrustScore -= 0.1
		if penaltyApplied == "None" {
			penaltyApplied = "Debt Incurred"
		} else {
			penaltyApplied += " & Debt Incurred"
		}
	}

	owner.TokenBalance += ownerPayout

	// Route platform fees and parking fines into the treasury account.
	treasuryJSON, _ := ctx.GetStub().GetState("USER_PlatformTreasury")
	if treasuryJSON != nil {
		var treasury UserProfile
		json.Unmarshal(treasuryJSON, &treasury)
		treasury.TokenBalance += totalPlatformRevenue
		updatedTreasury, _ := json.Marshal(treasury)
		ctx.GetStub().PutState("USER_PlatformTreasury", updatedTreasury)
	}

	// Use the Fabric transaction ID as the canonical trip identifier.
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
		CO2Saved:       co2SavedGrams,  // Preserve the server-calculated emissions value.
		PenaltyApplied: penaltyApplied, // Preserve the exact penalty description applied.
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

	// Return the trip ID so the API can mirror it into SQLite.
	return tripID, nil
}

// -----------------------------------------------------------------------------
// 5. Reputation, balances, and read queries
// -----------------------------------------------------------------------------

// RateTrip records one rating per participant side and updates the
// target user's weighted trust score.
func (s *SmartContract) RateTrip(ctx contractapi.TransactionContextInterface, tripID string, callerUserID string, stars float64) error {
	if stars < 1.0 {
		stars = 1.0
	} else if stars > 5.0 {
		stars = 5.0
	}

	tripJSON, err := ctx.GetStub().GetState(tripID)
	if err != nil || tripJSON == nil {
		return fmt.Errorf("Trip %s could not be found", tripID)
	}

	var trip Trip
	if err := json.Unmarshal(tripJSON, &trip); err != nil {
		return err
	}

	var targetUserID string
	isOwnerRatingRenter := callerUserID == trip.Owner

	if isOwnerRatingRenter {
		if trip.OwnerRated {
			return fmt.Errorf("You have already rated this trip")
		}
		targetUserID = trip.Renter
		trip.OwnerRated = true
	} else if callerUserID == trip.Renter {
		if trip.RenterRated {
			return fmt.Errorf("You have already rated this trip")
		}
		targetUserID = trip.Owner
		trip.RenterRated = true
	} else {
		return fmt.Errorf("Only people on this trip can leave a rating")
	}

	userJSON, err := ctx.GetStub().GetState("USER_" + targetUserID)
	if err != nil || userJSON == nil {
		return fmt.Errorf("The other user on this trip could not be found")
	}

	var user UserProfile
	if err := json.Unmarshal(userJSON, &user); err != nil {
		return err
	}

	// Update the trust score using the existing weighted average.
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
		return fmt.Errorf("Please enter a valid amount")
	}

	userJSON, err := ctx.GetStub().GetState("USER_" + userID)
	if err != nil || userJSON == nil {
		return fmt.Errorf("Account %s could not be found", userID)
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
		return nil, fmt.Errorf("Vehicle %s could not be found", id)
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
		return nil, fmt.Errorf("We could not load this account right now")
	}
	if userJSON == nil {
		return nil, fmt.Errorf("Account %s could not be found", userID)
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

// AdminForceEndTrip closes an in-progress ride from the admin side and
// records a minimal settlement so the asset can return to service.
func (s *SmartContract) AdminForceEndTrip(ctx contractapi.TransactionContextInterface, adminID string, assetID string) (string, error) {
	// Verify that the caller has administrator privileges.
	adminJSON, err := ctx.GetStub().GetState("USER_" + adminID)
	if err != nil || adminJSON == nil {
		return "", fmt.Errorf("The admin account could not be found")
	}

	var admin UserProfile
	json.Unmarshal(adminJSON, &admin)
	if admin.Role != RoleAdmin {
		return "", fmt.Errorf("This action requires an administrator account")
	}

	// Load the currently booked asset.
	assetJSON, err := ctx.GetStub().GetState("ASSET_" + assetID)
	if err != nil || assetJSON == nil {
		return "", fmt.Errorf("Vehicle could not be found")
	}

	var asset Asset
	json.Unmarshal(assetJSON, &asset)

	if asset.Status != StatusBooked {
		return "", fmt.Errorf("This vehicle is not currently on an active trip")
	}
	renterID := asset.CurrentRenter

	// Apply the emergency settlement. Only the base fee is charged, with no
	// distance-based billing.
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	endTime := txTimestamp.Seconds
	durationMins := (endTime - asset.StartTime) / 60
	if durationMins < 1 {
		durationMins = 1
	}

	var totalCost int64 = 1

	renterJSON, _ := ctx.GetStub().GetState("USER_" + renterID)
	var renter UserProfile
	json.Unmarshal(renterJSON, &renter)
	renter.TokenBalance -= totalCost // Emergency closures can still push the renter balance negative.

	ownerJSON, _ := ctx.GetStub().GetState("USER_" + asset.Owner)
	var owner UserProfile
	json.Unmarshal(ownerJSON, &owner)
	owner.TokenBalance += totalCost

	// Write an admin-generated trip record for auditability.
	tripID := "TRIP_ADMIN_" + ctx.GetStub().GetTxID()
	trip := Trip{
		TripID:         tripID,
		AssetID:        assetID,
		Renter:         renterID,
		Owner:          asset.Owner,
		StartTime:      asset.StartTime,
		EndTime:        endTime,
		DurationMins:   durationMins,
		TotalCost:      totalCost,
		CO2Saved:       0,
		PenaltyApplied: "Admin Emergency Termination",
	}

	// Return the asset to the available state.
	asset.Status = StatusAvailable
	asset.CurrentRenter = ""
	asset.StartTime = 0

	// Persist the asset, user balances, and trip record.
	updatedAsset, _ := json.Marshal(asset)
	ctx.GetStub().PutState("ASSET_"+assetID, updatedAsset)

	updatedRenter, _ := json.Marshal(renter)
	ctx.GetStub().PutState("USER_"+renterID, updatedRenter)

	updatedOwner, _ := json.Marshal(owner)
	ctx.GetStub().PutState("USER_"+asset.Owner, updatedOwner)

	tripBytes, _ := json.Marshal(trip)
	ctx.GetStub().PutState(tripID, tripBytes)

	return tripID, nil
}

// AdminRefundTrip reimburses a disputed trip from the treasury and marks
// the receipt so the same trip cannot be refunded twice.
func (s *SmartContract) AdminRefundTrip(ctx contractapi.TransactionContextInterface, adminID string, tripID string) error {
	// Verify that the caller has administrator privileges.
	adminJSON, err := ctx.GetStub().GetState("USER_" + adminID)
	if err != nil || adminJSON == nil {
		return fmt.Errorf("The admin account could not be found")
	}

	var admin UserProfile
	json.Unmarshal(adminJSON, &admin)
	if admin.Role != RoleAdmin {
		return fmt.Errorf("This action requires an administrator account")
	}

	// Load the disputed trip record.
	tripJSON, err := ctx.GetStub().GetState(tripID)
	if err != nil || tripJSON == nil {
		return fmt.Errorf("Trip %s could not be found", tripID)
	}

	var trip Trip
	json.Unmarshal(tripJSON, &trip)

	// Refuse repeated refunds for the same trip.
	if len(trip.PenaltyApplied) > 10 && trip.PenaltyApplied[:10] == "[REFUNDED]" {
		return fmt.Errorf("This trip has already been refunded")
	}

	// Load the renter and treasury accounts.
	renterJSON, _ := ctx.GetStub().GetState("USER_" + trip.Renter)
	var renter UserProfile
	json.Unmarshal(renterJSON, &renter)

	treasuryJSON, _ := ctx.GetStub().GetState("USER_PlatformTreasury")
	var treasury UserProfile
	if treasuryJSON != nil {
		json.Unmarshal(treasuryJSON, &treasury)
	}

	// Move the refunded amount from the treasury back to the renter.
	renter.TokenBalance += trip.TotalCost
	treasury.TokenBalance -= trip.TotalCost

	// Mark the trip so future reviews can see that it was refunded.
	trip.PenaltyApplied = "[REFUNDED] " + trip.PenaltyApplied

	// Persist the updated trip and account balances.
	updatedRenter, _ := json.Marshal(renter)
	ctx.GetStub().PutState("USER_"+trip.Renter, updatedRenter)

	if treasuryJSON != nil {
		updatedTreasury, _ := json.Marshal(treasury)
		ctx.GetStub().PutState("USER_PlatformTreasury", updatedTreasury)
	}

	updatedTrip, _ := json.Marshal(trip)
	ctx.GetStub().PutState(tripID, updatedTrip)

	return nil
}
