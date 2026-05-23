package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"time"

	"go.uber.org/zap"
)

// preserveCreationInfo copies creation metadata from an existing contract to a new contract info.
// This ensures we don't lose historical data like creator address and creation transaction.
func preserveCreationInfo(contractInfo *models.ContractInfo, existingContract *models.ContractInfo) {
	if existingContract == nil {
		return
	}
	contractInfo.CreatorAddress = existingContract.CreatorAddress
	contractInfo.CreationTransaction = existingContract.CreationTransaction
	contractInfo.CreationBlockNumber = existingContract.CreationBlockNumber
	contractInfo.ContractCode = existingContract.ContractCode
}

// EnsureContractClassified looks the contract up via DetectContractType and
// persists the (possibly updated) classification.
//
// Returns the merged ContractInfo and the detected standard string
// ("ERC-20"/"ERC-721"/"ERC-1155" or "" for unclassified). Callers should
// check `standard == ""` to skip non-token logs; the *ContractInfo will still
// be populated when an existing contract record was preserved.
//
// On transient RPC failure DetectContractType returns an error and we return
// (existing, ""), never demoting a previously-classified contract. The
// caller's "skip non-token" guard correctly skips this log; the next sighting
// re-tries and either succeeds (promotion) or stays unclassified (no harm).
func EnsureContractClassified(contractAddress string, blockNumber string, txHash string) (*models.ContractInfo, string) {
	detection, detErr := rpc.DetectContractType(contractAddress)
	if detErr != nil {
		// Transient probe failure, preserve existing state, don't write
		// anything that would clobber a previously-good classification.
		configs.Logger.Debug("Contract type detection failed; preserving existing classification",
			zap.String("address", contractAddress),
			zap.Error(detErr))
		existing, _ := GetContract(contractAddress)
		if existing != nil && existing.TokenStandard != "" {
			return existing, existing.TokenStandard
		}
		return existing, ""
	}
	if detection.Standard == "" {
		// Not a recognised standard; not necessarily an error. Don't churn
		// the DB on every log emission from such a contract.
		configs.Logger.Debug("Contract is not a recognised token standard",
			zap.String("address", contractAddress))
		return nil, ""
	}

	configs.Logger.Debug("Contract classified",
		zap.String("address", contractAddress),
		zap.String("standard", detection.Standard),
		zap.String("name", detection.Name),
		zap.String("symbol", detection.Symbol))

	existingContract, err := GetContract(contractAddress)
	if err != nil {
		configs.Logger.Debug("GetContract returned error (treating as fresh)",
			zap.String("address", contractAddress),
			zap.Error(err))
	}

	contractInfo := models.ContractInfo{
		Address:       contractAddress,
		Status:        "0x1",
		IsToken:       true,
		Name:          detection.Name,
		Symbol:        detection.Symbol,
		Decimals:      detection.Decimals,
		TotalSupply:   detection.TotalSupply,
		TokenStandard: detection.Standard,
		HasERC165:     detection.HasERC165,
		MetadataURI:   detection.MetadataURI,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if existingContract == nil {
		// First sighting, try to backfill creator info from the transfer
		// collection now (cheap DB lookup) rather than waiting for the
		// hourly reprocess job.
		contractInfo.CreationBlockNumber = blockNumber
		if creationTx := findCreationTransaction(contractAddress); creationTx != nil {
			contractInfo.CreationTransaction = creationTx.TxHash
			contractInfo.CreationBlockNumber = creationTx.BlockNumber
			if creationTx.From != "" && creationTx.From != "Q" {
				contractInfo.CreatorAddress = creationTx.From
			}
		}
	} else {
		preserveCreationInfo(&contractInfo, existingContract)
	}

	if err := StoreContract(contractInfo); err != nil {
		configs.Logger.Error("Failed to store/update classified contract",
			zap.String("address", contractAddress),
			zap.Error(err))
		return nil, ""
	}

	return &contractInfo, detection.Standard
}

// GetTokenFromDatabase retrieves token info from database and verifies it's a token.
// Returns nil if not found or not a token.
func GetTokenFromDatabase(contractAddress string) *models.ContractInfo {
	contract, err := GetContract(contractAddress)
	if err != nil {
		return nil
	}
	if !contract.IsToken {
		return nil
	}
	return contract
}
