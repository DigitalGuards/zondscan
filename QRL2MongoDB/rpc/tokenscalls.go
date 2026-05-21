package rpc

import (
	"QRL2MongoDB/models"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"QRL2MongoDB/validation"

	"go.uber.org/zap"
)

// Method signatures for ERC-20/721/1155 contract functions.
const (
	SIG_NAME               = "0x06fdde03" // name()
	SIG_SYMBOL             = "0x95d89b41" // symbol()
	SIG_DECIMALS           = "0x313ce567" // decimals()
	SIG_BALANCE            = "0x70a08231" // balanceOf(address)
	SIG_SUPPLY             = "0x18160ddd" // totalSupply()
	SIG_SUPPORTS_INTERFACE = "0x01ffc9a7" // supportsInterface(bytes4)
	SIG_OWNER_OF           = "0x6352211e" // ownerOf(uint256), ERC-721
	SIG_BALANCE_OF_1155    = "0x00fdd58e" // balanceOf(address,uint256), ERC-1155
	SIG_TOKEN_URI          = "0xc87b56dd" // tokenURI(uint256), ERC-721Metadata
	SIG_URI                = "0x0e89341c" // uri(uint256), ERC-1155MetadataURI
	SIG_CONTRACT_URI       = "0xe8a3d485" // contractURI(), OpenSea convention
)

// ERC-165 interface IDs, as the 4-byte XOR of the interface's method selectors.
var (
	InterfaceIDERC721          = [4]byte{0x80, 0xac, 0x58, 0xcd}
	InterfaceIDERC1155         = [4]byte{0xd9, 0xb6, 0x7a, 0x26}
	InterfaceIDERC721Metadata  = [4]byte{0x5b, 0x5e, 0x13, 0x9f}
	InterfaceIDERC1155Metadata = [4]byte{0x0e, 0x89, 0x34, 0x1c}
)

// Token-transfer event signatures.
//
// ERC-20 and ERC-721 share the same Transfer(address,address,uint256) hash;
// disambiguation is by topic count (3 vs 4) and confirmed against the
// contract's ERC-165 declaration in DetectContractType.
const (
	TransferEventSignature       = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	TransferSingleEventSignature = "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62"
	TransferBatchEventSignature  = "0x4a39dc06d4c0dbc64b50af327f5f6b67b0c4b21be9d6c92d40b00b3e7ec1c3ef"
)

