package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// processContracts processes contract-related information from a transaction
func processContracts(tx *models.Transaction) (string, string, string, bool) {
	var to string
	var contractAddress string
	var statusTx string
	var isContract bool

	// Check if it's a contract creation transaction
	if tx.To == "" {
		// Get contract address and status from transaction receipt
		var err error
		contractAddress, statusTx, err = rpc.GetContractAddress(tx.Hash)
		if err != nil {
			configs.Logger.Error("Failed to get contract address",
				zap.String("hash", tx.Hash),
				zap.Error(err))
			return "", "", "", false
		}

		if contractAddress != "" {
			isContract = true

			// Get contract code
			contractCode, err := rpc.GetCode(contractAddress, "latest")
			if err != nil {
				configs.Logger.Error("Failed to get contract code",
					zap.String("address", contractAddress),
					zap.Error(err))
			}

			// Classify the contract (ERC-20 / ERC-721 / ERC-1155 / unknown).
			// On transient probe failure the result is zero-valued; the
			// StoreContract merge's promote-only invariant prevents that
			// from clobbering a previously-good classification.
			detection, detErr := rpc.DetectContractType(contractAddress)
			if detErr != nil {
				configs.Logger.Warn("Contract type detection failed; storing without classification",
					zap.String("address", contractAddress),
					zap.Error(detErr))
			}

			// Store complete contract information
			contract := models.ContractInfo{
				Address:             contractAddress,
				Status:              statusTx,
				IsToken:             detection.Standard != "",
				Name:                detection.Name,
				Symbol:              detection.Symbol,
				Decimals:            detection.Decimals,
				TotalSupply:         detection.TotalSupply,
				TokenStandard:       detection.Standard,
				HasERC165:           detection.HasERC165,
				ContractCode:        contractCode,
				CreatorAddress:      tx.From,
				CreationTransaction: tx.Hash,
				CreationBlockNumber: tx.BlockNumber,
				UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
			}

			// Store the contract
			err = StoreContract(contract)
			if err != nil {
				configs.Logger.Error("Failed to store contract",
					zap.String("address", contractAddress),
					zap.Error(err))
			}
		}
	} else {
		to = tx.To
		statusTx = tx.Status

		// Check if the destination address is a contract
		isContract = IsAddressContract(to)
	}

	return to, contractAddress, statusTx, isContract
}

// IsAddressContract checks if an address is a contract by querying the contractCode collection
// and falling back to RPC getCode call if not found
func IsAddressContract(address string) bool {
	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)

	// First check our database
	contract := getContractFromDB(address)
	if contract != nil {
		return true
	}

	// If not in database, check via RPC
	code, err := rpc.GetCode(address, "latest")
	if err != nil {
		configs.Logger.Error("Failed to get code for address",
			zap.String("address", address),
			zap.Error(err))
		return false
	}

	// If code is not empty/0x, it's a contract
	isContract := code != "" && code != "0x" && code != "0x0"

	// If it's a contract, store it in our database
	if isContract {
		configs.Logger.Info("Detected existing contract",
			zap.String("address", address))

		// Classify (ERC-20 / 721 / 1155 / unknown). Transient probe
		// failures are logged but non-fatal, StoreContract preserves
		// any previously-good classification through its merge.
		detection, detErr := rpc.DetectContractType(address)
		if detErr != nil {
			configs.Logger.Warn("Contract type detection failed; storing without classification",
				zap.String("address", address),
				zap.Error(detErr))
		}

		// First try to get existing contract from both collections to preserve creation data
		existingContract, err := GetContract(address)

		// Create base contract info
		contract := models.ContractInfo{
			Address:       address,
			Status:        "0x1", // Assume successful
			IsToken:       detection.Standard != "",
			Name:          detection.Name,
			Symbol:        detection.Symbol,
			Decimals:      detection.Decimals,
			TotalSupply:   detection.TotalSupply,
			TokenStandard: detection.Standard,
			HasERC165:     detection.HasERC165,
			ContractCode:  code,
			UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		}

		// If we have existing contract data, preserve the creation information
		if err == nil && existingContract != nil {
			// Preserve creation information if present
			if existingContract.CreatorAddress != "" {
				contract.CreatorAddress = existingContract.CreatorAddress
			}
			if existingContract.CreationTransaction != "" {
				contract.CreationTransaction = existingContract.CreationTransaction
			}
			if existingContract.CreationBlockNumber != "" {
				contract.CreationBlockNumber = existingContract.CreationBlockNumber
			}
		}

		err = StoreContract(contract)
		if err != nil {
			configs.Logger.Error("Failed to store detected contract",
				zap.String("address", address),
				zap.Error(err))
		}
	}

	return isContract
}

// getContractFromDB retrieves contract information from the contractCode collection
// Local version to avoid naming conflicts
func getContractFromDB(address string) *models.ContractInfo {
	// First check in the main contracts collection
	mainContract, err := GetContract(address)
	if err == nil && mainContract != nil {
		// If found in main collection, return it
		return mainContract
	}

	// If not found in main collection, check the contractCode collection
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = collection.FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil
	}
	return &contract
}
