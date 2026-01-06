package db

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"Zond2mongoDB/rpc"
	"time"

	"go.uber.org/zap"
)

// TokenDetectionResult holds the result of token detection
type TokenDetectionResult struct {
	IsToken     bool
	Name        string
	Symbol      string
	Decimals    uint8
	TotalSupply string
}

// DetectToken checks if a contract address is a valid ERC20 token
// by calling the standard ERC20 methods (name, symbol, decimals)
func DetectToken(contractAddress string) TokenDetectionResult {
	name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)
	if !isToken {
		return TokenDetectionResult{IsToken: false}
	}

	// Get total supply if it's a token
	totalSupply, err := rpc.GetTokenTotalSupply(contractAddress)
	if err != nil {
		configs.Logger.Debug("Failed to get token total supply",
			zap.String("address", contractAddress),
			zap.Error(err))
	}

	return TokenDetectionResult{
		IsToken:     true,
		Name:        name,
		Symbol:      symbol,
		Decimals:    decimals,
		TotalSupply: totalSupply,
	}
}

// EnsureTokenInDatabase ensures a token contract exists in the database with up-to-date info.
// If the contract already exists, it preserves existing creation information.
// Returns the contract info and whether it's a token.
func EnsureTokenInDatabase(contractAddress string, blockNumber string, txHash string) (*models.ContractInfo, bool) {
	// First check via RPC if this is actually a token
	detection := DetectToken(contractAddress)
	if !detection.IsToken {
		configs.Logger.Debug("RPC check indicates contract is not a token",
			zap.String("address", contractAddress))
		return nil, false
	}

	configs.Logger.Debug("RPC check confirms contract is a token",
		zap.String("address", contractAddress),
		zap.String("name", detection.Name),
		zap.String("symbol", detection.Symbol))

	// Try to get existing contract from DB to preserve creation information
	existingContract, err := GetContract(contractAddress)
	if err != nil {
		// Log unexpected errors (not just "not found")
		configs.Logger.Debug("GetContract returned error",
			zap.String("address", contractAddress),
			zap.Error(err))
	}

	// Build the contract info
	contractInfo := models.ContractInfo{
		Address:   contractAddress,
		Status:    "0x1", // Assume successful
		IsToken:   true,
		Name:      detection.Name,
		Symbol:    detection.Symbol,
		Decimals:  detection.Decimals,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if detection.TotalSupply != "" {
		contractInfo.TotalSupply = detection.TotalSupply
	}

	// If this is the first time we're seeing this token, record discovery info
	if existingContract == nil {
		contractInfo.CreationBlockNumber = blockNumber
		contractInfo.CreationTransaction = txHash

		// Try to get creator from transaction
		txDetails, txErr := rpc.GetTxDetailsByHash(txHash)
		if txErr != nil {
			configs.Logger.Debug("Failed to get transaction details for token creator",
				zap.String("txHash", txHash),
				zap.Error(txErr))
		} else if txDetails != nil {
			contractInfo.CreatorAddress = txDetails.From
		}
	} else {
		// Preserve existing creation information directly
		contractInfo.CreatorAddress = existingContract.CreatorAddress
		contractInfo.CreationTransaction = existingContract.CreationTransaction
		contractInfo.CreationBlockNumber = existingContract.CreationBlockNumber
		contractInfo.ContractCode = existingContract.ContractCode
	}

	// Store/merge the contract info
	if err := StoreContract(contractInfo); err != nil {
		configs.Logger.Error("Failed to store/update token contract",
			zap.String("address", contractAddress),
			zap.Error(err))
		return nil, false
	}

	return &contractInfo, true
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

// RefreshTokenMetadata updates the token metadata (name, symbol, decimals, totalSupply)
// from the blockchain while preserving creation information.
func RefreshTokenMetadata(contractAddress string) error {
	detection := DetectToken(contractAddress)
	if !detection.IsToken {
		configs.Logger.Debug("Contract is not a token, skipping refresh",
			zap.String("address", contractAddress))
		return nil
	}

	// Get existing contract to preserve creation info
	existingContract, _ := GetContract(contractAddress)

	contractInfo := models.ContractInfo{
		Address:     contractAddress,
		IsToken:     true,
		Name:        detection.Name,
		Symbol:      detection.Symbol,
		Decimals:    detection.Decimals,
		TotalSupply: detection.TotalSupply,
		Status:      "0x1",
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// Preserve creation info if it exists
	if existingContract != nil {
		contractInfo.CreatorAddress = existingContract.CreatorAddress
		contractInfo.CreationTransaction = existingContract.CreationTransaction
		contractInfo.CreationBlockNumber = existingContract.CreationBlockNumber
		contractInfo.ContractCode = existingContract.ContractCode
	}

	return StoreContract(contractInfo)
}
