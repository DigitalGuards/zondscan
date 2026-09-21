package rpc

import (
	"QRL2MongoDB/validation"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

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
// the OpenSea-convention contractURI() probe. Empty means the contract does
// not implement contractURI or returned malformed optional metadata. Transport
// failures are returned so durable ingestion retries without persisting a
// partial classification.
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
// mandatory ERC-20 totalSupply and balanceOf behavior otherwise. ERC-20 name,
// symbol, and decimals remain optional enrichment.
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
	return detectContractType(addr, CallContractMethod)
}

// DetectContractTypeAtBlock classifies a contract against the exact state
// selected by blockHash. Historical token replay must not use latest state:
// contract code can be upgraded or removed after the claimed block.
func DetectContractTypeAtBlock(addr string, blockHash string) (ContractDetectionResult, error) {
	blockHash = strings.ToLower(blockHash)
	if err := validation.ValidateHexString(blockHash, validation.HashLength); err != nil {
		return ContractDetectionResult{}, fmt.Errorf("invalid classification block hash: %w", err)
	}
	return detectContractType(addr, func(contractAddress string, methodSig string) (string, error) {
		return callContractMethodAtBlockHash(contractAddress, methodSig, blockHash)
	})
}

func detectContractType(addr string, call contractMethodCaller) (ContractDetectionResult, error) {
	// Try ERC-1155 first (broader spec).
	supports, hasERC165, err := supportsInterfaceWithCaller(addr, InterfaceIDERC1155, call)
	if err != nil {
		return ContractDetectionResult{}, fmt.Errorf("supportsInterface(ERC-1155): %w", err)
	}
	if supports {
		name, err := optionalTokenString(addr, SIG_NAME, decodeTokenName, call)
		if err != nil {
			return ContractDetectionResult{}, fmt.Errorf("ERC-1155 name: %w", err)
		}
		symbol, err := optionalTokenString(addr, SIG_SYMBOL, decodeTokenSymbol, call)
		if err != nil {
			return ContractDetectionResult{}, fmt.Errorf("ERC-1155 symbol: %w", err)
		}
		metaURI, err := optionalTokenString(addr, SIG_CONTRACT_URI, parseDynamicString, call)
		if err != nil {
			return ContractDetectionResult{}, fmt.Errorf("ERC-1155 contractURI: %w", err)
		}
		return ContractDetectionResult{
			Standard:    StandardERC1155,
			Name:        name,
			Symbol:      symbol,
			HasERC165:   true,
			MetadataURI: metaURI,
		}, nil
	}

	// Try ERC-721.
	supports721, hasERC165From721, err := supportsInterfaceWithCaller(addr, InterfaceIDERC721, call)
	if err != nil {
		return ContractDetectionResult{}, fmt.Errorf("supportsInterface(ERC-721): %w", err)
	}
	if supports721 {
		name, err := optionalTokenString(addr, SIG_NAME, decodeTokenName, call)
		if err != nil {
			return ContractDetectionResult{}, fmt.Errorf("ERC-721 name: %w", err)
		}
		symbol, err := optionalTokenString(addr, SIG_SYMBOL, decodeTokenSymbol, call)
		if err != nil {
			return ContractDetectionResult{}, fmt.Errorf("ERC-721 symbol: %w", err)
		}
		metaURI, err := optionalTokenString(addr, SIG_CONTRACT_URI, parseDynamicString, call)
		if err != nil {
			return ContractDetectionResult{}, fmt.Errorf("ERC-721 contractURI: %w", err)
		}
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

	// Fall back to mandatory ERC-20 read behavior and optional metadata.
	name, symbol, decimals, isERC20, err := getTokenInfoStrict(addr, call)
	if err != nil {
		return ContractDetectionResult{}, err
	}
	if !isERC20 {
		return ContractDetectionResult{HasERC165: erc165Known}, nil
	}
	totalSupply, err := optionalTokenTotalSupply(addr, call)
	if err != nil {
		return ContractDetectionResult{}, fmt.Errorf("ERC-20 totalSupply: %w", err)
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

type contractMethodCaller func(contractAddress string, methodSig string) (string, error)

func callContractMethodAtBlockHash(
	contractAddress string,
	methodSig string,
	blockHash string,
) (string, error) {
	params, err := exactBlockContractCallParams(contractAddress, methodSig, blockHash)
	if err != nil {
		return "", err
	}
	var result struct {
		Result string `json:"result"`
	}
	err = rpcCall("qrl_call", params, &result)
	if err != nil {
		return "", err
	}
	return result.Result, nil
}

func exactBlockContractCallParams(
	contractAddress string,
	methodSig string,
	blockHash string,
) ([]interface{}, error) {
	contractAddress = validation.ConvertToQAddress(contractAddress)
	if !validation.IsValidAddress(contractAddress) {
		return nil, fmt.Errorf("invalid contract address: %s", contractAddress)
	}
	if !validation.IsValidHexString(methodSig) {
		return nil, fmt.Errorf("invalid contract calldata")
	}
	blockHash = strings.ToLower(blockHash)
	if err := validation.ValidateHexString(blockHash, validation.HashLength); err != nil {
		return nil, fmt.Errorf("invalid contract call block hash: %w", err)
	}
	return []interface{}{
		map[string]string{
			"to":   contractAddress,
			"data": methodSig,
		},
		map[string]interface{}{
			"blockHash":        blockHash,
			"requireCanonical": false,
		},
	}, nil
}

// isConfirmedContractRevert identifies the QRVM's explicit revert error.
// go-qrl reports a revert that carries return data as code 3 and a bare
// revert (no reason, for example a missing function selector on a contract
// without ERC-165) as code -32000 with the message "execution reverted".
// Both are deterministic contract answers. Other JSON-RPC errors, including
// unavailable historical state and server failures, must remain retryable.
func isConfirmedContractRevert(err error) bool {
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		return false
	}
	if rpcErr.Code == 3 {
		return true
	}
	return rpcErr.Code == -32000 && strings.HasPrefix(strings.TrimSpace(rpcErr.Message), "execution reverted")
}

// optionalTokenString treats a confirmed contract-level revert or malformed
// ABI value as an unsupported optional field. Transport failures remain
// retryable so callers cannot persist a partial classification during an RPC
// outage.
func optionalTokenString(
	contractAddress string,
	methodSig string,
	decode func(string) (string, error),
	call contractMethodCaller,
) (string, error) {
	result, err := call(contractAddress, methodSig)
	if err != nil {
		if isConfirmedContractRevert(err) {
			return "", nil
		}
		return "", err
	}
	value, err := decode(result)
	if err != nil {
		return "", nil
	}
	return value, nil
}

func optionalTokenTotalSupply(
	contractAddress string,
	call contractMethodCaller,
) (string, error) {
	result, err := call(contractAddress, SIG_SUPPLY)
	if err != nil {
		if isConfirmedContractRevert(err) {
			return "", nil
		}
		return "", err
	}
	totalSupply, err := decodeTokenTotalSupply(result)
	if err != nil {
		return "", nil
	}
	return totalSupply, nil
}

func getTokenInfoStrict(
	contractAddress string,
	call contractMethodCaller,
) (string, string, uint8, bool, error) {
	// totalSupply() and balanceOf(address) are mandatory ERC-20 behavior.
	// The Transfer log being processed provides the event half of the signal;
	// these two read-only probes avoid relying on optional metadata methods.
	result, err := call(contractAddress, SIG_SUPPLY)
	if err != nil {
		if isConfirmedContractRevert(err) {
			return "", "", 0, false, nil
		}
		return "", "", 0, false, fmt.Errorf("token totalSupply: %w", err)
	}
	if _, err := decodeTokenTotalSupply(result); err != nil {
		return "", "", 0, false, nil
	}
	encodedAddress := encodeAddressForABI(contractAddress)
	if encodedAddress == "" {
		return "", "", 0, false, fmt.Errorf("invalid token address: %s", contractAddress)
	}
	result, err = call(contractAddress, SIG_BALANCE+encodedAddress)
	if err != nil {
		if isConfirmedContractRevert(err) {
			return "", "", 0, false, nil
		}
		return "", "", 0, false, fmt.Errorf("token balanceOf: %w", err)
	}
	if _, err := decodeTokenBalance(result); err != nil {
		return "", "", 0, false, nil
	}

	name, err := optionalTokenString(contractAddress, SIG_NAME, decodeTokenName, call)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("token name: %w", err)
	}
	symbol, err := optionalTokenString(contractAddress, SIG_SYMBOL, decodeTokenSymbol, call)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("token symbol: %w", err)
	}
	decimals, err := optionalTokenDecimals(contractAddress, call)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("token decimals: %w", err)
	}

	return name, symbol, decimals, true, nil
}

