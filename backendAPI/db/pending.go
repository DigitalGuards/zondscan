package db

import (
	"backendAPI/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const DEFAULT_PAGE_SIZE = 10

func GetPendingTransactions(page, limit int) (*models.PaginatedPendingTransactions, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DEFAULT_PAGE_SIZE
	}

	transactions, err := fetchPendingTransactions()
	if err != nil {
		return nil, err
	}

	// Calculate pagination
	total := len(transactions)
	totalPages := (total + limit - 1) / limit
	startIndex := (page - 1) * limit
	endIndex := startIndex + limit
	if endIndex > total {
		endIndex = total
	}

	// Get paginated subset
	var paginatedTxs []models.PendingTransaction
	if startIndex < total {
		paginatedTxs = transactions[startIndex:endIndex]
	}

	return &models.PaginatedPendingTransactions{
		Transactions: paginatedTxs,
		Total:        total,
		Page:         page,
		Limit:        limit,
		TotalPages:   totalPages,
	}, nil
}

func GetPendingTransactionByHash(hash string) (*models.PendingTransaction, error) {
	transactions, err := fetchPendingTransactions()
	if err != nil {
		return nil, err
	}

	// Look for the transaction with matching hash
	for _, tx := range transactions {
		if tx.Hash == hash {
			return &tx, nil
		}
	}

	return nil, nil
}

// fetchPendingTransactions gets all pending transactions from the node
func fetchPendingTransactions() ([]models.PendingTransaction, error) {
	// Create JSON-RPC request
	rpcReq := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "txpool_content",
		Params:  []interface{}{},
		ID:      1,
	}

	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Get node URL from environment variable or use default
	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		nodeURL = "http://REDACTED:8545" // fallback to default if not set
	}

	// Create and send HTTP request
	req, err := http.NewRequest("POST", nodeURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Parse response
	var rpcResp models.PendingTransactionsResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	// Convert response to slice of transactions
	var allTransactions []models.PendingTransaction
	if rpcResp.Result.Pending != nil {
		for address, nonceTxs := range rpcResp.Result.Pending {
			for nonce, tx := range nonceTxs {
				tx.From = address  // Add the from address
				tx.Nonce = nonce   // Add the nonce
				allTransactions = append(allTransactions, tx)
			}
		}
	}

	return allTransactions, nil
}
