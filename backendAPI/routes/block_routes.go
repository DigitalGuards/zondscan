package routes

import (
	"backendAPI/db"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// handleLatestBlock serves GET /latestblock.
func handleLatestBlock(c *gin.Context) {
	// Several frontend pages and external pollers hit this very often;
	// a 5 s cache trims the load by orders of magnitude with no
	// visible staleness (chain block time is much longer).
	//
	// Carries qrlUsdPrice alongside the height so the tx + block detail
	// pages that already poll this endpoint can render USD-denominated
	// fees without a second request. The price comes from the same
	// coingecko-backed cache /overview uses (sub-millisecond lookup).
	v, err := routeCache.GetOrCompute("latestblock", 5*time.Second, func() (interface{}, error) {
		blockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest block: %w", err)
		}
		num, err := parseHexBlockNumber(blockNumber)
		if err != nil {
			return nil, fmt.Errorf("invalid block number format in sync state: %w", err)
		}
		return gin.H{
			"blockNumber": num,
			"qrlUsdPrice": db.GetCurrentPrice(),
		}, nil
	})
	if err != nil {
		log.Printf("error fetching latest block: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleBlocks serves GET /blocks, the paginated block list.
func handleBlocks(c *gin.Context) {
	page, limit := getPaginationParams(c, 1, 5)
	key := fmt.Sprintf("blocks:%d:%d", page, limit)
	v, err := routeCache.GetOrCompute(key, 10*time.Second, func() (interface{}, error) {
		blocks, err := db.ReturnLatestBlocks(page, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch blocks: %w", err)
		}
		countBlocks, err := db.CountBlocksNetwork()
		if err != nil {
			return nil, fmt.Errorf("failed to count blocks: %w", err)
		}
		// Limit total pages to 300
		maxPages := int64(300)
		maxBlocks := maxPages * int64(limit)
		if countBlocks > maxBlocks {
			countBlocks = maxBlocks
		}

		// Per-block activity rollup, same data already surfaced on the
		// block detail page (iter 8). Two batched aggregations across
		// every tx hash on the page so the blocks list can render a
		// "5 token, 12 calls" hint without follow-up requests.
		allHashes := make([]string, 0)
		blockHashIndex := make(map[string][]string, len(blocks))
		for _, b := range blocks {
			hashes := make([]string, 0, len(b.Transactions))
			for _, t := range b.Transactions {
				if t.Hash != "" {
					hashes = append(hashes, t.Hash)
					allHashes = append(allHashes, t.Hash)
				}
			}
			blockHashIndex[b.Number] = hashes
		}
		lookup := txActivityCounts(allHashes, "blocks-list token counts", "blocks-list internal counts")
		// Roll per-tx counts up to per-block totals. Both counter
		// helpers store their keys as the syncer wrote them, so the
		// outer loop iterates by block to keep keys aligned with the
		// frontend lookup which is also done by block.number.
		blockActivity := make(map[string]gin.H, len(blocks))
		for blockNumber, hashes := range blockHashIndex {
			tt, ic := 0, 0
			for _, h := range hashes {
				htt, hic := lookup.counts(h)
				tt += htt
				ic += hic
			}
			if tt > 0 || ic > 0 {
				blockActivity[blockNumber] = gin.H{"tokenTransfers": tt, "internalCalls": ic}
			}
		}

		return gin.H{"blocks": blocks, "total": countBlocks, "blockActivity": blockActivity}, nil
	})
	if err != nil {
		log.Printf("error fetching blocks: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleBlock serves GET /block/:query, the block detail endpoint.
func handleBlock(c *gin.Context) {
	blockStr := c.Param("query")
	var blockNum uint64
	var err error

	// Stays on strconv (not hexutil): the strconv error text below is
	// appended to the public 400 body, so swapping the parser would
	// change a client-visible message.
	if strings.HasPrefix(blockStr, "0x") {
		// Handle hex format by removing 0x prefix
		blockNum, err = strconv.ParseUint(blockStr[2:], 16, 64)
	} else {
		// Handle decimal format
		blockNum, err = strconv.ParseUint(blockStr, 10, 64)
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid block number. Please provide a decimal number or hex with 0x prefix: " + err.Error(),
		})
		return
	}

	block, err := db.ReturnSingleBlock(blockNum)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Per-tx activity counts: each row in the block's tx table can
	// surface "moved N tokens" + "made M internal calls" without a
	// follow-up /tx/:hash hit per row. Two scoped Mongo aggregations
	// per block fan in to a (hash; counts) map the client looks up
	// by row hash.
	txHashes := make([]string, 0, len(block.Result.Transactions))
	for _, t := range block.Result.Transactions {
		if t.Hash != "" {
			txHashes = append(txHashes, t.Hash)
		}
	}
	lookup := txActivityCounts(txHashes,
		fmt.Sprintf("token counts block %d", blockNum),
		fmt.Sprintf("internal counts block %d", blockNum))
	activity := make(map[string]gin.H, len(txHashes))
	for _, h := range txHashes {
		tt, ic := lookup.counts(h)
		if tt > 0 || ic > 0 {
			activity[h] = gin.H{"tokenTransfers": tt, "internalCalls": ic}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"block":      block,
		"txActivity": activity,
	})
}

// handleBlockSizes serves GET /blocksizes.
func handleBlockSizes(c *gin.Context) {
	// Block-size aggregate is a precomputed collection refreshed by the
	// syncer's periodic task, safe to cache for longer.
	v, err := routeCache.GetOrCompute("blocksizes", 30*time.Second, func() (interface{}, error) {
		query, err := db.ReturnBlockSizes()
		if err != nil {
			log.Printf("error fetching block sizes: %v", err)
		}
		return gin.H{"response": query}, nil
	})
	if err != nil {
		log.Printf("error fetching block sizes: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleCoinbase serves GET /coinbase/:query.
func handleCoinbase(c *gin.Context) {
	value, ok := requireTxHashParam(c, "query")
	if !ok {
		return
	}
	query, err := db.ReturnSingleTransfer(value)
	// A missing tx is not a server fault: keep the historical 200-with-empty
	// behavior for not-found. Any other DB error becomes a 500 instead of a
	// misleading 200.
	if err != nil && err != mongo.ErrNoDocuments {
		log.Printf("error fetching coinbase transfer %s: %v", value, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch transfer"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"response": query})
}
