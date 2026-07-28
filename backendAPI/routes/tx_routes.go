package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// receiptLog mirrors the subset of qrl_getTransactionReceipt.logs[] the tx
// page actually consumes. address + topics + data + logIndex is enough for
// the frontend's Event Logs panel to decode all token-flavoured signatures
// (Transfer / TransferSingle / TransferBatch / Approval / ApprovalForAll)
// and render a raw fallback for everything else. Everything else on the
// receipt (block hash, tx index, etc.) is already known to the page.
type receiptLog struct {
	Address  string   `json:"address"`
	Topics   []string `json:"topics"`
	Data     string   `json:"data"`
	LogIndex string   `json:"logIndex"`
	Removed  bool     `json:"removed"`
}

// receiptSummary is the subset of a tx receipt the /tx/:hash route
// surfaces alongside the events. Status is the EVM revert flag
// ("0x1" success, "0x0" reverted); the syncer's Transaction.Status
// field is empty for every historical row, so we resolve it from
// the live receipt here. Same RPC call that already fetches logs.
type receiptSummary struct {
	Status string `json:"status"`
	// GasUsed is the actual gas consumed (hex), distinct from the tx's gas
	// limit. ReturnSingleTransfer fills Transfer.GasUsed from the block's
	// `gas` field (the LIMIT) because the block doc has no receipt data; the
	// /tx route overrides it with this value when the receipt fetch succeeds.
	GasUsed string       `json:"gasUsed"`
	Logs    []receiptLog `json:"logs"`
}

// fetchReceipt pulls a tx's receipt over JSON-RPC and returns the
// caller-facing subset (status + logs). Best-effort: if the node is
// unreachable or the receipt hasn't been indexed yet, returns an
// empty struct without an error so the tx page still renders. Errors
// are logged at the call site.
func fetchReceipt(ctx context.Context, txHash string) receiptSummary {
	raw, rpcErr, transportErr := db.NodeRPC(ctx, "qrl_getTransactionReceipt", []interface{}{txHash})
	if transportErr != nil {
		log.Printf("receipt fetch %s: %v", txHash, transportErr)
		return receiptSummary{}
	}
	if rpcErr != nil {
		log.Printf("receipt fetch %s: rpc error: %s", txHash, rpcErr.Error())
		return receiptSummary{}
	}
	if len(raw) == 0 || string(raw) == "null" {
		return receiptSummary{}
	}
	var summary receiptSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		log.Printf("receipt decode %s: %v", txHash, err)
		return receiptSummary{}
	}
	return summary
}

// fetchTxInput pulls a tx's calldata from the node. The historical block
// docs in MongoDB carry an empty `data` field because the syncer's
// Transaction struct uses the wrong JSON tag (`data`, where the node
// returns `input`), so we resolve it over RPC instead. Best-effort:
// returns "" when the node is unreachable so the page still renders.
func fetchTxInput(ctx context.Context, txHash string) string {
	raw, rpcErr, transportErr := db.NodeRPC(ctx, "qrl_getTransactionByHash", []interface{}{txHash})
	if transportErr != nil {
		log.Printf("tx input fetch %s: %v", txHash, transportErr)
		return ""
	}
	if rpcErr != nil {
		log.Printf("tx input fetch %s: rpc error: %s", txHash, rpcErr.Error())
		return ""
	}
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var tx struct {
		Input string `json:"input"`
	}
	if err := json.Unmarshal(raw, &tx); err != nil {
		log.Printf("tx input decode %s: %v", txHash, err)
		return ""
	}
	return tx.Input
}

