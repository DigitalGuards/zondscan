package rpc

import (
	"QRL2MongoDB/models"
	"fmt"
	"math/big"
	"strings"

	"QRL2MongoDB/validation"
)

// Token-transfer event signatures.
//
// ERC-20 and ERC-721 share the same Transfer(address,address,uint256) hash;
// disambiguation is by topic count (3 vs 4) and confirmed against the
// contract's ERC-165 declaration in DetectContractType.
const (
	logTopicHashPadding          = "0000000000000000000000000000000000000000000000000000000000000000"
	TransferEventSignature       = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" + logTopicHashPadding
	TransferSingleEventSignature = "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62" + logTopicHashPadding
	// keccak256("TransferBatch(address,address,address,uint256[],uint256[])").
	// The earlier value (…b50af327f5f6b67…) was a typo, so qrl_getLogs filtered
	// by it never returned TransferBatch logs and the explorer silently dropped
	// every ERC-1155 batch mint / batch transfer.
	TransferBatchEventSignature = "0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb" + logTopicHashPadding
)

// ProcessTransferLogs processes ERC-20 Transfer events emitted by one exact
// contract. The receipt may contain logs from nested calls into other token
// contracts, so emitter binding is part of parsing rather than a caller-side
// convention. A malformed matching event fails the whole parse before callers
// persist any rows.
func ProcessTransferLogs(receipt *models.TransactionReceipt, emitter string) ([]TransferEvent, error) {
	if receipt == nil {
		return nil, fmt.Errorf("transaction receipt is nil")
	}
	emitter = validation.ConvertToQAddress(emitter)
	if !validation.IsValidAddress(emitter) {
		return nil, fmt.Errorf("invalid transfer emitter: %s", emitter)
	}

	var transfers []TransferEvent

	for index, log := range receipt.Result.Logs {
		if len(log.Topics) == 0 || !strings.EqualFold(log.Topics[0], TransferEventSignature) {
			continue
		}

		logEmitter := validation.ConvertToQAddress(log.Address)
		if !validation.IsValidAddress(logEmitter) {
			return nil, fmt.Errorf("transfer log %d has invalid emitter: %s", index, log.Address)
		}
		if logEmitter != emitter {
			continue
		}
		if log.Removed {
			return nil, fmt.Errorf("transfer log %d is marked removed", index)
		}
		if len(log.Topics) != 3 {
			return nil, fmt.Errorf("ERC-20 transfer log %d requires 3 topics, got %d", index, len(log.Topics))
		}
		if !validation.IsValidHexString(log.LogIndex) {
			return nil, fmt.Errorf("transfer log %d has invalid log index: %s", index, log.LogIndex)
		}

		from, to, amount, err := ParseTransferEvent(log)
		if err != nil {
			return nil, fmt.Errorf("parse transfer log %d: %w", index, err)
		}

		transfers = append(transfers, TransferEvent{
			From:     from,
			To:       to,
			Amount:   amount.String(),
			LogIndex: strings.ToLower(log.LogIndex),
		})
	}

	return transfers, nil
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

	data := strings.TrimPrefix(log.Data, "0x")
	if len(data) != abiWordHexLength {
		return "", "", nil, fmt.Errorf("transfer amount has %d hex chars, want %d", len(data), abiWordHexLength)
	}
	amount, err := parseUint256HexWord(data)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to parse amount from data: %w", err)
	}

	return from, to, amount, nil
}

