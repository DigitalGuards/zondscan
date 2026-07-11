package routes

import (
	"backendAPI/db"
	"backendAPI/hexutil"
	"backendAPI/models"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseHexBlockNumber parses a "0x"-prefixed hex string into a uint64.
// The strict-prefix check and its "invalid hex prefix" message are local
// (every caller wraps the error into a log line, never a response body);
// digit parsing delegates to hexutil. Real block numbers fit comfortably
// in int64, so the int64-range parser is safe here.
func parseHexBlockNumber(hexStr string) (uint64, error) {
	if !strings.HasPrefix(hexStr, "0x") {
		return 0, fmt.Errorf("invalid hex prefix")
	}
	v, err := hexutil.ParseInt64(hexStr)
	if err != nil {
		return 0, err
	}
	return uint64(v), nil
}

// respondInternal writes the generic 500 body shared by the read routes.
// The public string is a fixed lowercase "internal server error"; details
// belong in the server log at the call site, never in the response.
func respondInternal(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// requireAddressParam reads the named path param and validates it with
// isValidAddressParam, emitting the shared 400 body on failure. When ok
// is false the response has already been written and the handler must
// return without writing anything else.
func requireAddressParam(c *gin.Context, name string) (string, bool) {
	v := c.Param(name)
	if !isValidAddressParam(v) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address; expected Q or 0x followed by 40 hex chars"})
		return "", false
	}
	return v, true
}

// requireTxHashParam is the tx-hash counterpart of requireAddressParam.
func requireTxHashParam(c *gin.Context, name string) (string, bool) {
	v := c.Param(name)
	if !isValidTxHash(v) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction hash; expected 0x + 64 hex chars"})
		return "", false
	}
	return v, true
}

// parseStandardFilter validates the optional `standard` query param
// against the allowed set. An absent param returns ("", true); a valid
// value returns (value, true); an invalid value writes the 400 response
// and returns ("", false), after which the handler must return. The
// message keeps the historical per-route wording: "expected X or Y" for
// two options, "expected one of X, Y, Z" otherwise.
func parseStandardFilter(c *gin.Context, allowed ...string) (string, bool) {
	s := c.Query("standard")
	if s == "" {
		return "", true
	}
	for _, a := range allowed {
		if s == a {
			return s, true
		}
	}
	var expected string
	if len(allowed) == 2 {
		expected = allowed[0] + " or " + allowed[1]
	} else {
		expected = "one of " + strings.Join(allowed, ", ")
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"error": "invalid standard; expected " + expected,
	})
	return "", false
}

// contractInfoPayload builds the gin.H wrapper exposed to the frontend
// for a single contract, resolved from a db.GetContractsByAddresses
// result map. The map is keyed by canonical Q+lowercase hex (the shape
// db.GetContractsByAddresses returns from c.ContractAddress), so the
// incoming addr is normalised to that shape before lookup; matches would
// otherwise be missed when a log carries the address in 0x or
// lowercase-q form. The ABI is included only when the contract is
// verified, that's the only case where the frontend can trust the ABI to
// be authoritative. Returns nil when there's no entry for this address so
// the caller can omit the field entirely.
func contractInfoPayload(contractsByAddr map[string]models.ContractInfo, addr string) gin.H {
	c, ok := contractsByAddr[normalizeContractAddr(addr)]
	if !ok || c.ContractAddress == "" {
		return nil
	}
	payload := gin.H{
		"name":          c.TokenName,
		"symbol":        c.TokenSymbol,
		"tokenStandard": c.TokenStandard,
		"verified":      c.Verified,
		"contractName":  c.ContractName,
	}
	if c.Verified && c.Abi != "" {
		payload["abi"] = c.Abi
	}
	return payload
}

// txActivityCounts runs the two batched activity aggregations
// (tokenTransfers rows + internal-call rows per tx hash) over txHashes
// and returns a per-hash lookup. Aggregation failures are absorbed, the
// failing side falls back to an empty map so the page still renders;
// logLabelToken / logLabelInternal carry the call site's original log
// wording for that case.
func txActivityCounts(txHashes []string, logLabelToken, logLabelInternal string) txActivityLookup {
	tokenCounts, terr := db.CountTokenTransfersByTxHashes(txHashes)
	if terr != nil {
		log.Printf("%s: %v", logLabelToken, terr)
		tokenCounts = map[string]int{}
	}
	internalCounts, ierr := db.CountInternalTxsByTxHashes(txHashes)
	if ierr != nil {
		log.Printf("%s: %v", logLabelInternal, ierr)
		internalCounts = map[string]int{}
	}
	return txActivityLookup{tokenCounts: tokenCounts, internalCounts: internalCounts}
}

// txActivityLookup is the per-hash counting core shared by the /blocks
// list rollup and the /block/:query per-tx activity map.
type txActivityLookup struct {
	tokenCounts    map[string]int
	internalCounts map[string]int
}

// counts returns the token-transfer + internal-call counts for one tx
// hash. CountTokenTransfersByTxHashes normalises its keys to lowercase
// 0x...; internalTransactionByAddress is written from trace data that may
// not share casing with the block's tx.Hash field. Both lookups therefore
// try the exact key first and fall back to the lowercase form so we don't
// undercount. (The aggregations only ever emit counts >= 1, so the
// ok-check and zero-check phrasings the two original call sites used are
// equivalent; this is the shared form.)
func (l txActivityLookup) counts(h string) (tt, ic int) {
	lowerH := strings.ToLower(h)
	if c, ok := l.tokenCounts[h]; ok {
		tt = c
	} else if c, ok := l.tokenCounts[lowerH]; ok {
		tt = c
	}
	if c, ok := l.internalCounts[h]; ok {
		ic = c
	} else if c, ok := l.internalCounts[lowerH]; ok {
		ic = c
	}
	return tt, ic
}
