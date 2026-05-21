package routes

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"backendAPI/db"
	"backendAPI/middleware"

	"github.com/gin-gonic/gin"
)

// contract-call gas cap. The node treats this as "if your call gas-uses
// more than X, fail", a hard ceiling against pathological reads that
// would tie up the node. 50M is comfortably above any real read flow.
const contractCallGasCap = uint64(50_000_000)

// Maximum size of the hex `data` field. Generous enough for any realistic
// ABI-encoded calldata (a function selector + tens of dynamic args), but
// small enough that abuse is bounded.
const contractCallMaxDataBytes = 8 * 1024 // 8 KiB raw hex = ~4 KiB decoded

// RegisterContractCallRoute wires POST /contract/call. Distinct file from
// verification_routes.go to keep the M4 surface obviously separable from
// the verifier surface for review purposes.
//
// The route is effectively a typed `eth_call` proxy: the same network
// primitive Etherscan uses to back its "Read Contract" tab. Open-ended
// reads of view/pure functions are read-only and have no on-chain side
// effects, but the proxy must still cap (a) the request rate per IP, (b)
// the data payload size, (c) the gas cap, and (d) the per-call timeout ,
// all together, those bound how much node work an attacker can elicit.
func RegisterContractCallRoute(router *gin.Engine) {
	// 60 calls/min/IP, generous for a Read tab where each click on a view
	// function fires one call. The token bucket bursts at 60 too.
	callLimiter := middleware.PerIPRateLimit(60, 60, time.Minute)

	router.POST("/contract/call", callLimiter, func(c *gin.Context) {
		// Cap the request body so a malicious huge payload can't tie up
		// the JSON decoder before our size checks kick in.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024) // 32 KiB

		var req struct {
			To   string `json:"to" binding:"required"`
			Data string `json:"data" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
			return
		}

		req.To = strings.TrimSpace(req.To)
		req.Data = strings.ToLower(strings.TrimSpace(req.Data))

		if !isValidAddress(req.To) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Q-prefixed `to` address"})
			return
		}
		if !isValidHex(req.Data) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "`data` must be hex (0x…) with even length"})
			return
		}
		if len(req.Data) > contractCallMaxDataBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "`data` exceeds size cap", "max": contractCallMaxDataBytes})
			return
		}

		// Refuse to proxy reads of addresses that haven't been observed as
		// contracts. This is the only gate stopping the endpoint from
		// being a free open eth_call: querying arbitrary node state via a
		// non-contract `to` would still work but offers no useful surface
		// to an attacker (returns 0x). Keeping it scoped to known contracts
		// also lets us bound the audit story.
		known, err := db.ReturnContractCode(req.To)
		if err != nil {
			// Log unexpected database failures so they don't silently
			// hide behind the public 404 response.
			log.Printf("contract_call: ReturnContractCode(%s) failed: %v", req.To, err)
		}
		if err != nil || known.ContractAddress == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "address is not a known contract"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
		defer cancel()

		callObj := map[string]interface{}{
			"to":   req.To,
			"data": req.Data,
			// Pinned gas cap. The node may impose its own ceiling too;
			// having ours guarantees we never relay an uncapped read.
			"gas": uintToHex(contractCallGasCap),
		}

		raw, rpcErr, err := db.NodeRPC(ctx, "qrl_call", []interface{}{callObj, "latest"})
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "node call failed: " + err.Error()})
			return
		}
		if rpcErr != nil {
			// Surface the upstream error verbatim, "execution reverted"
			// is the most common case and clients (Read tab) display it.
			c.JSON(http.StatusOK, gin.H{
				"error":   rpcErr.Message,
				"code":    rpcErr.Code,
				"data":    rpcErr.Data,
				"reverted": true,
			})
			return
		}

		var result string
		if err := json.Unmarshal(raw, &result); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "could not decode node response"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result})
	})
}

// isValidHex returns true for "0x"-prefixed even-length strings of hex
// digits. Empty payloads (0x by itself) are allowed since some view
// functions take no arguments and call data is just the 4-byte selector.
func isValidHex(s string) bool {
	if !strings.HasPrefix(s, "0x") {
		return false
	}
	hex := s[2:]
	if len(hex)%2 != 0 {
		return false
	}
	for _, r := range hex {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// uintToHex formats a uint64 as an 0x-prefixed hex string (no leading
// zeros, lowercase). Used for the `gas` field in the qrl_call params.
func uintToHex(n uint64) string {
	if n == 0 {
		return "0x0"
	}
	const hexDigits = "0123456789abcdef"
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = hexDigits[n&0xf]
		n >>= 4
	}
	return "0x" + string(buf[i:])
}