// addressFromTopic extracts a native 64-byte address from a complete 64-byte
// indexed topic. QIP-55 addresses occupy the full word without padding.
func addressFromTopic(topic string) (string, error) {
	stripped := strings.TrimPrefix(topic, "0x")
	if len(stripped) != abiWordHexLength {
		return "", fmt.Errorf("topic has %d hex chars, want %d: %s", len(stripped), abiWordHexLength, topic)
	}
	addr := "Q" + strings.ToLower(stripped)
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
	if !strings.EqualFold(log.Data, "0x") {
		return "", "", nil, fmt.Errorf("ERC-721 Transfer data must be empty, got %d hex chars",
			len(strings.TrimPrefix(log.Data, "0x")))
	}
	from, err = addressFromTopic(log.Topics[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("from: %w", err)
	}
	to, err = addressFromTopic(log.Topics[2])
	if err != nil {
		return "", "", nil, fmt.Errorf("to: %w", err)
	}
	id, err := parseUint256HexWord(strings.TrimPrefix(log.Topics[3], "0x"))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to parse tokenID from topic[3]: %w", err)
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
	if len(data) != 2*abiWordHexLength {
		return "", "", nil, nil, fmt.Errorf("TransferSingle data has %d hex chars, want %d",
			len(data), 2*abiWordHexLength)
	}
	id, err = parseUint256HexWord(data[:abiWordHexLength])
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to parse id from data: %w", err)
	}
	value, err = parseUint256HexWord(data[abiWordHexLength : 2*abiWordHexLength])
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to parse value from data: %w", err)
	}
	return from, to, id, value, nil
}

// ParseERC1155TransferBatch decodes an ERC-1155 TransferBatch(operator, from, to, ids[], values[]) log.
//
// Layout: 4 topics [sig, operator, from, to], data = abi.encode(uint256[], uint256[]).
// The dynamic-array encoding starts with two 64-byte offsets pointing to
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
	if len(data) < 2*abiWordHexLength {
		return "", "", nil, nil, fmt.Errorf("TransferBatch data too short: %d hex chars", len(data))
	}

	idsOffsetBytes, err := readUint64FromWord(data, 0)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("ids offset: %w", err)
	}
	valuesOffsetBytes, err := readUint64FromWord(data, abiWordHexLength)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("values offset: %w", err)
	}

	dataHexLength := uint64(len(data))
	minimumArrayOffset := uint64(2 * abiWordBytes)
	if idsOffsetBytes != minimumArrayOffset {
		return "", "", nil, nil, fmt.Errorf("ids offset %d is not canonical, want %d",
			idsOffsetBytes, minimumArrayOffset)
	}
	if valuesOffsetBytes < minimumArrayOffset || valuesOffsetBytes%uint64(abiWordBytes) != 0 {
		return "", "", nil, nil, fmt.Errorf("values offset %d is not canonical", valuesOffsetBytes)
	}
	if idsOffsetBytes > dataHexLength/2 {
		return "", "", nil, nil, fmt.Errorf("ids offset %d exceeds data length", idsOffsetBytes)
	}
	if valuesOffsetBytes > dataHexLength/2 {
		return "", "", nil, nil, fmt.Errorf("values offset %d exceeds data length", valuesOffsetBytes)
	}
	idsOffsetHex := idsOffsetBytes * 2
	valuesOffsetHex := valuesOffsetBytes * 2

	ids, err = decodeUint256Array(data, idsOffsetHex)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("ids array: %w", err)
	}
	expectedValuesOffset, err := encodedUint256ArrayEndBytes(idsOffsetBytes, uint64(len(ids)))
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("ids array end: %w", err)
	}
	if valuesOffsetBytes != expectedValuesOffset {
		return "", "", nil, nil, fmt.Errorf("values offset %d is not canonical, want %d",
			valuesOffsetBytes, expectedValuesOffset)
	}
	values, err = decodeUint256Array(data, valuesOffsetHex)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("values array: %w", err)
	}
	if len(ids) != len(values) {
		return "", "", nil, nil, fmt.Errorf("ids/values length mismatch: %d vs %d", len(ids), len(values))
	}
	expectedDataBytes, err := encodedUint256ArrayEndBytes(valuesOffsetBytes, uint64(len(values)))
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("values array end: %w", err)
	}
	expectedDataHexLength, err := checkedUint64Multiply(expectedDataBytes, 2)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("values array encoded length: %w", err)
	}
	if dataHexLength != expectedDataHexLength {
		return "", "", nil, nil, fmt.Errorf("TransferBatch data has %d bytes, canonical encoding requires %d",
			dataHexLength/2, expectedDataBytes)
	}
	return from, to, ids, values, nil
}
