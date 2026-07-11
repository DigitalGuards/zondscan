package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleContractsCounts serves GET /contracts/counts, the four tab buckets
// the contracts page shows in one aggregation. Cached 30s; the underlying
// numbers shift only as new contracts are deployed (block-paced, slow).
func handleContractsCounts(c *gin.Context) {
	v, err := routeCache.GetOrCompute("contracts:counts", 30*time.Second, func() (interface{}, error) {
		counts, err := db.GetContractCountsByStandard()
		if err != nil {
			return nil, fmt.Errorf("failed to count contracts by standard: %w", err)
		}
		return counts, nil
	})
	if err != nil {
		log.Printf("error counting contracts by standard: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleContracts serves GET /contracts, the paginated contract list.
func handleContracts(c *gin.Context) {
	pageInt, limitInt := getPaginationParams(c, 0, 10)
	page := int64(pageInt)
	limit := int64(limitInt)
	search := c.Query("search")

	// Parse isToken filter (optional)
	var isTokenFilter *bool
	if isTokenParam := c.Query("isToken"); isTokenParam != "" {
		isToken := isTokenParam == "true"
		isTokenFilter = &isToken
	}

	// Parse standard filter (optional). Validates against the known
	// set to keep the BSON query well-typed and prevent arbitrary
	// strings from being passed as a filter predicate.
	standard, ok := parseStandardFilter(c, "ERC-20", "ERC-721", "ERC-1155")
	if !ok {
		return
	}
	var standardFilter *string
	if standard != "" {
		standardFilter = &standard
	}

	query, total, err := db.ReturnContracts(page, limit, search, isTokenFilter, standardFilter)
	if err != nil {
		log.Printf("error fetching contracts: %v", err)
		respondInternal(c)
		return
	}

	// Addresses are stored and returned as q-prefix (canonical form).
	// No presentation-layer conversion needed.

	c.JSON(http.StatusOK, gin.H{
		"response": query,
		"total":    total,
	})
}

// handleTokenInfo serves GET /token/:address/info, summary stats for a
// token contract.
func handleTokenInfo(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}

	info, err := db.GetTokenInfo(address)
	if err != nil {
		log.Printf("Error fetching token info for %s: %v", address, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Token not found",
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

// handleTokenHolders serves GET /token/:address/holders with pagination.
// Phase 2: optional `?tokenID=` query param filters to holders of a
// specific NFT tokenID. When absent, holders are aggregated across all
// ids (one row per distinct holder, balance is the cross-id total).
// ERC-20 behaviour unchanged.
func handleTokenHolders(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}
	tokenID := c.Query("tokenID")
	page, limit := getPaginationParams(c, 0, 25)

	holders, totalCount, err := db.GetTokenHolders(address, tokenID, page, limit)
	if err != nil {
		log.Printf("Error fetching token holders for %s (tokenID=%q): %v", address, tokenID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch token holders",
		})
		return
	}

	c.JSON(http.StatusOK, models.TokenHoldersResponse{
		ContractAddress: address,
		Holders:         holders,
		TotalHolders:    totalCount,
		Page:            page,
		Limit:           limit,
	})
}

// handleTokenIDs serves GET /token/:address/tokens. Phase 2: list distinct
// tokenIDs minted on an NFT contract, with the number of holders for each
// id. Paginated by `?page=&limit=` like the other token endpoints.
// Returns an empty list for ERC-20 contracts. Phase 3b: each row also
// carries `name` + `image` + `description` when off-chain metadata has
// been fetched.
func handleTokenIDs(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}
	page, limit := getPaginationParams(c, 0, 25)

	tokens, totalCount, err := db.GetTokenIDs(address, page, limit)
	if err != nil {
		log.Printf("Error fetching tokenIDs for %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch tokenIDs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contractAddress": address,
		"tokens":          tokens,
		"totalTokenIDs":   totalCount,
		"page":            page,
		"limit":           limit,
	})
}

// handleTokenMetadata serves GET /token/:address/:id. Phase 3b: per-token
// off-chain metadata. Returns the full metadata document for one
// (contract, tokenID), including OpenSea-style attributes. 404 if no stub
// exists (e.g. the contract isn't an NFT or that id has never been
// transferred). 400 if `id` isn't a decimal integer, this also guards
// against accidental collision with the sibling static-segment routes
// (/info, /holders, etc), although Gin's router already gives them
// priority by tree shape.
func handleTokenMetadata(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}
	tokenID := c.Param("id")

	// Cap the length before SetString: a uint256 token ID is at most 78
	// decimal digits, so anything longer is invalid and parsing an
	// arbitrarily long string into big.Int is a CPU-DoS vector.
	if len(tokenID) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tokenID is too large"})
		return
	}
	if _, ok := new(big.Int).SetString(tokenID, 10); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tokenID must be a decimal integer"})
		return
	}

	meta, err := db.GetTokenMetadata(address, tokenID)
	if err != nil {
		log.Printf("Error fetching token metadata for %s/%s: %v", address, tokenID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch token metadata"})
		return
	}
	if meta == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}
	c.JSON(http.StatusOK, meta)
}

// handleTokenTransfers serves GET /token/:address/transfers with pagination.
func handleTokenTransfers(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}
	page, limit := getPaginationParams(c, 0, 25)

	transfers, totalCount, err := db.GetTokenTransfers(address, page, limit)
	if err != nil {
		log.Printf("Error fetching token transfers for %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch token transfers",
		})
		return
	}

	c.JSON(http.StatusOK, models.TokenTransfersResponse{
		ContractAddress: address,
		Transfers:       transfers,
		TotalTransfers:  totalCount,
		Page:            page,
		Limit:           limit,
	})
}
