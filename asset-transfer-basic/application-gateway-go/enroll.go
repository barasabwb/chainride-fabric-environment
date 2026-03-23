package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

func main() {
	log.Println("============ Creating Wallet ============")

	// 1. Create the wallet folder
	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}

	if !wallet.Exists("appUser") {
		// 2. USE THE ABSOLUTE PATH YOU FOUND
		// We are looking for Admin's keys in the crypto-config folder
		credPath := "/home/were_brian329/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"

		// 3. Read the Certificate
		// Note: The file is named "Admin@org1.example.com-cert.pem"
		certPath := filepath.Join(credPath, "signcerts", "cert.pem")
		cert, err := os.ReadFile(certPath)
		if err != nil {
			log.Fatalf("Failed to read certificate: %v", err)
		}

		// 4. Read the Private Key
		keyDir := filepath.Join(credPath, "keystore")
		files, err := os.ReadDir(keyDir)
		if err != nil {
			log.Fatalf("Failed to read key directory: %v", err)
		}
		// The key file name is random (e.g., "sk_..."), so we just grab the first file
		keyPath := filepath.Join(keyDir, files[0].Name())
		key, err := os.ReadFile(keyPath)
		if err != nil {
			log.Fatalf("Failed to read key: %v", err)
		}

		// 5. Create the Identity and put it in the wallet
		// We label it "appUser" so server.go can find it easily
		identity := gateway.NewX509Identity("Org1MSP", string(cert), string(key))
		
		err = wallet.Put("appUser", identity)
		if err != nil {
			log.Fatalf("Failed to put identity into wallet: %v", err)
		}
		fmt.Println("Success! 'appUser' (using Admin creds) created in ./wallet")
	} else {
		fmt.Println("Wallet already exists. 'appUser' is ready.")
	}
}

