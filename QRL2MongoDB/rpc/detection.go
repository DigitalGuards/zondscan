package rpc

import (
	"encoding/hex"
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
		name, _ := GetTokenName(addr)      // best-effort; many ERC-1155s omit name()
		symbol, _ := GetTokenSymbol(addr)  // best-effort; many ERC-1155s omit symbol()
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
