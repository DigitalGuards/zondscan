package rpc

import (
	"QRL2MongoDB/models"
	"fmt"
	"math/big"
	"strings"

	"QRL2MongoDB/validation"

	"go.uber.org/zap"
)

// Token-transfer event signatures.
//
// ERC-20 and ERC-721 share the same Transfer(address,address,uint256) hash;
// disambiguation is by topic count (3 vs 4) and confirmed against the
// contract's ERC-165 declaration in DetectContractType.
const (
	TransferEventSignature       = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	TransferSingleEventSignature = "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62"
	// keccak256("TransferBatch(address,address,address,uint256[],uint256[])").
	// The earlier value (…b50af327f5f6b67…) was a typo, so qrl_getLogs filtered
	// by it never returned TransferBatch logs and the explorer silently dropped
	// every ERC-1155 batch mint / batch transfer.
	TransferBatchEventSignature = "0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb"
)

// DecodeTransferEvent decodes token transfers from both:
// 1. Direct transfer calls (tx.data starting with 0xa9059cbb)
// 2. Transfer events in transaction logs
func DecodeTransferEvent(data string) (string, string, string) {
	// First try to decode direct transfer call
	if len(data) >= 10 && data[:10] == "0xa9059cbb" {
		if len(data) != 138 { // 2 (0x) + 8 (func) + 64 (to) + 64 (amount) = 138
			return "", "", ""
		}

		// Extract recipient address, canonical Q-prefix form
		recipient := "Q" + strings.ToLower(data[34:74])
		if len(recipient) != 41 { // Check if it's a valid address length (Z + 40 hex chars)
			return "", "", ""
		}

		// Extract amount
		amount := "0x" + data[74:]
		return "", recipient, amount
	}

	return "", "", ""
}

// ProcessTransferLogs processes Transfer events from transaction logs.
// LogIndex on the returned events is the originating log's index inside
// the receipt; callers persist it so multi-transfer txs (a swap that
// emits 3 Transfer events) don't dedup-collide on tx hash alone.
func ProcessTransferLogs(receipt *models.TransactionReceipt) []TransferEvent {
	var transfers []TransferEvent

	for _, log := range receipt.Result.Logs {
		// Check if this is a Transfer event
		if len(log.Topics) == 3 && log.Topics[0] == TransferEventSignature {
			from, to, amount, err := ParseTransferEvent(log)
			if err != nil {
				// Log the error but continue processing other logs
				zap.L().Error("Failed to parse transfer event", zap.Error(err))
				continue
			}

			transfers = append(transfers, TransferEvent{
				From:     from,
				To:       to,
				Amount:   amount.String(),
				LogIndex: log.LogIndex,
			})
		}
	}

	return transfers
}

type TransferEvent struct {
	From     string
	To       string
	Amount   string
	LogIndex string
}