// CallContractMethod makes a qrl_call to a contract method and returns the result
func CallContractMethod(contractAddress string, methodSig string) (string, error) {
	zap.L().Debug("Calling contract method",
		zap.String("contractAddress", contractAddress),
		zap.String("methodSig", methodSig[:10]+"...")) // Log just the beginning of the signature for brevity

	// Ensure contract address has Q prefix for QRL RPC
	contractAddress = validation.ConvertToQAddress(contractAddress)

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "qrl_call",
		Params: []interface{}{
			map[string]string{
				"to":   contractAddress,
				"data": methodSig,
			},
			"latest",
		},
		ID: 1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		zap.L().Error("Failed to marshal JSON for contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	zap.L().Debug("Sending RPC request",
		zap.String("url", Endpoints().CurrentURL()),
		zap.String("method", "qrl_call"))

	body, err := DoNodeRPC(b)
	if err != nil {
		zap.L().Error("Failed to execute HTTP request for contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to execute request: %v", err)
	}

	// Log full response for debugging
	zap.L().Debug("Received contract call response",
		zap.String("contractAddress", contractAddress),
		zap.String("response", string(body)))

	var result struct {
		Jsonrpc string
		ID      int
		Result  string
		Error   *struct {
			Code    int
			Message string
		}
	}
	if err := json.Unmarshal(body, &result); err != nil {
		zap.L().Error("Failed to unmarshal response from contract call",
			zap.String("contractAddress", contractAddress),
			zap.Error(err))
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if result.Error != nil {
		zap.L().Error("RPC error in contract call",
			zap.String("contractAddress", contractAddress),
			zap.Int("errorCode", result.Error.Code),
			zap.String("errorMessage", result.Error.Message))
		return "", fmt.Errorf("RPC error: %v", result.Error.Message)
	}

	// Truncate the result for logging if it's too long
	resultForLog := result.Result
	if len(resultForLog) > 100 {
		resultForLog = resultForLog[:100] + "..."
	}
	zap.L().Debug("Contract call successful",
		zap.String("contractAddress", contractAddress),
		zap.String("result", resultForLog))

	return result.Result, nil
}

// Canonical TokenStandard values persisted on ContractInfo.TokenStandard.
const (
	StandardERC20   = "ERC-20"
	StandardERC721  = "ERC-721"
	StandardERC1155 = "ERC-1155"
)

// ContractDetectionResult is returned by DetectContractType. Fields outside
// of Standard/Name/Symbol/HasERC165 are only populated for ERC-20 (Decimals,
// TotalSupply); NFT collections often omit `decimals()` entirely.
//
// MetadataURI is best-effort populated for NFT (ERC-721/1155) contracts via
// the OpenSea-convention contractURI() probe. Empty means either "the contract
// doesn't implement contractURI" or "the probe failed transiently" - the
// background metadata fetcher service handles the URI -> JSON resolution
// separately, so a missing URI at classification time can be filled in on a
// later reclassification pass without breaking anything.
type ContractDetectionResult struct {
	Standard    string // StandardERC20 | StandardERC721 | StandardERC1155 | ""
	Name        string
	Symbol      string
	Decimals    uint8
	TotalSupply string
	HasERC165   bool
	MetadataURI string
}

// DetectContractType classifies a contract by trying ERC-165 supportsInterface
// first (the cheap, definitive signal for ERC-721/1155) and falling back to
// the ERC-20 name+symbol+decimals triad otherwise.
//
// Error contract: a non-nil error means the *probe itself* failed (transport
// blip, etc.), the caller MUST bail without writing classification fields,
// to preserve the C5 promote-only invariant established in #88. A nil error
// with Standard=="" simply means "we can't tell what this is" (and that IS
// safe to write; the merge in StoreContract treats "" as no-op).
//
// Detection order picks the broader standard first so ERC-1155 implementations
// that ALSO satisfy ERC-721 are categorised as ERC-1155 (matches the plan's
// dual-impl tie-breaker).
func DetectContractType(addr string) (ContractDetectionResult, error) {
	// Try ERC-1155 first (broader spec).
	supports, hasERC165, err := SupportsInterface(addr, InterfaceIDERC1155)
	if err != nil {
		return ContractDetectionResult{}, fmt.Errorf("supportsInterface(ERC-1155): %w", err)
	}
	if supports {
		name, _ := GetTokenName(addr)     // best-effort; many ERC-1155s omit name()
		symbol, _ := GetTokenSymbol(addr) // best-effort; many ERC-1155s omit symbol()
		metaURI, _ := GetContractURI(addr) // best-effort; many collections omit contractURI()
		return ContractDetectionResult{
			Standard:    StandardERC1155,
			Name:        name,
			Symbol:      symbol,
			HasERC165:   true,
			MetadataURI: metaURI,
		}, nil
	}

	// Try ERC-721.
	supports721, hasERC165From721, err := SupportsInterface(addr, InterfaceIDERC721)
	if err != nil {
		return ContractDetectionResult{}, fmt.Errorf("supportsInterface(ERC-721): %w", err)
	}
	if supports721 {
		name, _ := GetTokenName(addr)
		symbol, _ := GetTokenSymbol(addr)
		metaURI, _ := GetContractURI(addr)
		return ContractDetectionResult{
			Standard:    StandardERC721,
			Name:        name,
			Symbol:      symbol,
			HasERC165:   true,
			MetadataURI: metaURI,
		}, nil
	}

	// Either probe confirmed the contract responds to ERC-165 (it just
	// doesn't support either NFT interface). Record that, then fall through
	// to ERC-20 detection, some hybrid contracts (rare) declare ERC-165
	// without being ERC-721/1155 and ARE ERC-20.
	erc165Known := hasERC165 || hasERC165From721

	// Fall back to the original ERC-20 name+symbol+decimals triad.
	name, symbol, decimals, isERC20 := GetTokenInfo(addr)
	if !isERC20 {
		return ContractDetectionResult{HasERC165: erc165Known}, nil
	}
	totalSupply, err := GetTokenTotalSupply(addr)
	if err != nil {
		// totalSupply RPC failure is non-fatal, the triad already
		// classified this as ERC-20. Leave TotalSupply empty; the merge
		// in StoreContract won't demote an existing value.
		totalSupply = ""
	}
	return ContractDetectionResult{
		Standard:    StandardERC20,
		Name:        name,
		Symbol:      symbol,
		Decimals:    decimals,
		TotalSupply: totalSupply,
		HasERC165:   erc165Known,
	}, nil
}

// GetTokenInfo attempts to determine if a contract is an ERC20 token and returns its details
func GetTokenInfo(contractAddress string) (string, string, uint8, bool) {
	zap.L().Info("Checking if contract is a token", zap.String("address", contractAddress))

	// First check if the contract has a valid 'name' method
	name, err := GetTokenName(contractAddress)
	if err != nil {
		zap.L().Debug("Contract does not have a valid name method",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	zap.L().Info("Contract has a valid name",
		zap.String("address", contractAddress),
		zap.String("name", name))

	// Now check for symbol
	symbol, err := GetTokenSymbol(contractAddress)
	if err != nil {
		zap.L().Debug("Contract does not have a valid symbol method",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	zap.L().Info("Contract has a valid symbol",
		zap.String("address", contractAddress),
		zap.String("symbol", symbol))

	// Finally check for decimals
	decimals, err := GetTokenDecimals(contractAddress)
	if err != nil {
		zap.L().Debug("Contract does not have a valid decimals method",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	zap.L().Info("Contract has valid decimals",
		zap.String("address", contractAddress),
		zap.Uint8("decimals", decimals))

	// If we got here, this is likely a valid token
	zap.L().Info("Detected valid ERC20 token",
		zap.String("address", contractAddress),
		zap.String("name", name),
		zap.String("symbol", symbol),
		zap.Uint8("decimals", decimals))

	return name, symbol, decimals, true
}

// GetTokenName retrieves the name of an ERC20 token
func GetTokenName(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_NAME)
	if err != nil {
		return "", err
	}

	// Remove 0x prefix
	result = strings.TrimPrefix(result, "0x")

	// If the result is empty or all zeros, return an error
	if len(result) == 0 || strings.TrimLeft(result, "0") == "" {
		return "", fmt.Errorf("empty result")
	}

	// Handle different response formats:

	// Format 1: Dynamic string (most common)
	// First 32 bytes (64 chars) contain the offset to the string data
	// Next 32 bytes contain the string length
	// Followed by the string data
	if len(result) >= 128 {
		// Try parsing as dynamic string
		offsetHex := result[:64]
		offset, err := strconv.ParseInt(offsetHex, 16, 64)
		if err == nil && offset*2 < int64(len(result)) {
			// Get the length from the offset position
			startPos := offset * 2
			if startPos+64 <= int64(len(result)) {
				lengthHex := result[startPos : startPos+64]
				length, err := strconv.ParseInt(lengthHex, 16, 64)
				if err == nil && startPos+64+length*2 <= int64(len(result)) {
					dataHex := result[startPos+64 : startPos+64+length*2]
					if nameBytes, err := hex.DecodeString(dataHex); err == nil {
						return string(nameBytes), nil
					}
				}
			}
		}
	}

	// Format 2: Fixed string (less common)
	// The entire response is the hex-encoded string
	if nameBytes, err := hex.DecodeString(strings.TrimRight(result, "0")); err == nil {
		return string(nameBytes), nil
	}

	return "", fmt.Errorf("failed to decode token name")
}

// GetTokenSymbol retrieves the symbol of an ERC20 token
func GetTokenSymbol(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_SYMBOL)
	if err != nil {
		return "", err
	}

	// Decode the ABI-encoded string
	if len(result) < 130 {
		return "", fmt.Errorf("response too short")
	}

	// Extract the string length and data
	dataStart := 2 + 64 // skip "0x" and first 32 bytes
	lengthHex := result[dataStart : dataStart+64]
	length, err := strconv.ParseInt(lengthHex, 16, 64)
	if err != nil {
		return "", err
	}

	dataHex := result[dataStart+64 : dataStart+64+int(length)*2]
	symbolBytes, err := hex.DecodeString(dataHex)
	if err != nil {
		return "", err
	}

	return string(symbolBytes), nil
}

// GetTokenDecimals retrieves the number of decimals for an ERC20 token
func GetTokenDecimals(contractAddress string) (uint8, error) {
	result, err := CallContractMethod(contractAddress, SIG_DECIMALS)
	if err != nil {
		return 0, err
	}

	if len(result) < 66 {
		return 0, fmt.Errorf("response too short")
	}

	decimals, err := strconv.ParseUint(result[2:], 16, 8)
	if err != nil {
		return 0, err
	}

	return uint8(decimals), nil
}

// GetTokenTotalSupply retrieves the total supply of an ERC20 token
func GetTokenTotalSupply(contractAddress string) (string, error) {
	result, err := CallContractMethod(contractAddress, SIG_SUPPLY)
	if err != nil {
		return "", err
	}

	if len(result) < 66 {
		return "", fmt.Errorf("response too short")
	}

	// Convert hex to decimal
	bigInt := new(big.Int)
	if _, ok := bigInt.SetString(strings.TrimPrefix(result, "0x"), 16); !ok {
		return "", fmt.Errorf("failed to parse total supply")
	}

	// Return decimal string
	return bigInt.String(), nil
}

// GetTokenBalance retrieves the balance of an ERC20 token for a specific address
func GetTokenBalance(contractAddress string, holderAddress string) (string, error) {
	// balanceOf(address) function signature
	methodID := "0x70a08231"

	// Enhanced logging with full input addresses
	zap.L().Debug("Getting token balance - raw input",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", holderAddress))

	// Special handling for zero address (common in mint events)
	// Handle multiple formats of zero address
	if holderAddress == "Q0" ||
		holderAddress == "Q0000000000000000000000000000000000000000" ||
		holderAddress == "0x0" ||
		holderAddress == "0x0000000000000000000000000000000000000000" {
		zap.L().Info("Zero address detected, returning zero balance",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", holderAddress))
		return "0", nil
	}

	// Ensure contract address has Q prefix for QRL RPC
	contractAddress = validation.ConvertToQAddress(contractAddress)

	// Ensure holder address has Q prefix for RPC
	originalHolderAddress := holderAddress // Keep original for logging
	holderAddress = validation.ConvertToQAddress(holderAddress)

	// Extract the raw address (without prefix) for padding
	rawAddress := validation.StripAddressPrefix(holderAddress)

	// Pad address to 32 bytes (64 hex chars) for ABI encoding
	paddedAddress := rawAddress
	for len(paddedAddress) < 64 {
		paddedAddress = "0" + paddedAddress
	}

	// Combine method ID and padded address
	data := methodID + paddedAddress
	zap.L().Debug("Prepared contract call data",
		zap.String("contractAddress", contractAddress),
		zap.String("formattedAddress", holderAddress),
		zap.String("rawAddress", rawAddress),
		zap.String("paddedAddress", paddedAddress),
		zap.String("data", data))

	// Make the call
	result, err := CallContractMethod(contractAddress, data)
	if err != nil {
		// Try up to 3 times with exponential backoff on failure
		maxRetries := 2
		for retry := 0; retry < maxRetries && err != nil; retry++ {
			retryDelay := time.Duration(500*(retry+1)) * time.Millisecond

			zap.L().Warn("Retrying token balance call after failure",
				zap.String("contractAddress", contractAddress),
				zap.String("holderAddress", holderAddress),
				zap.Int("retry", retry+1),
				zap.Duration("delay", retryDelay),
				zap.Error(err))

			time.Sleep(retryDelay)
			result, err = CallContractMethod(contractAddress, data)
		}

		// If all retries failed
		if err != nil {
			zap.L().Error("Contract call for token balance failed after retries",
				zap.String("contractAddress", contractAddress),
				zap.String("holderAddress", originalHolderAddress),
				zap.String("formattedAddress", holderAddress),
				zap.String("paddedAddress", paddedAddress),
				zap.Error(err))
			return "", fmt.Errorf("contract call failed: %v", err)
		}
	}

	// Parse result
	if len(result) < 2 {
		zap.L().Warn("Empty result from token balance call",
			zap.String("contractAddress", contractAddress),
			zap.String("holderAddress", originalHolderAddress))
		return "0", nil
	}

	// Convert hex string to big.Int
	bigInt := new(big.Int)
	bigInt.SetString(strings.TrimPrefix(result, "0x"), 16)

	balance := bigInt.String()
	zap.L().Info("Retrieved token balance",
		zap.String("contractAddress", contractAddress),
		zap.String("holderAddress", originalHolderAddress),
		zap.String("balance", balance))

	return balance, nil
}

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

// GetTransactionReceipt gets the transaction receipt which includes logs
func GetTransactionReceipt(txHash string) (*models.TransactionReceipt, error) {
	if txHash == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "qrl_getTransactionReceipt",
		Params:  []interface{}{txHash},
		ID:      1,
	}

	b, err := json.Marshal(group)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	body, err := DoNodeRPC(b)
	if err != nil {
		return nil, fmt.Errorf("failed to make RPC request: %v", err)
	}

	// First unmarshal into a map to check for JSON-RPC error
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Check for JSON-RPC error
	if errObj, ok := rawResponse["error"]; ok {
		return nil, fmt.Errorf("RPC error: %v", errObj)
	}

	var receipt models.TransactionReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %v", err)
	}

	return &receipt, nil
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

// TrimLeftZeros trims leading zeros from hex string
func TrimLeftZeros(hex string) string {
	for i := 0; i < len(hex); i++ {
		if hex[i] != '0' {
			return hex[i:]
		}
	}
	return "0"
}

// ParseTransferEvent parses a transfer event log.
// Addresses are returned in canonical Q-prefix form.
func ParseTransferEvent(log models.Log) (string, string, *big.Int, error) {
	// Extract addresses from topics (32-byte padded, take last 40 hex chars)
	from := "Q" + strings.ToLower(log.Topics[1][26:])
	to := "Q" + strings.ToLower(log.Topics[2][26:])

	// Validate addresses
	if !validation.IsValidAddress(from) {
		return "", "", nil, fmt.Errorf("invalid from address: %s", from)
	}

	if !validation.IsValidAddress(to) {
		return "", "", nil, fmt.Errorf("invalid to address: %s", to)
	}

	// Parse amount from data field
	amount := new(big.Int)
	if len(log.Data) > 2 {
		data := log.Data
		if _, success := amount.SetString(data, 16); !success {
			return "", "", nil, fmt.Errorf("failed to parse amount from data: %s", log.Data)
		}
	}

	return from, to, amount, nil
}

// SupportsInterface probes a contract via ERC-165 supportsInterface(bytes4).
//
// Returns:
//   - supports    : contract declared support for the queried interface
//   - hasERC165   : contract returned a well-formed bool32 (i.e. it implements
//     ERC-165 at all). A `false, true` result means "ERC-165 contract, doesn't
//     implement this interface", useful to skip later probes.
//   - err         : transport-level failure (timeout, network down, etc).
//     The caller MUST check err and bail without classifying, a transient
//     blip never demotes existing state (mirrors the C5 promote-only
//     invariant in db/contracts.go:StoreContract).
//
// A contract-level revert ("execution reverted") is mapped to
// `false, false, nil` because legacy ERC-20s without ERC-165 revert on
// any unknown selector, and that's the discriminator we rely on.
//
// Calldata layout per ABI spec is selector + interfaceID (right-padded to
// 32 bytes), NOT left-padded, `bytes4` is a fixed-length type and the
// ABI pads fixed types on the right with zero bytes.
func SupportsInterface(addr string, interfaceID [4]byte) (supports, hasERC165 bool, err error) {
	calldata := SIG_SUPPORTS_INTERFACE + hex.EncodeToString(interfaceID[:]) + strings.Repeat("0", 56)

	result, callErr := CallContractMethod(addr, calldata)
	if callErr != nil {
		// "RPC error: <message>" is set by CallContractMethod ONLY when the
		// node returned a JSON-RPC error object (revert / invalid opcode /
		// out-of-gas / unknown method). That's the not-ERC-165 signal.
		// Anything else (request build, transport, unmarshal) is transient.
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return false, false, nil
		}
		return false, false, callErr
	}

	stripped := strings.TrimPrefix(result, "0x")
	// Empty / "0x" / malformed-too-short: treat as not-ERC-165 (a real
	// ERC-165 contract returns a full 32-byte bool32).
	if len(stripped) < 64 {
		return false, false, nil
	}

	// Last byte of the 32-byte bool32 tells us true (0x01) vs false (0x00).
	lastByte := stripped[len(stripped)-2:]
	switch lastByte {
	case "01":
		return true, true, nil
	case "00":
		return false, true, nil
	default:
		// Non-bool return (e.g. legacy contract returning data of different
		// shape). Conservatively not-ERC-165.
		return false, false, nil
	}
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

// maxBatchArrayLen caps the per-array element count we'll decode from a
// TransferBatch log. Real-world batches are <100; this guards against an
// adversarial payload claiming a huge array to OOM the decoder.
const maxBatchArrayLen = 10000

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

// readUint64FromWord parses one 32-byte ABI word at hex position `posHex`
// and returns its value as uint64. Returns an error if the value exceeds
// uint64 range (offsets in real logs are tens to hundreds of bytes).
func readUint64FromWord(data string, posHex uint64) (uint64, error) {
	if posHex+64 > uint64(len(data)) {
		return 0, fmt.Errorf("data too short at hex offset %d", posHex)
	}
	word := new(big.Int)
	if _, ok := word.SetString(data[posHex:posHex+64], 16); !ok {
		return 0, fmt.Errorf("failed to parse word at hex offset %d", posHex)
	}
	if !word.IsUint64() {
		return 0, fmt.Errorf("word value exceeds uint64 at hex offset %d", posHex)
	}
	return word.Uint64(), nil
}

// decodeUint256Array reads an ABI-encoded uint256[] starting at hex position
// `startHex` of `data`. The encoding is `[length(32B) || elements(32B each)]`.
func decodeUint256Array(data string, startHex uint64) ([]*big.Int, error) {
	length, err := readUint64FromWord(data, startHex)
	if err != nil {
		return nil, err
	}
	if length > maxBatchArrayLen {
		return nil, fmt.Errorf("array length %d exceeds cap %d", length, maxBatchArrayLen)
	}
	elemStart := startHex + 64
	end := elemStart + length*64
	if end > uint64(len(data)) {
		return nil, fmt.Errorf("data too short for %d elements", length)
	}
	out := make([]*big.Int, length)
	for i := uint64(0); i < length; i++ {
		pos := elemStart + i*64
		item := new(big.Int)
		if _, ok := item.SetString(data[pos:pos+64], 16); !ok {
			return nil, fmt.Errorf("failed to parse element %d", i)
		}
		out[i] = item
	}
	return out, nil
}

// encodeAddressForABI strips the Q/0x prefix from `addr` and left-pads it to a
// 32-byte word for ABI encoding. Caller is responsible for passing a validated
// address; the upstream `validation.IsValidAddress` check is enforced where
// callers source addresses from log topics or user input.
func encodeAddressForABI(addr string) string {
	raw := strings.ToLower(validation.StripAddressPrefix(addr))
	if len(raw) > 64 {
		raw = raw[len(raw)-64:]
	}
	return strings.Repeat("0", 64-len(raw)) + raw
}

// encodeUint256ForABI left-pads `v` to a 32-byte uint256 word.
// Negative values are rejected (uint256 has no sign).
func encodeUint256ForABI(v *big.Int) (string, error) {
	if v == nil {
		return "", fmt.Errorf("uint256 cannot be nil")
	}
	if v.Sign() < 0 {
		return "", fmt.Errorf("uint256 cannot be negative: %s", v.String())
	}
	h := v.Text(16)
	if len(h) > 64 {
		return "", fmt.Errorf("uint256 exceeds 32 bytes: %s", v.String())
	}
	return strings.Repeat("0", 64-len(h)) + h, nil
}

// parseAddressFromWord decodes the rightmost 32-byte word of `result` into a
// canonical Q-prefix lowercase address. Returns ("", nil) if the word is the
// zero address, callers interpret that as "no owner" (sparse storage).
func parseAddressFromWord(result string) (string, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) < 64 {
		return "", fmt.Errorf("address word too short: %d hex chars", len(stripped))
	}
	word := stripped[len(stripped)-64:]
	// All-zero word is the zero address: treat as "no owner".
	if strings.TrimLeft(word, "0") == "" {
		return "", nil
	}
	addrHex := word[len(word)-40:]
	// The high 12 bytes (24 hex chars) must be zero for a valid 20-byte address.
	if strings.TrimLeft(word[:24], "0") != "" {
		return "", fmt.Errorf("address word has nonzero high bytes: %s", word)
	}
	addr := "Q" + strings.ToLower(addrHex)
	if !validation.IsValidAddress(addr) {
		return "", fmt.Errorf("invalid address derived from word: %s", addr)
	}
	return addr, nil
}

// parseUint256FromWord decodes the rightmost 32-byte word of `result` into a
// *big.Int. Returns nil + error when the word is shorter than 32 bytes or
// fails hex parsing.
func parseUint256FromWord(result string) (*big.Int, error) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) < 64 {
		return nil, fmt.Errorf("uint256 word too short: %d hex chars", len(stripped))
	}
	word := stripped[len(stripped)-64:]
	v := new(big.Int)
	if _, ok := v.SetString(word, 16); !ok {
		return nil, fmt.Errorf("failed to parse uint256 from word: %s", word)
	}
	return v, nil
}

// GetERC721Owner queries `ownerOf(uint256)` on an ERC-721 contract and returns
// the current owner's Q-prefix address.
//
// Three-way error contract (mirrors SupportsInterface):
//   - Contract revert (burned or never-minted id): returns ("", nil), callers
//     treat as "no current owner" and delete any stale balance row.
//   - Transport error: returns ("", err), callers preserve existing state to
//     uphold the C5 promote-only invariant.
//   - Zero address / empty word: returns ("", nil), same "no owner" semantics.
//
// TODO(scale): one RPC call per ERC-721 transfer. Testnet handles it; mainnet
// at high NFT volume would want an event-delta accumulator with periodic
// reconciliation instead.
func GetERC721Owner(contractAddress string, tokenID *big.Int) (string, error) {
	idWord, err := encodeUint256ForABI(tokenID)
	if err != nil {
		return "", fmt.Errorf("encode tokenID: %w", err)
	}
	calldata := SIG_OWNER_OF + idWord

	result, callErr := CallContractMethod(contractAddress, calldata)
	if callErr != nil {
		// JSON-RPC error => contract reverted (typical for burned/unminted ids).
		// Anything else (transport, marshalling) is transient; propagate so the
		// caller can preserve existing state.
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return "", nil
		}
		return "", callErr
	}
	return parseAddressFromWord(result)
}

// parseDynamicString decodes the ABI-encoded `bytes`/`string` return value of
// a no-argument view function (e.g. name(), symbol(), contractURI()).
//
// The ABI layout for a single dynamic return value is:
//
//	[ offset (32B) || length (32B) || data (length bytes, right-padded to 32B) ]
//
// `result` is the raw `qrl_call` result with or without the "0x" prefix.
// Empty or all-zero results return ("", nil), the caller treats that as
// "method returned empty" which is distinct from a transport / decode error.
//
// Defensive against malformed payloads: every offset / length read is
// bounds-checked against the available hex chars, so a truncated response
// returns a clean error instead of panicking.
func parseDynamicString(result string) (string, error) {
	stripped := strings.TrimPrefix(result, "0x")

	// Empty or all-zero is treated as "method exists but returned empty
	// string"; not an error so callers can record `metadataURI=""` as a
	// successful probe (no URI set on chain).
	if len(stripped) == 0 || strings.TrimLeft(stripped, "0") == "" {
		return "", nil
	}

	// Minimum payload: 32 bytes offset + 32 bytes length = 128 hex chars.
	if len(stripped) < 128 {
		return "", fmt.Errorf("dynamic string payload too short: %d hex chars", len(stripped))
	}

	// ABI words are 256 bits; strconv.ParseInt caps at 64. Real offsets are
	// tiny (start at 0x20 = 32 bytes for one dynamic return), but an
	// attacker-crafted return value could fill the full 32-byte word to try
	// to crash the decoder. Parse as big.Int and reject anything that
	// doesn't fit a signed int64 before downcasting.
	offsetInt := new(big.Int)
	if _, ok := offsetInt.SetString(stripped[:64], 16); !ok {
		return "", fmt.Errorf("invalid offset word")
	}
	if !offsetInt.IsInt64() || offsetInt.Sign() < 0 {
		return "", fmt.Errorf("offset out of int64 range")
	}
	offset := offsetInt.Int64()
	startPos := offset * 2
	if startPos+64 > int64(len(stripped)) {
		return "", fmt.Errorf("string length word out of bounds: offset=%d payload=%d", offset, len(stripped)/2)
	}

	lengthInt := new(big.Int)
	if _, ok := lengthInt.SetString(stripped[startPos:startPos+64], 16); !ok {
		return "", fmt.Errorf("invalid length word")
	}
	if !lengthInt.IsInt64() || lengthInt.Sign() < 0 {
		return "", fmt.Errorf("length out of int64 range")
	}
	length := lengthInt.Int64()
	// Cap at a sane upper bound; nothing legitimate is going to return a
	// multi-MB string from a view call, and an attacker could otherwise
	// force a huge allocation.
	const maxDynamicStringBytes = 1 << 20 // 1 MiB
	if length > maxDynamicStringBytes {
		return "", fmt.Errorf("dynamic string length %d exceeds cap %d", length, maxDynamicStringBytes)
	}
	if startPos+64+length*2 > int64(len(stripped)) {
		return "", fmt.Errorf("string data out of bounds: declared length=%d available=%d", length, (int64(len(stripped))-startPos-64)/2)
	}

	dataHex := stripped[startPos+64 : startPos+64+length*2]
	bytes, err := hex.DecodeString(dataHex)
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}
	return string(bytes), nil
}

// GetContractURI queries `contractURI()` on a contract (OpenSea convention
// for collection-level NFT metadata) and returns the raw URI string.
//
// Three-way error contract, mirrors SupportsInterface / GetERC721Owner:
//   - Contract revert (the contract doesn't implement contractURI, which is
//     true of most collections): returns ("", nil) so the caller can record
//     "no collection metadata" without treating it as a fetch failure.
//   - Transport error: returns ("", err); callers preserve existing state
//     (C5 promote-only invariant).
//   - Empty / malformed return: returns ("", nil), same "no URI" semantic.
//
// The returned URI is whatever the contract emitted, no IPFS gateway
// resolution happens here, the metadata service does that.
func GetContractURI(contractAddress string) (string, error) {
	result, callErr := CallContractMethod(contractAddress, SIG_CONTRACT_URI)
	if callErr != nil {
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return "", nil
		}
		return "", callErr
	}
	uri, decErr := parseDynamicString(result)
	if decErr != nil {
		// Malformed return word, treat as "no URI" rather than failing the
		// caller; we never want a single misbehaving contract to throw off
		// classification of every contract that comes after.
		return "", nil
	}
	return uri, nil
}

