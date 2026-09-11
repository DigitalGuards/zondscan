package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// preserveCreationInfo copies creation metadata from an existing contract to a new contract info.
// This ensures we don't lose historical data like creator address and creation transaction.
func preserveCreationInfo(contractInfo *models.ContractInfo, existingContract *models.ContractInfo) {
	if existingContract == nil {
		return
	}
	contractInfo.CreatorAddress = existingContract.CreatorAddress
	contractInfo.CreatorAddressProvenance = existingContract.CreatorAddressProvenance
	contractInfo.CreationTransaction = existingContract.CreationTransaction
	contractInfo.CreationBlockNumber = existingContract.CreationBlockNumber
	contractInfo.CreationBlockHash = existingContract.CreationBlockHash
	contractInfo.ChainID = existingContract.ChainID
	contractInfo.ContractCode = existingContract.ContractCode
	contractInfo.ContractCodeSHA256 = existingContract.ContractCodeSHA256
	contractInfo.GenesisContract = existingContract.GenesisContract
}

type preparedContractClassification struct {
	contract *models.ContractInfo
	standard string
	persist  bool
}

// prepareContractClassification resolves a contract against the exact claimed
// block state and builds the row that will be persisted after every block log
// has decoded successfully.
func prepareContractClassification(
	contractAddress string,
	blockNumber string,
	blockHash string,
	txHash string,
) (preparedContractClassification, error) {
	existingContract, err := GetContract(contractAddress)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return preparedContractClassification{}, fmt.Errorf("load existing contract %s: %w", contractAddress, err)
	}

	detection, detErr := rpc.DetectContractTypeAtBlock(contractAddress, blockHash)
	if detErr != nil {
		return preparedContractClassification{}, fmt.Errorf(
			"detect contract type %s at block %s %s for transaction %s: %w",
			contractAddress,
			blockNumber,
			blockHash,
			txHash,
			detErr,
		)
	}
	if detection.Standard == "" {
		configs.Logger.Debug("Contract is not a recognised token standard at claimed block",
			zap.String("address", contractAddress),
			zap.String("blockNumber", blockNumber),
			zap.String("blockHash", blockHash))
		return preparedContractClassification{contract: existingContract}, nil
	}

	configs.Logger.Debug("Contract classified at claimed block",
		zap.String("address", contractAddress),
		zap.String("blockNumber", blockNumber),
		zap.String("blockHash", blockHash),
		zap.String("standard", detection.Standard),
		zap.String("name", detection.Name),
		zap.String("symbol", detection.Symbol))

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

	if existingContract != nil {
		preserveCreationInfo(&contractInfo, existingContract)
	}

	// Backfill creation info now (cheap DB lookup) rather than waiting for the
	// hourly reprocess job. Runs on first sighting AND when an existing record
	// still has empty creation fields (gap-fill, not blind preserve). Only the
	// authoritative sources are consulted here: the earliest-mint heuristic
	// and the genesis code probe are deliberately left to the reprocess pass,
	// which orders the probe first so a genesis-baked token cannot get a later
	// mint tx latched as its creation tx. The first-sighted log block is NOT
	// the creation block for a token first seen late, so nothing is guessed.
	if contractInfo.CreationTransaction == "" {
		creationTx, creationErr := findCreationTransaction(contractAddress)
		if creationErr != nil {
			return preparedContractClassification{}, fmt.Errorf(
				"look up authoritative contract creation evidence for %s: %w",
				contractAddress,
				creationErr,
			)
		} else if creationTx != nil {
			contractInfo.CreationTransaction = creationTx.TxHash
			contractInfo.CreationBlockNumber = creationTx.BlockNumber
			if creationTx.From != "" && creationTx.From != "Q" {
				contractInfo.CreatorAddress = creationTx.From
			}
			contractInfo.CreatorAddressProvenance = creationTx.Provenance
		}
	}

	return preparedContractClassification{
		contract: &contractInfo,
		standard: detection.Standard,
		persist:  true,
	}, nil
}

func persistPreparedContractClassification(classification preparedContractClassification) error {
	if !classification.persist {
		return nil
	}
	if classification.contract == nil {
		return fmt.Errorf("prepared contract classification has no metadata row")
	}
	// Historical replay establishes the standard at one exact block. Mutable
	// token metadata belongs to the current-head reprocessor: persisting an old
	// totalSupply (or pre-upgrade name/symbol/decimals/URI) would latch that
	// first historical value through StoreContract's gap-fill merge.
	persisted := *classification.contract
	persisted.Name = ""
	persisted.Symbol = ""
	persisted.Decimals = 0
	persisted.TotalSupply = ""
	persisted.MetadataURI = ""
	if err := StoreContract(persisted); err != nil {
		return fmt.Errorf("store classified contract %s: %w", classification.contract.Address, err)
	}
	return nil
}

// EnsureContractClassified resolves a contract against one exact block and
// immediately persists the classification. Block-wide ingestion uses the
// prepare/persist split directly so later malformed logs cannot observe an
// early classification side effect.
//
// Returns the merged ContractInfo and the detected standard string
// ("ERC-20"/"ERC-721"/"ERC-1155" or "" for unclassified). Callers should
// check `standard == ""` to skip non-token logs; the *ContractInfo will still
// be populated when an existing contract record was preserved.
//
// Every claimed block is probed at its own hash, including contracts with an
// existing classification, so proxy upgrades and later self-destruction do
// not rewrite historical event meaning. Existing rows provide provenance and
// creation metadata only. Transport, decode, and Mongo failures reach the
// durable block retry queue before token transfer effects begin.
func EnsureContractClassified(
	contractAddress string,
	blockNumber string,
	blockHash string,
	txHash string,
) (*models.ContractInfo, string, error) {
	classification, err := prepareContractClassification(
		contractAddress,
		blockNumber,
		blockHash,
		txHash,
	)
	if err != nil {
		return nil, "", err
	}
	if err := persistPreparedContractClassification(classification); err != nil {
		return nil, "", err
	}
	return classification.contract, classification.standard, nil
}
