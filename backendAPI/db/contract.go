package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnContracts(page int64, limit int64, search string, isTokenFilter *bool) ([]models.ContractInfo, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the main model directly, it now has correct BSON tags
	var contracts []models.ContractInfo

	// Base filter
	filter := bson.D{}

	// Add isToken filter if specified
	if isTokenFilter != nil {
		filter = append(filter, bson.E{Key: "isToken", Value: *isTokenFilter})
	}

	// Add search if provided, using correct field names
	if search != "" {
		// Normalize the search address to canonical Q-prefix form
		normalizedSearch := normalizeAddress(search)

		// Zond addresses start with 'Q'. Search by normalized address or token name.
		searchFilter := bson.D{
			{Key: "$or", Value: bson.A{
				bson.D{{Key: "address", Value: normalizedSearch}},        // Match contract address
				bson.D{{Key: "creatorAddress", Value: normalizedSearch}}, // Match creator address
				bson.D{{Key: "name", Value: bson.D{{Key: "$regex", Value: search}, {Key: "$options", Value: "i"}}}}, // Match token name
			}},
		}
		// Combine with existing filter
		if len(filter) > 0 {
			filter = bson.D{{Key: "$and", Value: bson.A{filter, searchFilter}}}
		} else {
			filter = searchFilter
		}
	}

	// Get total count for pagination
	total, err := configs.ContractInfoCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Set up pagination options
	skip := page * limit
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "_id", Value: -1}}) // Latest first

	cursor, err := configs.ContractInfoCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	// Decode directly into the slice of models.ContractInfo
	if err := cursor.All(ctx, &contracts); err != nil {
		return nil, 0, err
	}

	// Return empty slice instead of nil if no contracts found
	if contracts == nil {
		contracts = make([]models.ContractInfo, 0)
	}

	return contracts, total, nil
}

func ReturnContractCode(address string) (models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// Normalize address to canonical Q-prefix form
	normalizedAddr := normalizeAddress(address)

	// Query for contract code
	filter := bson.M{"address": normalizedAddr}
	err := configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Log that we couldn't find the contract
			log.Printf("No contract found for address: %s (normalized: %s)", address, normalizedAddr)
			// Return empty contract code with expected structure
			return models.ContractInfo{
				ContractAddress:        "",
				ContractCreatorAddress: "",
				ContractCode:           "",
				CreationTransaction:    "",
				IsToken:                false,
				Status:                 "",
				TokenDecimals:          0,
				TokenName:              "",
				TokenSymbol:            "",
				UpdatedAt:              "",
			}, nil
		}
		return result, err
	}

	return result, nil
}

// CountContracts returns the total number of smart contracts. Uses
// EstimatedDocumentCount (metadata read, ~microseconds) instead of
// CountDocuments({}) which is O(rows) and blocked hot endpoints
// (/overview) under any concurrency.
func CountContracts() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.ContractInfoCollection.EstimatedDocumentCount(ctx)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// MarkContractVerified writes the verification result onto the existing
// contract document using a **targeted $set of only verification fields**.
// This is the symmetric half of the syncer's field-scoped $set (see
// QRL2MongoDB/db/contracts.go:syncerOwnedSet) — neither writer ever
// touches the other's keys, so concurrent updates can't clobber each
// other regardless of which thread reads first.
//
// `verifiedAt` is stamped here; callers should not pre-populate it.
func MarkContractVerified(address string, result models.VerificationResult) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	normalizedAddr := normalizeAddress(address)
	verifiedAt := time.Now().UTC().Format(time.RFC3339)

	verificationSet := bson.M{
		"verified":             true,
		"sourceCode":           result.SourceCode,
		"abi":                  result.Abi,
		"contractName":         result.ContractName,
		"compilerVersion":      result.CompilerVersion,
		"optimizationEnabled":  result.OptimizationEnabled,
		"optimizationRuns":     result.OptimizationRuns,
		"evmVersion":           result.EvmVersion,
		"constructorArguments": result.ConstructorArguments,
		"libraries":            result.Libraries,
		"license":              result.License,
		"verificationMethod":   result.VerificationMethod,
		"verifiedAt":           verifiedAt,
	}

	res, err := configs.ContractInfoCollection.UpdateOne(
		ctx,
		bson.M{"address": normalizedAddr},
		bson.M{"$set": verificationSet},
	)
	if err != nil {
		log.Printf("MarkContractVerified: failed to update %s: %v", normalizedAddr, err)
		return err
	}
	if res.MatchedCount == 0 {
		log.Printf("MarkContractVerified: no contract document for %s", normalizedAddr)
		return mongo.ErrNoDocuments
	}
	return nil
}