// GetERC1155Balance queries `balanceOf(address,uint256)` on an ERC-1155
// contract and returns the holder's current per-id balance.
//
// Three-way error contract:
//   - Contract revert: returns (big.NewInt(0), nil), treated as "no balance"
//     (sparse storage; caller deletes the row).
//   - Transport error: returns (nil, err), caller preserves existing state.
//   - Empty / zero return: returns (big.NewInt(0), nil), same as revert.
//
// TODO(scale): one RPC call per (from, to) side of an ERC-1155 transfer.
// Same accumulator/reconciliation concern as GetERC721Owner above.
func GetERC1155Balance(contractAddress, holderAddress string, tokenID *big.Int) (*big.Int, error) {
	if !validation.IsValidAddress(holderAddress) {
		return nil, fmt.Errorf("invalid holder address: %s", holderAddress)
	}
	idWord, err := encodeUint256ForABI(tokenID)
	if err != nil {
		return nil, fmt.Errorf("encode tokenID: %w", err)
	}
	calldata := SIG_BALANCE_OF_1155 + encodeAddressForABI(holderAddress) + idWord

	result, callErr := CallContractMethod(contractAddress, calldata)
	if callErr != nil {
		if strings.HasPrefix(callErr.Error(), "RPC error:") {
			return big.NewInt(0), nil
		}
		return nil, callErr
	}
	return parseUint256FromWord(result)
}