// handlePendingTransactions serves GET /pending-transactions with pagination.
func handlePendingTransactions(c *gin.Context) {
	page, limit := getPaginationParams(c, 1, 10)
	key := fmt.Sprintf("pending-tx:%d:%d", page, limit)
	// Frontend polls this every 30 s × every user. 5 s TTL absorbs the
	// herd; mempool freshness only suffers by at most 5 s.
	v, err := routeCache.GetOrCompute(key, 5*time.Second, func() (interface{}, error) {
		result, err := db.GetPendingTransactions(page, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch pending transactions: %w", err)
		}
		if result.Transactions == nil {
			result.Transactions = make([]models.PendingTransaction, 0)
		}
		return result, nil
	})
	if err != nil {
		log.Printf("Error fetching pending transactions: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handlePendingTransaction serves GET /pending-transaction/:hash, fetching
// a specific pending transaction.
func handlePendingTransaction(c *gin.Context) {
	hash, ok := requireTxHashParam(c, "hash")
	if !ok {
		return
	}
	transaction, err := db.GetPendingTransactionByHash(hash)
	if err != nil {
		log.Printf("error fetching pending transaction %s: %v", hash, err)
		respondInternal(c)
		return
	}

	// If not found in pending, check if it's in the transactions collection
	if transaction == nil {
		tx, err := db.GetTransactionByHash(hash)
		if err != nil {
			log.Printf("error fetching transaction %s: %v", hash, err)
			respondInternal(c)
			return
		}

		// Mined tx whose tombstone was already swept, return 404 in the
		// same shape as the tombstone branch below. The frontend treats
		// any non-200 from this endpoint as "not pending" and fetches
		// /tx/:hash for the mined payload, so returning the mined details
		// here just duplicates work and creates a state-dependent API.
		if tx != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Transaction has been mined",
				"status":  "mined",
				"details": "This transaction has been confirmed. Please view it as a confirmed transaction.",
			})
			return
		}

		// Transaction not found in either collection
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Transaction not found",
			"details": "This transaction is no longer in the mempool. It may have been dropped or replaced.",
		})
		return
	}

	// If the row is a tombstone for a mined tx, return 404, let the
	// frontend fall back to /tx/<hash>. Don't delete here: the tombstone
	// protects against a stale mempool poll re-inserting the row as
	// "pending". CleanupOldPendingTransactions sweeps it after maxAge.
	if transaction.Status == "mined" {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Transaction has been mined",
			"status":  "mined",
			"details": "This transaction has been confirmed. Please view it as a confirmed transaction.",
		})
		return
	}

	response := gin.H{"transaction": transaction}

	// Surface verified-contract metadata for the recipient when one
	// exists; mirrors the /tx/:hash plumbing so the pending view can
	// use the same ABI-driven calldata decoder as the confirmed view.
	// One row lookup per pending detail render, no batch needed
	// because there's only one address to resolve.
	if transaction.To != "" {
		contracts, terr := db.GetContractsByAddresses([]string{transaction.To})
		if terr != nil {
			log.Printf("pending target contract lookup %s: %v", hash, terr)
		} else if meta := contractInfoPayload(contracts, transaction.To); meta != nil {
			// contracts map is keyed by canonical Q+lowercase hex (the
			// syncer's storage form). The previous strings.ToLower path
			// produced "0x..." or "q..." keys that never matched.
			response["targetContract"] = meta
		}
	}

	c.JSON(http.StatusOK, response)
}