// ParseTransferEvent parses a transfer event log.
// Addresses are returned in canonical Q-prefix form.
func ParseTransferEvent(log models.Log) (string, string, *big.Int, error) {
	// Extract addresses from topics via the length-validating helper.
	if len(log.Topics) < 3 {
		return "", "", nil, fmt.Errorf("transfer event requires 3 topics, got %d", len(log.Topics))
	}
	from, err := addressFromTopic(log.Topics[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("from: %w", err)
	}
	to, err := addressFromTopic(log.Topics[2])
	if err != nil {
		return "", "", nil, fmt.Errorf("to: %w", err)
	}

	// Parse amount from data field. big.Int.SetString rejects the "0x"
	// prefix at base 16, so strip it first or amount silently stays 0.
	amount := new(big.Int)
	if len(log.Data) > 2 {
		data := strings.TrimPrefix(log.Data, "0x")
		if _, success := amount.SetString(data, 16); !success {
			return "", "", nil, fmt.Errorf("failed to parse amount from data: %s", log.Data)
		}
	}

	return from, to, amount, nil
}

// addressFromTopic extracts a 20-byte address from a 32-byte indexed topic.
// The topic is the standard "0x" + 64 hex chars; the address is the last
// 40 hex chars. Returns canonical Q-prefix lowercase form.
func addressFromTopic(topic string) (string, error) {
	stripped := strings.TrimPrefix(topic, "0x")
	if len(stripped) < 40 {
		return "", fmt.Errorf("topic too short: %s", topic)
	}
	addr := "Q" + strings.ToLower(stripped[len(stripped)-40:])
	if !validation.IsValidAddress(addr) {
		return "", fmt.Errorf("invalid address derived from topic: %s", addr)
	}
	return addr, nil
}

// ParseERC721Transfer decodes an ERC-721 Transfer(from, to, tokenId) log.
//
// Layout: 4 topics [sig, from, to, tokenID], empty data.
// `tokenID` is the indexed parameter in topic[3]; the data field is empty.
func ParseERC721Transfer(log models.Log) (from, to string, tokenID *big.Int, err error) {
	if len(log.Topics) != 4 {
		return "", "", nil, fmt.Errorf("ERC-721 Transfer requires 4 topics, got %d", len(log.Topics))
	}
	from, err = addressFromTopic(log.Topics[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("from: %w", err)
	}
	to, err = addressFromTopic(log.Topics[2])
	if err != nil {
		return "", "", nil, fmt.Errorf("to: %w", err)
	}
	id := new(big.Int)
	if _, ok := id.SetString(strings.TrimPrefix(log.Topics[3], "0x"), 16); !ok {
		return "", "", nil, fmt.Errorf("failed to parse tokenID from topic[3]: %s", log.Topics[3])
	}
	return from, to, id, nil
}

// ParseERC1155TransferSingle decodes an ERC-1155 TransferSingle(operator, from, to, id, value) log.
//
// Layout: 4 topics [sig, operator, from, to], data = abi.encode(uint256 id, uint256 value).
// The operator is dropped, callers care about the (from, to) pair, not who
// orchestrated the transfer.
func ParseERC1155TransferSingle(log models.Log) (from, to string, id, value *big.Int, err error) {
	if len(log.Topics) != 4 {
		return "", "", nil, nil, fmt.Errorf("ERC-1155 TransferSingle requires 4 topics, got %d", len(log.Topics))
	}
	from, err = addressFromTopic(log.Topics[2])
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("from: %w", err)
	}
	to, err = addressFromTopic(log.Topics[3])
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("to: %w", err)
	}
	data := strings.TrimPrefix(log.Data, "0x")
	if len(data) < 128 {
		return "", "", nil, nil, fmt.Errorf("TransferSingle data too short: %d hex chars", len(data))
	}
	id = new(big.Int)
	if _, ok := id.SetString(data[:64], 16); !ok {
		return "", "", nil, nil, fmt.Errorf("failed to parse id from data")
	}
	value = new(big.Int)
	if _, ok := value.SetString(data[64:128], 16); !ok {
		return "", "", nil, nil, fmt.Errorf("failed to parse value from data")
	}
	return from, to, id, value, nil
}

// ParseERC1155TransferBatch decodes an ERC-1155 TransferBatch(operator, from, to, ids[], values[]) log.
//
// Layout: 4 topics [sig, operator, from, to], data = abi.encode(uint256[], uint256[]).
// The dynamic-array encoding starts with two 32-byte offsets pointing to
// each array's `length || elements...` section. Both arrays must have the
// same length per the ERC-1155 spec.
func ParseERC1155TransferBatch(log models.Log) (from, to string, ids, values []*big.Int, err error) {
	if len(log.Topics) != 4 {
		return "", "", nil, nil, fmt.Errorf("ERC-1155 TransferBatch requires 4 topics, got %d", len(log.Topics))
	}
	from, err = addressFromTopic(log.Topics[2])
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("from: %w", err)
	}
	to, err = addressFromTopic(log.Topics[3])
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("to: %w", err)
	}

	data := strings.TrimPrefix(log.Data, "0x")
	if len(data) < 128 {
		return "", "", nil, nil, fmt.Errorf("TransferBatch data too short: %d hex chars", len(data))
	}

	idsOffsetBytes, err := readUint64FromWord(data, 0)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("ids offset: %w", err)
	}
	valuesOffsetBytes, err := readUint64FromWord(data, 64)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("values offset: %w", err)
	}

	ids, err = decodeUint256Array(data, idsOffsetBytes*2)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("ids array: %w", err)
	}
	values, err = decodeUint256Array(data, valuesOffsetBytes*2)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("values array: %w", err)
	}
	if len(ids) != len(values) {
		return "", "", nil, nil, fmt.Errorf("ids/values length mismatch: %d vs %d", len(ids), len(values))
	}
	return from, to, ids, values, nil
}
