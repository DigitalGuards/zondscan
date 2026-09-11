package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// processContracts processes contract-related information from a transaction.
func processContracts(tx *models.Transaction) (string, string, string, bool, error) {
	var to string
	var contractAddress string
	var statusTx string
	var isContract bool
	var errs []error

	// Check if it's a contract creation transaction
	if tx.To == "" {
		// Get contract address and status from transaction receipt
		var err error
		contractAddress, statusTx, err = rpc.GetContractAddress(tx.Hash)
		if err != nil {
			configs.Logger.Error("Failed to get contract address",
				zap.String("hash", tx.Hash),
				zap.Error(err))
			return "", "", "", false, fmt.Errorf("get contract address for %s: %w", tx.Hash, err)
		}

		if contractAddress != "" {
			isContract = true

			// Get contract code
			contractCode, err := rpc.GetCode(contractAddress, "latest")
			if err != nil {
				configs.Logger.Error("Failed to get contract code",
					zap.String("address", contractAddress),
					zap.Error(err))
				errs = append(errs, fmt.Errorf("get contract code for %s: %w", contractAddress, err))
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
				errs = append(errs, fmt.Errorf("detect contract type for %s: %w", contractAddress, detErr))
			}

			// Store complete contract information
			contract := models.ContractInfo{
				Address:                  contractAddress,
				Status:                   statusTx,
				IsToken:                  detection.Standard != "",
				Name:                     detection.Name,
				Symbol:                   detection.Symbol,
				Decimals:                 detection.Decimals,
				TotalSupply:              detection.TotalSupply,
				TokenStandard:            detection.Standard,
				HasERC165:                detection.HasERC165,
				ContractCode:             contractCode,
				CreatorAddress:           tx.From,
				CreatorAddressProvenance: models.CreatorAddressProvenanceDirectDeployment,
				CreationTransaction:      tx.Hash,
				CreationBlockNumber:      tx.BlockNumber,
				CreationBlockHash:        tx.BlockHash,
				ChainID:                  tx.ChainID,
				UpdatedAt:                time.Now().UTC().Format(time.RFC3339),
			}

			// Store the contract
			err = StoreContract(contract)
			if err != nil {
				configs.Logger.Error("Failed to store contract",
					zap.String("address", contractAddress),
					zap.Error(err))
				errs = append(errs, fmt.Errorf("store contract %s: %w", contractAddress, err))
			}
		}
	} else {
		to = tx.To
		statusTx = tx.Status

		// Check if the destination address is a contract
		var err error
		isContract, err = IsAddressContract(to)
		if err != nil {
			errs = append(errs, fmt.Errorf("classify destination %s: %w", to, err))
		}
	}

	return to, contractAddress, statusTx, isContract, errors.Join(errs...)
}

// storeInternalContractCreations registers contracts created by nested
// CREATE/CREATE2 frames of a traced transaction. The creation branch in
// processContracts only sees top-level deployments (tx.To == ""), so without
// this sync-time hook a factory-created contract gets no contractCode row
// until a later interaction or reprocess pass stumbles on it. Mirrors the
// processContracts store flow (code fetch + classification + StoreContract),
// with the outer transaction's sender/hash/block as the creation metadata.
// The calls slice is empty unless debug tracing is enabled
// (ENABLE_DEBUG_TRACE), so this is a no-op on nodes without the debug_ API.
func storeInternalContractCreations(
	calls []rpc.InternalCall,
	creator string,
	txHash string,
	blockNumber string,
	blockHash string,
	chainID string,
	statusTx string,
) error {
	var errs []error
	for _, call := range calls {
		if !strings.HasPrefix(call.Type, "CREATE") || call.To == "" {
			continue
		}

		contractCode, err := rpc.GetCode(call.To, "latest")
		if err != nil {
			configs.Logger.Error("Failed to get contract code",
				zap.String("address", call.To),
				zap.Error(err))
			errs = append(errs, fmt.Errorf("get internal contract code for %s: %w", call.To, err))
		}

		// Classify the contract (ERC-20 / ERC-721 / ERC-1155 / unknown).
		// On transient probe failure the result is zero-valued; the
		// StoreContract merge's promote-only invariant prevents that
		// from clobbering a previously-good classification.
		detection, detErr := rpc.DetectContractType(call.To)
		if detErr != nil {
			configs.Logger.Warn("Contract type detection failed; storing without classification",
				zap.String("address", call.To),
				zap.Error(detErr))
			errs = append(errs, fmt.Errorf("detect internal contract type for %s: %w", call.To, detErr))
		}

		contract := models.ContractInfo{
			Address:                  call.To,
			Status:                   statusTx,
			IsToken:                  detection.Standard != "",
			Name:                     detection.Name,
			Symbol:                   detection.Symbol,
			Decimals:                 detection.Decimals,
			TotalSupply:              detection.TotalSupply,
			TokenStandard:            detection.Standard,
			HasERC165:                detection.HasERC165,
			ContractCode:             contractCode,
			CreatorAddress:           creator,
			CreatorAddressProvenance: models.CreatorAddressProvenanceCreateTraceOuter,
			CreationTransaction:      txHash,
			CreationBlockNumber:      blockNumber,
			CreationBlockHash:        blockHash,
			ChainID:                  chainID,
			UpdatedAt:                time.Now().UTC().Format(time.RFC3339),
		}

		if err := StoreContract(contract); err != nil {
			configs.Logger.Error("Failed to store contract",
				zap.String("address", call.To),
				zap.Error(err))
			errs = append(errs, fmt.Errorf("store internal contract %s: %w", call.To, err))
		}
	}
	return errors.Join(errs...)
}

// IsAddressContract checks if an address is a contract and returns every
// lookup, probe, and persistence failure to the durable ingestion boundary.
func IsAddressContract(address string) (bool, error) {
	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)

	// First check our database
	contract, err := getContractFromDB(address)
	if err != nil {
		return false, err
	}
	if contract != nil {
		return true, nil
	}

	// If not in database, check via RPC
	code, err := rpc.GetCode(address, "latest")
	if err != nil {
		configs.Logger.Error("Failed to get code for address",
			zap.String("address", address),
			zap.Error(err))
		return false, err
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
			configs.Logger.Warn("Contract type detection failed",
				zap.String("address", address),
				zap.Error(detErr))
			return false, detErr
		}

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

		err = StoreContract(contract)
		if err != nil {
			configs.Logger.Error("Failed to store detected contract",
				zap.String("address", address),
				zap.Error(err))
			return false, err
		}
	}

	return isContract, nil
}

// getContractFromDB retrieves contract information from the contractCode collection
// Local version to avoid naming conflicts
func getContractFromDB(address string) (*models.ContractInfo, error) {
	// First check in the main contracts collection
	mainContract, err := GetContract(address)
	if err == nil && mainContract != nil {
		// If found in main collection, return it
		return mainContract, nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// If not found in main collection, check the contractCode collection
	collection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = collection.FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contract, nil
}