// handleTxs serves GET /txs, the paginated network-wide transaction list.
func handleTxs(c *gin.Context) {
	page, limit := getPaginationParams(c, 1, 10)

	key := fmt.Sprintf("txs:%d:%d", page, limit)
	v, err := routeCache.GetOrCompute(key, 10*time.Second, func() (interface{}, error) {
		txs, err := db.ReturnTransactionsNetwork(page, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch transactions: %w", err)
		}
		countTransactions, err := db.CountTransactionsNetwork()
		if err != nil {
			return nil, fmt.Errorf("failed to count transactions: %w", err)
		}
		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		latestBlockNum, err := parseHexBlockNumber(latestBlockNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to parse block number: %w", err)
		}
		if txs == nil {
			txs = make([]models.TransactionByAddress, 0)
		}
		return gin.H{
			"txs":         txs,
			"total":       countTransactions,
			"latestBlock": latestBlockNum,
		}, nil
	})
	if err != nil {
		log.Printf("error fetching transactions: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleTx serves GET /tx/:query, the tx detail endpoint.
func handleTx(c *gin.Context) {
	value, ok := requireTxHashParam(c, "query")
	if !ok {
		return
	}
	query, err := db.ReturnSingleTransfer(value)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		// A real db failure must surface as a retryable 500: it used to be
		// swallowed into the 404 branch below, so a Mongo outage rendered
		// as a permanent "transaction not found" (and got cached as such
		// by the frontend's notFound() mapping).
		log.Printf("error fetching transfer %s: %v", value, err)
		respondInternal(c)
		return
	}

	// Detect not-found via an empty TxHash (mongo.ErrNoDocuments or a hash
	// missing from its block) and return 404 instead of a misleading 200
	// with empty fields. The frontend tx page maps 404 to notFound().
	if query.TxHash == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	latestBlockNumber, err := db.GetLatestBlockFromSyncState()
	if err != nil {
		log.Printf("error getting latest block: %v", err)
		respondInternal(c)
		return
	}

	latestBlockNum, err := parseHexBlockNumber(latestBlockNumber)
	if err != nil {
		log.Printf("error parsing latest block number %q: %v", latestBlockNumber, err)
		respondInternal(c)
		return
	}

	// Check if this transaction created a contract
	contractCreated, err := db.GetContractByCreationTx(value)
	if err != nil {
		log.Printf("Error checking for contract creation tx %s: %v", value, err)
	}

	// Check if this transaction is a token transfer. A single tx can
	// emit multiple Transfer events (DEX swaps, ERC-1155 TransferBatch
	// fan-out), so we surface them all as an array.
	tokenTransfers, err := db.GetTokenTransfersByTxHash(value)
	if err != nil {
		log.Printf("Error checking for token transfers tx %s: %v", value, err)
	}

	// Pull any internal-call entries the syncer captured under this
	// tx. Simple value transfers don't have any; complex contract
	// calls with delegate / static / call sub-frames will.
	internalTxs, err := db.GetInternalTransactionsByTxHash(value)
	if err != nil {
		log.Printf("Error checking for internal txs %s: %v", value, err)
	}

	// Receipt logs power the Event Logs panel on the tx page; tx input
	// powers the Input Data card. Best-effort RPC fetches in parallel
	// with a shared 6s budget so the page still renders if the node is
	// briefly unreachable. Tx input has to come from the node because
	// the syncer's Transaction struct uses the wrong JSON tag (`data`
	// where the node returns `input`), leaving the persisted field
	// empty for every historical tx.
	rpcCtx, cancelRPC := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancelRPC()
	var receipt receiptSummary
	var txInput string
	var rpcWG sync.WaitGroup
	rpcWG.Add(2)
	go func() {
		defer rpcWG.Done()
		receipt = fetchReceipt(rpcCtx, value)
	}()
	go func() {
		defer rpcWG.Done()
		txInput = fetchTxInput(rpcCtx, value)
	}()
	rpcWG.Wait()
	logs := receipt.Logs

	// ReturnSingleTransfer fills query.GasUsed from the block's gas LIMIT
	// (the block doc carries no receipt). Override it with the actual gas
	// consumed from the receipt when present so the tx page shows the real
	// figure. For a plain transfer this equals the limit (21000), so the
	// simple-transfer case is unaffected.
	if receipt.GasUsed != "" {
		query.GasUsed = receipt.GasUsed
	}

	// Attach per-log + per-target contract metadata so the frontend
	// can decode unknown event signatures + method selectors when the
	// contract has been source-verified. One batched lookup covers
	// every log address + the tx's target address; addresses without
	// a contractCode row simply don't appear in the map and the
	// frontend falls back to the raw render.
	contractAddrs := make([]string, 0, len(logs)+1)
	for _, l := range logs {
		if l.Address != "" {
			contractAddrs = append(contractAddrs, l.Address)
		}
	}
	if query.To != "" {
		contractAddrs = append(contractAddrs, query.To)
	}
	contractsByAddr, err := db.GetContractsByAddresses(contractAddrs)
	if err != nil {
		log.Printf("contracts-by-addresses lookup tx %s: %v", value, err)
		contractsByAddr = map[string]models.ContractInfo{}
	}

	response := gin.H{
		"response":    query,
		"latestBlock": latestBlockNum,
	}
	// Receipt status is "0x1" on success, "0x0" on revert. Surface it
	// so the frontend can render a "Reverted" badge instead of falsely
	// labelling a failed-but-confirmed tx as confirmed. Empty when
	// the RPC fetch failed; the frontend treats absent as success
	// (matches the existing fallback).
	if receipt.Status != "" {
		response["receiptStatus"] = receipt.Status
	}
	if len(logs) > 0 {
		// Re-emit logs with the optional contract field attached so the
		// frontend has everything it needs in one pass.
		emitted := make([]gin.H, 0, len(logs))
		for _, l := range logs {
			entry := gin.H{
				"address":  l.Address,
				"topics":   l.Topics,
				"data":     l.Data,
				"logIndex": l.LogIndex,
				"removed":  l.Removed,
			}
			if ci := contractInfoPayload(contractsByAddr, l.Address); ci != nil {
				entry["contract"] = ci
			}
			emitted = append(emitted, entry)
		}
		response["logs"] = emitted
	}
	if txInput != "" && txInput != "0x" {
		response["input"] = txInput
		// targetContract carries the same shape as log.contract, used
		// by the frontend Input Data card to decode the method
		// selector + args off the target's ABI.
		if ci := contractInfoPayload(contractsByAddr, query.To); ci != nil {
			response["targetContract"] = ci
		}
	}

	if contractCreated != nil {
		response["contractCreated"] = gin.H{
			"address":       contractCreated.ContractAddress,
			"isToken":       contractCreated.IsToken,
			"name":          contractCreated.TokenName,
			"symbol":        contractCreated.TokenSymbol,
			"decimals":      contractCreated.TokenDecimals,
			"tokenStandard": contractCreated.TokenStandard,
		}
	}

	if len(tokenTransfers) > 0 {
		rows := make([]gin.H, 0, len(tokenTransfers))
		for _, t := range tokenTransfers {
			rows = append(rows, gin.H{
				"contractAddress": t.ContractAddress,
				"from":            t.From,
				"to":              t.To,
				"amount":          t.Amount,
				"tokenName":       t.TokenName,
				"tokenSymbol":     t.TokenSymbol,
				"tokenDecimals":   t.TokenDecimals,
				"tokenStandard":   t.TokenStandard,
				"tokenID":         t.TokenID,
				"logIndex":        t.LogIndex,
			})
		}
		response["tokenTransfers"] = rows
	}

	if len(internalTxs) > 0 {
		rows := make([]gin.H, 0, len(internalTxs))
		for _, t := range internalTxs {
			rows = append(rows, gin.H{
				"type":         t.Type,
				"callType":     t.CallType,
				"from":         t.From,
				"to":           t.To,
				"input":        t.Input,
				"output":       t.Output,
				"value":        t.Value,
				"gas":          t.Gas,
				"gasUsed":      t.GasUsed,
				"traceAddress": t.TraceAddress,
			})
		}
		response["internalTransactions"] = rows
	}

	c.JSON(http.StatusOK, response)
}

// handleTransactions serves GET /transactions.
func handleTransactions(c *gin.Context) {
	// Network-wide latest-transactions feed. Hot path on the homepage ,
	// every visitor's 30s refresh hits this. Cache for 5s so the burst
	// fans into one Mongo round-trip.
	v, err := routeCache.GetOrCompute("latest-txs", 5*time.Second, func() (interface{}, error) {
		query, qerr := db.ReturnLatestTransactions()
		if qerr != nil {
			// Return the error so the cache doesn't store an empty
			// payload, the next caller will retry against Mongo
			// instead of being served stale junk for 5s.
			log.Printf("error fetching latest transactions: %v", qerr)
			return nil, qerr
		}
		return gin.H{"response": query}, nil
	})
	if err != nil {
		log.Printf("error fetching latest transactions: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handlePendingTxETA serves GET /pending-tx-eta/:hash, a best-effort
// inclusion ETA for one pending tx, plus the supporting numbers (avg block
// time, avg gas usage, mempool size, median gasPrice, gas-ahead sum). The
// math is in db/gas.go; the route just plumbs it together.
func handlePendingTxETA(c *gin.Context) {
	hash, ok := requireTxHashParam(c, "hash")
	if !ok {
		return
	}
	v, err := routeCache.GetOrCompute("eta:"+hash, 5*time.Second, func() (interface{}, error) {
		tx, err := db.GetPendingTransactionByHash(hash)
		if err != nil {
			return nil, err
		}
		if tx == nil || tx.Status != "pending" {
			return nil, nil
		}

		samples, err := db.GetRecentBlockSamples(30)
		if err != nil {
			return nil, err
		}
		blockStats := db.ComputeBlockStats(samples)

		mempool, err := db.GetPendingMempoolSnapshot()
		if err != nil {
			return nil, err
		}
		mp := db.ComputeMempoolStatsFor(mempool, tx.GasPrice)

		// ETA = ceil(gasAhead / avgGasUsed) * avgBlockTime.
		avgGas := blockStats.AvgGasUsed
		if avgGas.Sign() <= 0 {
			avgGas = big.NewInt(1)
		}
		// Integer ceil-division: (a + b - 1) / b.
		num := new(big.Int).Add(mp.GasAhead, new(big.Int).Sub(avgGas, big.NewInt(1)))
		blocksAhead := new(big.Int).Quo(num, avgGas).Int64()
		etaSec := float64(blocksAhead) * blockStats.AvgBlockTimeSec
		// At minimum one block-time of wait, gasAhead = 0 still means we
		// wait for the next block to be produced.
		if etaSec < blockStats.AvgBlockTimeSec {
			etaSec = blockStats.AvgBlockTimeSec
		}

		return gin.H{
			"etaSec":            etaSec,
			"avgBlockTimeSec":   blockStats.AvgBlockTimeSec,
			"avgGasUsedHex":     "0x" + blockStats.AvgGasUsed.Text(16),
			"pendingCount":      mp.Count,
			"gasAheadHex":       "0x" + mp.GasAhead.Text(16),
			"medianGasPriceHex": "0x" + mp.MedianGasPrice.Text(16),
			"yourGasPriceHex":   tx.GasPrice,
		}, nil
	})
	if err != nil {
		log.Printf("error computing pending-tx eta %s: %v", hash, err)
		respondInternal(c)
		return
	}
	if v == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pending transaction not found"})
		return
	}
	c.JSON(http.StatusOK, v)
}