// SaveContractExplanation persists the M6a AI-generated explanation onto
// the contract document. Uses the same field-scoped $set pattern as
// MarkContractVerified so the syncer cannot stomp the cache during a
// concurrent contract-state refresh.
//
// generatedAt is the caller's responsibility (typically time.Now().UTC()
// formatted RFC3339) so the explainer can echo back the same timestamp it
// returns to the HTTP client without a re-read.
func SaveContractExplanation(address, explanation, model, generatedAt string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	normalizedAddr := normalizeAddress(address)
	res, err := configs.ContractInfoCollection.UpdateOne(
		ctx,
		bson.M{"address": normalizedAddr},
		bson.M{"$set": bson.M{
			"aiExplanation":      explanation,
			"aiExplanationAt":    generatedAt,
			"aiExplanationModel": model,
		}},
	)
	if err != nil {
		log.Printf("SaveContractExplanation: failed to update %s: %v", normalizedAddr, err)
		return err
	}
	if res.MatchedCount == 0 {
		log.Printf("SaveContractExplanation: no contract document for %s", normalizedAddr)
		return mongo.ErrNoDocuments
	}
	return nil
}

// CreateVerificationJob inserts a fresh job row in `pending` state. The
// caller (HTTP layer) supplies a UUID-like JobID and the echoed Payload.
func CreateVerificationJob(job models.ContractVerificationJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)
	job.Address = normalizeAddress(job.Address)
	if job.Status == "" {
		job.Status = models.VerificationJobPending
	}
	if job.CreatedAt == "" {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	_, err := configs.ContractVerificationsCollection.InsertOne(ctx, job)
	if err != nil {
		log.Printf("CreateVerificationJob: failed for %s/%s: %v", job.Address, job.JobID, err)
		return err
	}
	return nil
}

// GetVerificationJob looks up a job by its JobID. Returns (nil, nil) when
// no job matches so callers can treat absence and error distinctly.
func GetVerificationJob(jobID string) (*models.ContractVerificationJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var job models.ContractVerificationJob
	err := configs.ContractVerificationsCollection.FindOne(ctx, bson.M{"jobId": jobID}).Decode(&job)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

// UpdateVerificationJob applies a targeted $set to a job document — only
// the supplied fields are written. Use this for status transitions
// (pending → compiling → success/failed), error notes, and the result
// handle. `updatedAt` is stamped automatically.
func UpdateVerificationJob(jobID string, patch bson.M) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if patch == nil {
		patch = bson.M{}
	}
	patch["updatedAt"] = time.Now().UTC().Format(time.RFC3339)

	res, err := configs.ContractVerificationsCollection.UpdateOne(
		ctx,
		bson.M{"jobId": jobID},
		bson.M{"$set": patch},
	)
	if err != nil {
		log.Printf("UpdateVerificationJob: %s: %v", jobID, err)
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// FindVerificationJobsByAddress lists verification jobs for a contract
// address, filtered to a single status (e.g. "pending" or "compiling").
// `limit` bounds the result; pass 1 for a "does any in-flight job exist"
// check.
func FindVerificationJobsByAddress(address string, status models.VerificationJobStatus, limit int64) ([]models.ContractVerificationJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	normalizedAddr := normalizeAddress(address)
	filter := bson.M{"address": normalizedAddr, "status": status}
	opts := options.Find().SetLimit(limit).SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := configs.ContractVerificationsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var jobs []models.ContractVerificationJob
	if err := cursor.All(ctx, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// GetContractByCreationTx returns contract info for a given creation transaction hash
func GetContractByCreationTx(txHash string) (*models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// Normalize tx hash to lowercase with 0x prefix
	normalizedHash := strings.ToLower(txHash)
	if !strings.HasPrefix(normalizedHash, "0x") {
		normalizedHash = "0x" + normalizedHash
	}

	filter := bson.D{{Key: "creationTransaction", Value: normalizedHash}}
	err := configs.ContractInfoCollection.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No contract created by this tx
		}
		return nil, err
	}

	return &result, nil
}