func optionalTokenDecimals(contractAddress string, call contractMethodCaller) (uint8, error) {
	result, err := call(contractAddress, SIG_DECIMALS)
	if err != nil {
		if isConfirmedContractRevert(err) {
			return 0, nil
		}
		return 0, err
	}
	decimals, err := decodeTokenDecimals(result)
	if err != nil {
		return 0, nil
	}
	return decimals, nil
}

// GetTokenInfo retains the historical best-effort API. Durable classification
// uses getTokenInfoStrict so transport failures reach the block retry queue.
func GetTokenInfo(contractAddress string) (string, string, uint8, bool) {
	zap.L().Info("Checking if contract is a token", zap.String("address", contractAddress))
	name, symbol, decimals, isERC20, err := getTokenInfoStrict(contractAddress, CallContractMethod)
	if err != nil {
		zap.L().Warn("Token classification RPC failed",
			zap.String("address", contractAddress),
			zap.Error(err))
		return "", "", 0, false
	}
	if !isERC20 {
		return "", "", 0, false
	}

	zap.L().Info("Detected valid ERC20 token",
		zap.String("address", contractAddress),
		zap.String("name", name),
		zap.String("symbol", symbol),
		zap.Uint8("decimals", decimals))

	return name, symbol, decimals, true
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
//     invariant in db/contracts_store.go:StoreContract).
//
// A contract-level revert ("execution reverted") is mapped to
// `false, false, nil` because legacy ERC-20s without ERC-165 revert on
// any unknown selector, and that's the discriminator we rely on.
//
// Calldata layout is selector + interfaceID right-padded to one 64-byte ABI
// word. `bytes4` is fixed-length and occupies the high four bytes.
func SupportsInterface(addr string, interfaceID [4]byte) (supports, hasERC165 bool, err error) {
	return supportsInterfaceWithCaller(addr, interfaceID, CallContractMethod)
}

func supportsInterfaceWithCaller(
	addr string,
	interfaceID [4]byte,
	call contractMethodCaller,
) (supports, hasERC165 bool, err error) {
	calldata := SIG_SUPPORTS_INTERFACE + encodeBytes4ForABI(interfaceID)

	result, callErr := call(addr, calldata)
	if callErr != nil {
		// Only the QRVM's confirmed code-3 revert is the not-ERC-165 signal.
		// Other JSON-RPC, transport, and decode failures remain retryable.
		if isConfirmedContractRevert(callErr) {
			return false, false, nil
		}
		return false, false, callErr
	}

	value, ok := parseBoolFromWord(result)
	return value, ok, nil
}

func encodeBytes4ForABI(value [4]byte) string {
	return hex.EncodeToString(value[:]) + strings.Repeat("0", abiWordHexLength-8)
}

func parseBoolFromWord(result string) (bool, bool) {
	stripped := strings.TrimPrefix(result, "0x")
	if len(stripped) != abiWordHexLength {
		return false, false
	}
	word := stripped
	if strings.TrimLeft(word[:abiWordHexLength-2], "0") != "" {
		return false, false
	}
	switch word[abiWordHexLength-2:] {
	case "01":
		return true, true
	case "00":
		return false, true
	default:
		return false, false
	}
}
