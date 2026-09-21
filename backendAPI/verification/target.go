package verification

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"backendAPI/db"
	"backendAPI/models"
)

var (
	ErrVerificationTargetIncomplete = errors.New("verification target identity is incomplete")
	ErrVerificationTargetChanged    = errors.New("verification target changed")
)

// CaptureVerificationTarget binds a job to the exact indexed deployment and
// checks that the connected node currently agrees on chain, creation block,
// and runtime code before the expensive compilation starts.
func CaptureVerificationTarget(ctx context.Context, address string) (models.VerificationTarget, error) {
	contract, err := db.ReturnContractCode(address)
	if err != nil {
		return models.VerificationTarget{}, err
	}
	if contract.ContractAddress == "" {
		return models.VerificationTarget{}, fmt.Errorf("%w: contract is not indexed", ErrVerificationTargetIncomplete)
	}

	canonicalBlockNumber := canonicalHexQuantity(contract.CreationBlockNumber)
	canonicalChainID := canonicalHexQuantity(contract.ChainID)
	canonicalAddress := canonicalQAddress(contract.ContractAddress)
	canonicalCreationTransaction := strings.ToLower(contract.CreationTransaction)
	target := models.VerificationTarget{
		Address:             canonicalAddress,
		CreationTransaction: canonicalCreationTransaction,
		CreationBlockNumber: canonicalBlockNumber,
		CreationBlockHash:   strings.ToLower(contract.CreationBlockHash),
		ChainID:             canonicalChainID,
		DeployedCodeSHA256:  strings.ToLower(contract.ContractCodeSHA256),
		GenesisContract:     contract.GenesisContract,
	}
	if !sameCanonicalAddress(target.Address, address) {
		return models.VerificationTarget{}, fmt.Errorf("%w: indexed address mismatch", ErrVerificationTargetChanged)
	}
	if err := validateVerificationTarget(target); err != nil {
		return models.VerificationTarget{}, err
	}
	if contract.ContractAddress != canonicalAddress ||
		contract.CreationTransaction != canonicalCreationTransaction ||
		strings.ToLower(contract.CreationBlockNumber) != canonicalBlockNumber ||
		strings.ToLower(contract.ChainID) != canonicalChainID {
		return models.VerificationTarget{}, fmt.Errorf("%w: indexed numeric identity is not canonical", ErrVerificationTargetIncomplete)
	}

	indexedDigest, err := RuntimeCodeSHA256(contract.ContractCode)
	if err != nil {
		return models.VerificationTarget{}, fmt.Errorf("%w: indexed runtime code: %v", ErrVerificationTargetIncomplete, err)
	}
	if indexedDigest != target.DeployedCodeSHA256 {
		return models.VerificationTarget{}, fmt.Errorf("%w: indexed runtime-code digest mismatch", ErrVerificationTargetChanged)
	}
	if err := RevalidateVerificationTarget(ctx, target); err != nil {
		return models.VerificationTarget{}, err
	}
	return target, nil
}

// RevalidateVerificationTarget checks node-backed state immediately before a
// verification result is committed. The database commit independently checks
// the indexed row and canonical block under one Mongo transaction.
func RevalidateVerificationTarget(ctx context.Context, target models.VerificationTarget) error {
	if err := validateVerificationTarget(target); err != nil {
		return err
	}

	chainID, err := fetchChainID(ctx)
	if err != nil {
		return fmt.Errorf("read live chain ID: %w", err)
	}
	if chainID != target.ChainID {
		return fmt.Errorf("%w: chain ID changed", ErrVerificationTargetChanged)
	}

	blockHash, err := fetchBlockHash(ctx, target.CreationBlockNumber)
	if err != nil {
		return fmt.Errorf("read live creation block: %w", err)
	}
	if blockHash != target.CreationBlockHash {
		return fmt.Errorf("%w: creation block changed", ErrVerificationTargetChanged)
	}

	code, err := FetchOnChainCode(ctx, target.Address)
	if err != nil {
		return fmt.Errorf("read live runtime code: %w", err)
	}
	digest, err := RuntimeCodeSHA256(code)
	if err != nil {
		return fmt.Errorf("%w: live runtime code is absent or malformed", ErrVerificationTargetChanged)
	}
	if digest != target.DeployedCodeSHA256 {
		return fmt.Errorf("%w: deployed runtime code changed", ErrVerificationTargetChanged)
	}
	return nil
}

