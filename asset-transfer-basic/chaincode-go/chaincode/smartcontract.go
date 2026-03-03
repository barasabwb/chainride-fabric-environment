package chaincode

import (
        "encoding/json"
        "fmt"
        "strconv" // 🛡️ Needed for safe type conversion

        "github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

type SmartContract struct {
        contractapi.Contract
}

// ==========================================
// 1. DATA STRUCTURES
// ==========================================

type UserProfile struct {
        UserID        string  `json:"UserID"`
        Role          string  `json:"Role"`
        TokenBalance  int     `json:"TokenBalance"`
        LoyaltyPoints int     `json:"LoyaltyPoints"`
        TrustScore    float64 `json:"TrustScore"`
}

type Asset struct {
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

// ==========================================
// 2. INITIALIZATION
// ==========================================

func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
        users := []UserProfile{
                {UserID: "Anna", Role: "USER", TokenBalance: 0, LoyaltyPoints: 0, TrustScore: 5.0},
                {UserID: "Brian", Role: "USER", TokenBalance: 0, LoyaltyPoints: 0, TrustScore: 5.0},
                {UserID: "appUser", Role: "ADMIN", TokenBalance: 5000, LoyaltyPoints: 50, TrustScore: 5.0},
        }

        for _, user := range users {
                userJSON, err := json.Marshal(user)
                if err != nil {
                        return err
                }
                if err = ctx.GetStub().PutState("USER_"+user.UserID, userJSON); err != nil {
                        return fmt.Errorf("failed to put user to world state: %v", err)
                }
        }

        assets := []Asset{
                {ID: "CAR_001", Type: "Car", FuelType: "Electric", Owner: "Anna", Status: "AVAILABLE", PricePerMinute: 8, StartTime: 0, CurrentRenter: "", CO2SavingsRate: 100},
                {ID: "SCOOTER_001", Type: "Scooter", FuelType: "Electric", Owner: "Brian", Status: "AVAILABLE", PricePerMinute: 3, StartTime: 0, CurrentRenter: "", CO2SavingsRate: 130},
                {ID: "BIKE_001", Type: "Bike", FuelType: "Human", Owner: "Bence", Status: "AVAILABLE", PricePerMinute: 1, StartTime: 0, CurrentRenter: "", CO2SavingsRate: 150},
        }

        for _, asset := range assets {
                assetJSON, err := json.Marshal(asset)
                if err != nil {
                        return err
                }
                if err = ctx.GetStub().PutState("ASSET_"+asset.ID, assetJSON); err != nil {
                        return fmt.Errorf("failed to put asset to world state: %v", err)
                }
        }

        return nil
}

// ==========================================
// 3. CORE FUNCTIONS & GOVERNANCE
// ==========================================

func (s *SmartContract) RegisterUser(ctx contractapi.TransactionContextInterface, userID string, initialBalanceStr string) error {
        initialBalance, err := strconv.Atoi(initialBalanceStr)
        if err != nil {
                return fmt.Errorf("initial balance must be a valid integer: %v", err)
        }

        user := UserProfile{
                UserID:        userID,
                Role:          "USER",
                TokenBalance:  initialBalance,
                LoyaltyPoints: 0,
                TrustScore:    5.0,
        }
        userJSON, err := json.Marshal(user)
        if err != nil {
                return err
        }
        return ctx.GetStub().PutState("USER_"+userID, userJSON)
}

func (s *SmartContract) CreateAsset(ctx contractapi.TransactionContextInterface, id string, assetType string, fuelType string, owner string, price int, co2Rate int) error {
        assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
        if err != nil {
                return fmt.Errorf("failed to read from world state: %v", err)
        }
        if assetJSON != nil {
                return fmt.Errorf("the asset %s already exists", id)
        }

        asset := Asset{
                ID:             id,
                Type:           assetType,
                FuelType:       fuelType,
                Owner:          owner,
                Status:         "PENDING",
                PricePerMinute: price,
                StartTime:      0,
                CurrentRenter:  "",
                CO2SavingsRate: co2Rate,
        }
        newAssetJSON, err := json.Marshal(asset)
        if err != nil {
                return err
        }
        return ctx.GetStub().PutState("ASSET_"+id, newAssetJSON)
}

func (s *SmartContract) ApproveAsset(ctx contractapi.TransactionContextInterface, adminID string, assetID string) error {
        userJSON, err := ctx.GetStub().GetState("USER_" + adminID)
        if err != nil || userJSON == nil {
                return fmt.Errorf("admin user %s does not exist", adminID)
        }

        var admin UserProfile
        if err := json.Unmarshal(userJSON, &admin); err != nil {
                return err
        }
        if admin.Role != "ADMIN" {
                return fmt.Errorf("SECURITY ALERT: User %s lacks ADMIN privileges", adminID)
        }

        assetJSON, err := ctx.GetStub().GetState("ASSET_" + assetID)
        if err != nil || assetJSON == nil {
                return fmt.Errorf("asset %s does not exist", assetID)
        }

        var asset Asset
        if err := json.Unmarshal(assetJSON, &asset); err != nil {
                return err
        }
        if asset.Status != "PENDING" {
                return fmt.Errorf("asset %s is already processed", assetID)
        }

        asset.Status = "AVAILABLE"
        updatedAssetJSON, err := json.Marshal(asset)
        if err != nil {
                return err
        }
        return ctx.GetStub().PutState("ASSET_"+assetID, updatedAssetJSON)
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
        // 🛡️ Bulletproof range query using highest Unicode character
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

// =========================================================================================
// 4. THE DECENTRALIZED BANK: RENTAL LOGIC & PAYMENTS
// =========================================================================================

func (s *SmartContract) RentAsset(ctx contractapi.TransactionContextInterface, id string, newRenter string) error {
        assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
        if err != nil || assetJSON == nil {
                return fmt.Errorf("asset %s not found", id)
        }

        var asset Asset
        if err := json.Unmarshal(assetJSON, &asset); err != nil {
                return err
        }

        // 🛡️ Ensure asset is actually AVAILABLE
        if asset.Status != "AVAILABLE" {
                return fmt.Errorf("asset %s is not available", id)
        }

        // 🛡️ Ensure renter exists and has funds
        renterJSON, err := ctx.GetStub().GetState("USER_" + newRenter)
        if err != nil || renterJSON == nil {
                return fmt.Errorf("renter %s does not exist on the network", newRenter)
        }

        var renter UserProfile
        if err := json.Unmarshal(renterJSON, &renter); err != nil {
                return err
        }
        if renter.TokenBalance <= 0 {
                return fmt.Errorf("renter %s has insufficient funds to start a trip", newRenter)
        }

        txTimestamp, err := ctx.GetStub().GetTxTimestamp()
        if err != nil {
                return err
        }

        asset.Status = "BOOKED"
        asset.CurrentRenter = newRenter
        asset.StartTime = txTimestamp.Seconds

        updatedAssetJSON, err := json.Marshal(asset)
        if err != nil {
                return err
        }
        return ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON)
}

func (s *SmartContract) ReturnAsset(ctx contractapi.TransactionContextInterface, id string, callerUserID string) (string, error) {
        assetJSON, err := ctx.GetStub().GetState("ASSET_" + id)
        if err != nil || assetJSON == nil {
                return "", fmt.Errorf("asset not found")
        }

        var asset Asset
        if err := json.Unmarshal(assetJSON, &asset); err != nil {
                return "", err
        }
        if asset.Status != "BOOKED" {
                return "", fmt.Errorf("asset %s is not currently booked", id)
        }

        // 🛡️ SECURE CHECK: Only the recorded renter can return the asset
        if asset.CurrentRenter != callerUserID {
                return "", fmt.Errorf("SECURITY ALERT: user %s is not authorized to return vehicle rented by %s", callerUserID, asset.CurrentRenter)
        }

        txTimestamp, err := ctx.GetStub().GetTxTimestamp()
        if err != nil {
                return "", err
        }

        durationMinutes := (txTimestamp.Seconds - asset.StartTime) / 60
        if durationMinutes < 1 {
                durationMinutes = 1
        }

        co2Saved := int(durationMinutes) * asset.CO2SavingsRate

        // 🛡️ CORRECTED TOKENOMICS MATH
        baseCost := int(durationMinutes) * asset.PricePerMinute
        lateFee := 0
        if durationMinutes > 1440 {
                lateFee = 50
        }

        platformCut := baseCost * 10 / 100
        ownerEarnings := baseCost - platformCut
        totalCost := baseCost + lateFee

        // 🛡️ FETCH WALLETS & FAIL IF MISSING (No more free money)
        renterID := asset.CurrentRenter
        ownerID := asset.Owner

        renterJSON, err := ctx.GetStub().GetState("USER_" + renterID)
        if err != nil || renterJSON == nil {
                return "", fmt.Errorf("CRITICAL: Renter %s wallet missing", renterID)
        }
        var renter UserProfile
        if err := json.Unmarshal(renterJSON, &renter); err != nil {
                return "", err
        }

        if renter.TokenBalance < totalCost {
                return "", fmt.Errorf("renter %s has insufficient tokens. Cost: %d, Balance: %d", renterID, totalCost, renter.TokenBalance)
        }

        ownerJSON, err := ctx.GetStub().GetState("USER_" + ownerID)
        if err != nil || ownerJSON == nil {
                return "", fmt.Errorf("CRITICAL: Owner %s wallet missing", ownerID)
        }
        var owner UserProfile
        if err := json.Unmarshal(ownerJSON, &owner); err != nil {
                return "", err
        }

        treasuryJSON, err := ctx.GetStub().GetState("USER_PlatformTreasury")
        var treasury UserProfile
        if treasuryJSON != nil {
                json.Unmarshal(treasuryJSON, &treasury)
        } else {
                // Treasury is the only wallet allowed to auto-create on demand
                treasury = UserProfile{UserID: "PlatformTreasury", TokenBalance: 0, LoyaltyPoints: 0, TrustScore: 5.0}
        }

        // 🛡️ EXECUTE TRANSFERS
        renter.TokenBalance -= totalCost
        owner.TokenBalance += ownerEarnings
        treasury.TokenBalance += (platformCut + lateFee) // Treasury gets the platform cut AND late fee!
        renter.LoyaltyPoints += co2Saved

        // 🛡️ SAVE STATE & STRICT ERROR CHECKING
        renterBytes, err := json.Marshal(renter)
        if err != nil {
                return "", fmt.Errorf("failed to marshal renter: %v", err)
        }

        ownerBytes, err := json.Marshal(owner)
        if err != nil {
                return "", fmt.Errorf("failed to marshal owner: %v", err)
        }

        treasuryBytes, err := json.Marshal(treasury)
        if err != nil {
                return "", fmt.Errorf("failed to marshal treasury: %v", err)
        }

        if err := ctx.GetStub().PutState("USER_"+renterID, renterBytes); err != nil {
                return "", err
        }
        if err := ctx.GetStub().PutState("USER_"+ownerID, ownerBytes); err != nil {
                return "", err
        }
        if err := ctx.GetStub().PutState("USER_PlatformTreasury", treasuryBytes); err != nil {
                return "", err
        }

        asset.Status = "AVAILABLE"
        asset.CurrentRenter = ""
        asset.StartTime = 0

        updatedAssetJSON, err := json.Marshal(asset)
        if err != nil {
                return "", fmt.Errorf("failed to marshal asset: %v", err)
        }
        if err := ctx.GetStub().PutState("ASSET_"+id, updatedAssetJSON); err != nil {
                return "", err
        }

        receipt := fmt.Sprintf("Trip Complete! %d mins. Paid Total: $%d (Base: $%d, Late Fee: $%d). 🌱 Saved %dg CO2!",
                durationMinutes, totalCost, baseCost, lateFee, co2Saved)
        return receipt, nil
}

// ==========================================
// 5. BANKING & REPUTATION FUNCTIONS
// ==========================================

// 🛡️ TOP UP WALLET ACCEPTS STRING AND PARSES SAFELY
func (s *SmartContract) TopUpWallet(ctx contractapi.TransactionContextInterface, userID string, amountStr string) error {
        amount, err := strconv.Atoi(amountStr)
        if err != nil {
                return fmt.Errorf("amount must be a valid integer: %v", err)
        }
        if amount <= 0 {
                return fmt.Errorf("top-up amount must be greater than zero")
        }

        userJSON, err := ctx.GetStub().GetState("USER_" + userID)
        if err != nil || userJSON == nil {
                return fmt.Errorf("cannot top up: user %s does not exist", userID)
        }

        var user UserProfile
        if err := json.Unmarshal(userJSON, &user); err != nil {
                return err
        }

        user.TokenBalance += amount

        updatedUserJSON, err := json.Marshal(user)
        if err != nil {
                return err
        }

        return ctx.GetStub().PutState("USER_"+userID, updatedUserJSON)
}

func (s *SmartContract) RateUser(ctx contractapi.TransactionContextInterface, targetUserID string, stars float64) error {
        if stars < 1.0 {
                stars = 1.0
        } else if stars > 5.0 {
                stars = 5.0
        }

        userJSON, err := ctx.GetStub().GetState("USER_" + targetUserID)
        if err != nil || userJSON == nil {
                return fmt.Errorf("user %s not found", targetUserID)
        }

        var user UserProfile
        if err := json.Unmarshal(userJSON, &user); err != nil {
                return err
        }

        user.TrustScore = (user.TrustScore + stars) / 2.0
        updatedUser, err := json.Marshal(user)
        if err != nil {
                return err
        }
        return ctx.GetStub().PutState("USER_"+targetUserID, updatedUser)
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


// 🛡️ BUGFIX: We extract the Timestamp into its own named struct so Fabric's 
// schema generator doesn't get confused and corrupt the validation rules.
type TxTimestamp struct {
	Seconds int64 `json:"seconds"`
}

type HistoryQueryResult struct {
	TxId      string      `json:"TxId"`
	Timestamp TxTimestamp `json:"Timestamp"`
	Value     Asset       `json:"Value"`
}

// GetAssetHistory returns the chain of custody and state changes for an asset
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
			err = json.Unmarshal(response.Value, &asset)
			if err != nil {
				return nil, err
			}
		}

		record := HistoryQueryResult{
			TxId:  response.TxId,
			Value: asset,
		}
		
		// Safely grab the timestamp
		if response.Timestamp != nil {
			record.Timestamp.Seconds = response.Timestamp.Seconds
		}
		
		records = append(records, record)
	}

	return records, nil
}