func validateVerificationTarget(target models.VerificationTarget) error {
	if target.Address == "" || target.CreationBlockNumber == "" ||
		target.CreationBlockHash == "" || target.ChainID == "" ||
		target.DeployedCodeSHA256 == "" {
		return ErrVerificationTargetIncomplete
	}
	if !target.GenesisContract && target.CreationTransaction == "" {
		return fmt.Errorf("%w: creation transaction is missing", ErrVerificationTargetIncomplete)
	}
	addressHex := strings.TrimPrefix(target.Address, "Q")
	blockHashHex := strings.TrimPrefix(target.CreationBlockHash, "0x")
	if canonicalHexQuantity(target.CreationBlockNumber) != target.CreationBlockNumber ||
		canonicalHexQuantity(target.ChainID) != target.ChainID ||
		canonicalQAddress(target.Address) != target.Address ||
		len(addressHex) != 128 ||
		!strings.HasPrefix(target.CreationBlockHash, "0x") ||
		target.CreationBlockHash != strings.ToLower(target.CreationBlockHash) ||
		len(blockHashHex) != 64 ||
		target.DeployedCodeSHA256 != strings.ToLower(target.DeployedCodeSHA256) ||
		len(target.DeployedCodeSHA256) != 64 {
		return fmt.Errorf("%w: malformed canonical identity", ErrVerificationTargetIncomplete)
	}
	if _, err := hex.DecodeString(blockHashHex); err != nil {
		return fmt.Errorf("%w: malformed creation block hash", ErrVerificationTargetIncomplete)
	}
	if _, err := hex.DecodeString(target.DeployedCodeSHA256); err != nil {
		return fmt.Errorf("%w: malformed runtime-code digest", ErrVerificationTargetIncomplete)
	}
	if _, err := hex.DecodeString(addressHex); err != nil {
		return fmt.Errorf("%w: malformed contract address", ErrVerificationTargetIncomplete)
	}
	if !target.GenesisContract {
		creationTransactionHex := strings.TrimPrefix(target.CreationTransaction, "0x")
		if !strings.HasPrefix(target.CreationTransaction, "0x") ||
			len(creationTransactionHex) != 64 ||
			target.CreationTransaction != strings.ToLower(target.CreationTransaction) {
			return fmt.Errorf("%w: malformed creation transaction", ErrVerificationTargetIncomplete)
		}
		if _, err := hex.DecodeString(creationTransactionHex); err != nil {
			return fmt.Errorf("%w: malformed creation transaction", ErrVerificationTargetIncomplete)
		}
	}
	return nil
}

func fetchChainID(ctx context.Context) (string, error) {
	raw, rpcErr, err := db.NodeRPC(ctx, "qrl_chainId", []interface{}{})
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", rpcErr
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	canonical := canonicalHexQuantity(value)
	if canonical == "" {
		return "", errors.New("node returned malformed chain ID")
	}
	return canonical, nil
}

func fetchBlockHash(ctx context.Context, blockNumber string) (string, error) {
	raw, rpcErr, err := db.NodeRPC(ctx, "qrl_getBlockByNumber", []interface{}{blockNumber, false})
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", rpcErr
	}
	var block struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return "", err
	}
	hash := strings.ToLower(block.Hash)
	if len(strings.TrimPrefix(hash, "0x")) != 64 {
		return "", errors.New("node returned malformed block hash")
	}
	return hash, nil
}

func canonicalHexQuantity(value string) string {
	text := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
	if text == "" {
		return ""
	}
	n := new(big.Int)
	if _, ok := n.SetString(text, 16); !ok || n.Sign() < 0 || n.BitLen() > 256 {
		return ""
	}
	return "0x" + n.Text(16)
}

func sameCanonicalAddress(left, right string) bool {
	return canonicalQAddress(left) == canonicalQAddress(right)
}

func canonicalQAddress(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "Q") || strings.HasPrefix(value, "q") {
		value = value[1:]
	} else {
		value = strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	}
	return "Q" + strings.ToLower(value)
}
