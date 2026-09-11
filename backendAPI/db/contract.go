package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"backendAPI/sourcebundle"
	"context"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ReturnContracts paginates contracts with optional `isToken` and
// `tokenStandard` filters. `standardFilter` overlays on top of
// `isTokenFilter` so e.g. `isToken=true,standard=ERC-721` returns the
// intersection (NFT collections only). Pass nil/empty for either filter to
// skip that predicate.
func ReturnContracts(page int64, limit int64, search string, isTokenFilter *bool, standardFilter *string) ([]models.ContractInfo, int64, error) {
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

	// Add tokenStandard filter if specified. Both filters AND together ,
	// `?isToken=true&standard=ERC-721` returns NFT collections only.
	if standardFilter != nil && *standardFilter != "" {
		filter = append(filter, bson.E{Key: "tokenStandard", Value: *standardFilter})
	}

	// Add search if provided, using correct field names
	if search != "" {
		// Normalize the search address to canonical Q-prefix form (for the
		// address + creatorAddress exact-match branches).
		normalizedSearch := normalizeAddress(search)

		// Build the $or branches. Exact address/creatorAddress matches are
		// always included so short queries can still resolve a contract by
		// its address. The case-insensitive name/symbol/metadataName regex
		// branches run an unanchored full-collection scan, so we only add
		// them for queries of at least 3 chars: a 1-2 char regex matches
		// nearly every row and turns search into a table scan with no useful
		// result. Shorter queries simply return the exact-match results
		// (often empty), which the contracts page renders as a normal empty
		// paginated list.
		orBranches := bson.A{
			bson.D{{Key: "address", Value: normalizedSearch}},
			bson.D{{Key: "creatorAddress", Value: normalizedSearch}},
		}
		if len(search) >= 3 {
			// Escape regex metacharacters in the user input so a `.` or `*`
			// in the search term doesn't behave as a wildcard or DoS vector.
			escaped := regexp.QuoteMeta(search)
			nameRegex := bson.D{{Key: "$regex", Value: escaped}, {Key: "$options", Value: "i"}}

			// Symbol was previously absent, searching "MQW" missed tokens whose
			// `name` field held the full project label instead of the ticker.
			// Phase 3a adds metadataName so a user searching by the off-chain
			// display title hits the row even when on-chain name() is empty.
			orBranches = append(orBranches,
				bson.D{{Key: "name", Value: nameRegex}},
				bson.D{{Key: "symbol", Value: nameRegex}},
				bson.D{{Key: "metadataName", Value: nameRegex}},
			)
		}

		searchFilter := bson.D{{Key: "$or", Value: orBranches}}
		// Combine with existing filter
		if len(filter) > 0 {
			filter = bson.D{{Key: "$and", Value: bson.A{filter, searchFilter}}}
		} else {
			filter = searchFilter
		}
	}

	// Get the total after applying the same creation-block visibility fence as
	// the page query below.
	total, err := aggregateCanonicalCount(
		ctx,
		configs.ContractInfoCollection,
		canonicalContractPipeline(filter),
	)
	if err != nil {
		return nil, 0, err
	}

	// Set up pagination options. The contracts list/grid renders only the
	// lightweight summary fields (address, name, symbol, decimals, standard,
	// metadata thumbnail, etc), so project out the heavy verification blobs
	// (full source, ABI, constructor args, libraries, imports, raw bytecode, AI
	// explanation). These can be tens to hundreds of KB per row and would
	// otherwise be streamed for every page even though no consumer reads them.
	skip := page * limit
	projection := bson.D{
		{Key: "sourceCode", Value: 0},
		{Key: "abi", Value: 0},
		{Key: "constructorArguments", Value: 0},
		{Key: "libraries", Value: 0},
		{Key: "imports", Value: 0},
		{Key: "contractCode", Value: 0},
		{Key: "aiExplanation", Value: 0},
	}
	pipeline := canonicalContractPipeline(filter)
	pipeline = append(pipeline,
		bson.M{"$sort": bson.D{{Key: "_id", Value: -1}}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
		bson.M{"$project": projection},
	)
	cursor, err := configs.ContractInfoCollection.Aggregate(ctx, pipeline)
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

// ContractCountsByStandard is the per-tab breakdown returned by
// /contracts/counts: the three known token standards plus "other"
// (non-token contracts). Used to populate the count chips on the
// contracts-page tab buttons in one round trip.
type ContractCountsByStandard struct {
	ERC20   int64 `json:"erc20"`
	ERC721  int64 `json:"erc721"`
	ERC1155 int64 `json:"erc1155"`
	Other   int64 `json:"other"`
}

// GetContractCountsByStandard returns the four tab buckets the frontend
// shows on /contracts in a single aggregation. One $group instead of
// four CountDocuments calls keeps the cost predictable as the
// contractCode collection grows.
//
// The "other" bucket is non-token contracts (isToken=false). Three
// token buckets are scoped by tokenStandard. Legacy rows without a
// tokenStandard tag (pre-Phase 1) and isToken=true still count as
// "other" so the four buckets always sum to the total contractCode
// canonical-visible document count, useful for sanity-checking.
func GetContractCountsByStandard() (ContractCountsByStandard, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var out ContractCountsByStandard

	// $group by a synthesised bucket label so we get all four counts back
	// in one aggregation pass.
	pipeline := canonicalContractPipeline(bson.D{})
	pipeline = append(pipeline,
		bson.M{
			"$group": bson.M{
				"_id": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []interface{}{"$isToken", true}},
						bson.M{
							"$switch": bson.M{
								"branches": []bson.M{
									{"case": bson.M{"$eq": []interface{}{"$tokenStandard", "ERC-20"}}, "then": "erc20"},
									{"case": bson.M{"$eq": []interface{}{"$tokenStandard", "ERC-721"}}, "then": "erc721"},
									{"case": bson.M{"$eq": []interface{}{"$tokenStandard", "ERC-1155"}}, "then": "erc1155"},
								},
								"default": "other",
							},
						},
						"other",
					},
				},
				"count": bson.M{"$sum": 1},
			},
		},
	)

	cursor, err := configs.ContractInfoCollection.Aggregate(ctx, pipeline)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return out, nil
		}
		return out, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&row); err != nil {
			continue
		}
		switch row.ID {
		case "erc20":
			out.ERC20 = row.Count
		case "erc721":
			out.ERC721 = row.Count
		case "erc1155":
			out.ERC1155 = row.Count
		default:
			out.Other += row.Count
		}
	}
	if err := cursor.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// GetContractsByAddresses batch-loads contract docs for a set of addresses,
// returning a map keyed by the canonical Q-prefix address. Used by the
// /tx/:hash route to attach ABI metadata to receipt logs without N
// round-trips. Missing addresses simply don't appear in the returned map,
// callers handle the absent-key case as "no contract info available".
//
// Each input address is normalized to the canonical Q-prefix lowercase
// form the syncer stores. Empty input returns an empty map without
// touching mongo.
func GetContractsByAddresses(addresses []string) (map[string]models.ContractInfo, error) {
	out := make(map[string]models.ContractInfo)
	if len(addresses) == 0 {
		return out, nil
	}

	// Dedupe + normalize so the $in list is minimal.
	seen := make(map[string]struct{}, len(addresses))
	normalized := make([]string, 0, len(addresses))
	for _, a := range addresses {
		n := normalizeAddress(a)
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		normalized = append(normalized, n)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// The /tx and /pending-transaction consumers read only compact metadata.
	pipeline := canonicalContractPipeline(bson.M{
		"address": bson.M{"$in": normalized},
	})
	pipeline = append(pipeline, bson.M{"$project": contractMetadataProjection()})
	cursor, err := configs.ContractInfoCollection.Aggregate(ctx, pipeline)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return out, nil
		}
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var c models.ContractInfo
		if err := cursor.Decode(&c); err != nil {
			continue
		}
		out[c.ContractAddress] = c
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// contractMetadataProjection keeps compact transaction and log lookups lean.
// It deliberately retains the verification-record schema, compiler
// provenance, sourceBundleDigest, and ABI so routes can apply the
// recorded-metadata trust gate without loading source bytes. Exact
// source-bundle recomputation belongs to full contract reads.
func contractMetadataProjection() bson.D {
	return bson.D{
		{Key: "sourceCode", Value: 0},
		{Key: "constructorArguments", Value: 0},
		{Key: "libraries", Value: 0},
		{Key: "imports", Value: 0},
		{Key: "contractCode", Value: 0},
		{Key: "aiExplanation", Value: 0},
	}
}

func ReturnContractCode(address string) (models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractInfo

	// Normalize address to canonical Q-prefix form
	normalizedAddr := normalizeAddress(address)

	// Query for contract code after proving its creation block is complete.
	filter := bson.M{"address": normalizedAddr}
	pipeline := canonicalContractPipeline(filter)
	pipeline = append(pipeline, bson.M{"$limit": 1})
	cursor, err := configs.ContractInfoCollection.Aggregate(ctx, pipeline)
	if err == nil {
		defer cursor.Close(ctx)
		if cursor.Next(ctx) {
			err = cursor.Decode(&result)
		} else if cursor.Err() != nil {
			err = cursor.Err()
		} else {
			err = mongo.ErrNoDocuments
		}
	}

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

	sanitizeCachedExplanation(&result)
	return result, nil
}

func sanitizeCachedExplanation(contract *models.ContractInfo) {
	if contract.AIExplanation == "" {
		return
	}
	if sourcebundle.ClassifyStoredVerification(*contract) != models.CompilerProvenanceDigestBacked {
		clearCachedExplanation(contract)
		return
	}
	currentDigest := sourcebundle.Digest(
		contract.ContractName,
		contract.SourceCode,
		contract.Imports,
	)
	if contract.AIExplanationSourceDigest == currentDigest &&
		contract.SourceBundleDigest == currentDigest {
		return
	}
	clearCachedExplanation(contract)
}

func clearCachedExplanation(contract *models.ContractInfo) {
	contract.AIExplanation = ""
	contract.AIExplanationAt = ""
	contract.AIExplanationModel = ""
	contract.AIExplanationSourceDigest = ""
}

// CountContracts returns the exact number of contracts whose creation block is
// complete, plus explicitly marked genesis contracts. Callers cache the result.
func CountContracts() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := aggregateCanonicalCount(
		ctx,
		configs.ContractInfoCollection,
		canonicalContractPipeline(bson.D{}),
	)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// MarkContractVerified atomically publishes the contract verification and the
// terminal job result. The transaction reads the canonical creation block and
// conditionally updates the exact contract and job generation captured before
// compilation. A rollback transaction conflicts with this transaction and a
// recreated address cannot satisfy the old target filter.
func MarkContractVerified(
	jobID string,
	expected models.VerificationTarget,
	result models.VerificationResult,
) (*models.VerificationJobResultRef, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if configs.DB == nil {
		return nil, errors.New("MarkContractVerified: database client is unavailable")
	}
	verifiedAt := time.Now().UTC().Format(time.RFC3339Nano)
	sourceBundleDigest := sourcebundle.Digest(result.ContractName, result.SourceCode, result.Imports)
	artifactDigest := models.VerificationArtifactDigestV2(expected, result, sourceBundleDigest)
	resultRef := &models.VerificationJobResultRef{
		Abi:                result.Abi,
		BytecodeHash:       expected.DeployedCodeSHA256,
		ArtifactDigest:     artifactDigest,
		CompilerProvenance: result.CompilerProvenance,
		VerifiedAt:         verifiedAt,
	}

	session, err := configs.DB.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		if err := configs.BlocksCollection.FindOne(
			sessCtx,
			verificationCanonicalBlockFilter(expected),
			options.FindOne().SetProjection(bson.M{"_id": 1}),
		).Err(); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, ErrVerificationTargetChanged
			}
			return nil, err
		}

		if err := updateContractVerification(
			sessCtx,
			configs.ContractInfoCollection,
			expected,
			contractVerificationUpdate(result, verifiedAt, expected),
		); err != nil {
			return nil, err
		}

		jobResult, err := configs.ContractVerificationsCollection.UpdateOne(
			sessCtx,
			verificationJobCommitFilter(jobID, expected),
			bson.M{"$set": bson.M{
				"status":    models.VerificationJobSuccess,
				"error":     "",
				"result":    resultRef,
				"updatedAt": verifiedAt,
			}},
		)
		if err != nil {
			return nil, err
		}
		if jobResult.MatchedCount == 0 {
			return nil, ErrVerificationTargetChanged
		}
		return nil, nil
	})
	if err != nil {
		log.Printf("MarkContractVerified: fenced job %s for %s: %v", jobID, expected.Address, err)
		return nil, err
	}
	return resultRef, nil
}

type verificationConditionalUpdater interface {
	UpdateOne(
		context.Context,
		interface{},
		interface{},
		...*options.UpdateOptions,
	) (*mongo.UpdateResult, error)
}

func updateContractVerification(
	ctx context.Context,
	collection verificationConditionalUpdater,
	expected models.VerificationTarget,
	update bson.M,
) error {
	res, err := collection.UpdateOne(ctx, verificationContractTargetFilter(expected), update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrVerificationTargetChanged
	}
	return nil
}

func verificationContractTargetFilter(expected models.VerificationTarget) bson.M {
	filter := bson.M{
		"address":             normalizeAddress(expected.Address),
		"creationTransaction": expected.CreationTransaction,
		"creationBlockNumber": expected.CreationBlockNumber,
		"creationBlockHash":   expected.CreationBlockHash,
		"chainId":             expected.ChainID,
		"contractCodeSha256":  expected.DeployedCodeSHA256,
		"verified":            bson.M{"$ne": true},
	}
	if expected.GenesisContract {
		filter["genesisContract"] = true
	} else {
		filter["genesisContract"] = bson.M{"$ne": true}
	}
	return filter
}

func verificationCanonicalBlockFilter(expected models.VerificationTarget) bson.M {
	return bson.M{
		"result.number":  expected.CreationBlockNumber,
		"result.hash":    expected.CreationBlockHash,
		"ingestionState": completedBlockIngestionState,
	}
}

func verificationJobCommitFilter(jobID string, expected models.VerificationTarget) bson.M {
	filter := verificationJobTargetFilter(jobID, expected)
	filter["status"] = models.VerificationJobCompiling
	return filter
}

func verificationJobTargetFilter(jobID string, expected models.VerificationTarget) bson.M {
	return bson.M{
		"jobId":                      jobID,
		"address":                    normalizeAddress(expected.Address),
		"target.address":             normalizeAddress(expected.Address),
		"target.creationTransaction": expected.CreationTransaction,
		"target.creationBlockNumber": expected.CreationBlockNumber,
		"target.creationBlockHash":   expected.CreationBlockHash,
		"target.chainId":             expected.ChainID,
		"target.deployedCodeSha256":  expected.DeployedCodeSHA256,
		"target.genesisContract":     expected.GenesisContract,
	}
}

func contractVerificationUpdate(
	result models.VerificationResult,
	verifiedAt string,
	targets ...models.VerificationTarget,
) bson.M {
	unset := bson.M{
		"aiExplanation":             "",
		"aiExplanationAt":           "",
		"aiExplanationModel":        "",
		"aiExplanationSourceDigest": "",
		"aiExplanationLease":        "",
	}
	if len(result.Imports) == 0 {
		// A successful single-file re-verification replaces any imports from a
		// previous multi-file verification instead of leaving a stale bundle.
		unset["imports"] = ""
	}
	if result.CompilerProvenance == nil {
		// Legacy or direct callers without an artifact identity must not inherit
		// provenance from an earlier verification run.
		unset["compilerProvenance"] = ""
	}
	if len(targets) == 0 {
		unset["verificationArtifactDigest"] = ""
	}
	return bson.M{
		"$set":   contractVerificationSet(result, verifiedAt, targets...),
		"$unset": unset,
	}
}

func contractVerificationSet(
	result models.VerificationResult,
	verifiedAt string,
	targets ...models.VerificationTarget,
) bson.M {
	sourceBundleDigest := sourcebundle.Digest(
		result.ContractName,
		result.SourceCode,
		result.Imports,
	)
	fields := bson.M{
		"verified":                 true,
		"verificationRecordSchema": models.VerificationRecordSchemaV1,
		"sourceCode":               result.SourceCode,
		"abi":                      result.Abi,
		"contractName":             result.ContractName,
		"compilerVersion":          result.CompilerVersion,
		"optimizationEnabled":      result.OptimizationEnabled,
		"optimizationRuns":         result.OptimizationRuns,
		"evmVersion":               result.EvmVersion,
		"constructorArguments":     result.ConstructorArguments,
		"libraries":                result.Libraries,
		"license":                  result.License,
		"verificationMethod":       result.VerificationMethod,
		"verifiedAt":               verifiedAt,
		"sourceBundleDigest":       sourceBundleDigest,
	}
	if len(targets) > 0 {
		fields["verificationRecordSchema"] = models.VerificationRecordSchemaV2
		fields["verificationArtifactDigest"] = models.VerificationArtifactDigestV2(
			targets[0],
			result,
			sourceBundleDigest,
		)
	}
	if len(result.Imports) > 0 {
		fields["imports"] = result.Imports
	}
	if result.CompilerProvenance != nil {
		fields["compilerProvenance"] = result.CompilerProvenance
	}
	return fields
}

// ErrAIRegenCap signals that the per-contract regen cap has been reached
// inside the current rolling window. The HTTP layer maps it to 429.
var ErrAIRegenCap = errors.New("AI regen cap reached")

// ErrSourceBundleChanged means an AI explanation finished after the verified
// source bundle changed. Callers must discard that explanation and retry from
// a fresh contract snapshot.
var ErrSourceBundleChanged = errors.New("contract source bundle changed")

// ErrVerificationTargetChanged means a verification worker lost authority to
// publish because its job, canonical block, or indexed contract generation no
// longer matches the snapshot captured before compilation.
var ErrVerificationTargetChanged = errors.New("verification target changed")

// ReserveAIRegenSlot atomically reserves one regeneration slot on the
// contract document. Returns ErrAIRegenCap when:
//
//   - the existing window has not yet rolled over (now - windowStart < window) AND
//   - the count has already reached limit.
//
// Otherwise it either increments the count within the current window or
// resets the window start to now with count=1 (rollover case / first regen).
// Both paths use a single $set/$inc UpdateOne so there's no read-modify-
// write race between concurrent callers, the loser sees ErrAIRegenCap.
func ReserveAIRegenSlot(address string, limit int, window time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	normalizedAddr := normalizeAddress(address)
	now := time.Now().UTC()
	cutoff := now.Add(-window).Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	// Path A: increment within the current window (window is still fresh,
	// count is below limit).
	res, err := configs.ContractInfoCollection.UpdateOne(
		ctx,
		bson.M{
			"address":                       normalizedAddr,
			"aiExplanationRegenWindowStart": bson.M{"$gte": cutoff},
			"aiExplanationRegenCount":       bson.M{"$lt": limit},
		},
		bson.M{"$inc": bson.M{"aiExplanationRegenCount": 1}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 {
		return nil
	}

	// Path B: open a fresh window. Matches when the window expired, was
	// never started, or the field is missing entirely.
	res, err = configs.ContractInfoCollection.UpdateOne(
		ctx,
		bson.M{
			"address": normalizedAddr,
			"$or": []bson.M{
				{"aiExplanationRegenWindowStart": bson.M{"$lt": cutoff}},
				{"aiExplanationRegenWindowStart": bson.M{"$exists": false}},
				{"aiExplanationRegenWindowStart": ""},
			},
		},
		bson.M{"$set": bson.M{
			"aiExplanationRegenCount":       1,
			"aiExplanationRegenWindowStart": nowStr,
		}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 {
		return nil
	}

	// Neither path matched: a fresh window exists AND count is at limit.
	// (If the address truly doesn't exist, Path B's first clause `$or`
	// branches would not match because the address filter rules them out;
	// but the caller already validated existence via ReturnContractCode,
	// so we know the contract is present.)
	return ErrAIRegenCap
}

// SaveContractExplanation persists the M6a AI-generated explanation onto
// the contract document. The update is conditional on the exact verification
// generation, source identity, and single-flight lease observed before the
// model call. The same atomic update clears the matching lease, so an in-flight
// explanation cannot repopulate cache state cleared by a concurrent
// re-verification. The syncer cannot write these backend-owned fields.
//
// generatedAt is the caller's responsibility (typically time.Now().UTC()
// formatted RFC3339) so the explainer can echo back the same timestamp it
// returns to the HTTP client without a re-read.
func SaveContractExplanation(
	ctx context.Context,
	address,
	explanation,
	model,
	generatedAt,
	sourceDigest string,
	lease models.AIExplanationLease,
	expected models.ContractInfo,
) error {
	if sourcebundle.ClassifyStoredVerification(expected) != models.CompilerProvenanceDigestBacked ||
		expected.SourceBundleDigest != sourceDigest ||
		expected.VerifiedAt == "" ||
		lease.Token == "" ||
		lease.SourceDigest != sourceDigest ||
		lease.VerifiedAt != expected.VerifiedAt {
		return ErrSourceBundleChanged
	}
	operationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	normalizedAddr := normalizeAddress(address)
	res, err := configs.ContractInfoCollection.UpdateOne(
		operationCtx,
		explanationSaveFilter(normalizedAddr, expected, sourceDigest, lease),
		explanationSaveUpdate(explanation, model, generatedAt, sourceDigest),
	)
	if err != nil {
		log.Printf("SaveContractExplanation: failed to update %s: %v", normalizedAddr, err)
		return err
	}
	if res.MatchedCount == 0 {
		log.Printf("SaveContractExplanation: source bundle changed for %s", normalizedAddr)
		return ErrSourceBundleChanged
	}
	return nil
}

func explanationSaveUpdate(explanation, model, generatedAt, sourceDigest string) bson.M {
	return bson.M{
		"$set": bson.M{
			"aiExplanation":             explanation,
			"aiExplanationAt":           generatedAt,
			"aiExplanationModel":        model,
			"aiExplanationSourceDigest": sourceDigest,
		},
		"$unset": bson.M{"aiExplanationLease": ""},
	}
}

func explanationSaveFilter(
	normalizedAddr string,
	expected models.ContractInfo,
	sourceDigest string,
	lease models.AIExplanationLease,
) bson.M {
	filter := explanationSourceSnapshotFilter(normalizedAddr, expected, sourceDigest)
	for key, value := range explanationLeaseIdentityFilter(normalizedAddr, lease) {
		filter[key] = value
	}
	return filter
}

func explanationSourceSnapshotFilter(
	normalizedAddr string,
	expected models.ContractInfo,
	sourceDigest string,
) bson.M {
	filter := bson.M{
		"address":                  normalizedAddr,
		"verified":                 true,
		"verificationRecordSchema": expected.VerificationRecordSchema,
		"verifiedAt":               expected.VerifiedAt,
		"sourceCode":               expected.SourceCode,
		"contractName":             expected.ContractName,
		"sourceBundleDigest":       sourceDigest,
	}
	return filter
}

// CreateVerificationJob inserts a fresh job row in `pending` state. The
// caller (HTTP layer) supplies a UUID-like JobID and the echoed Payload.
func CreateVerificationJob(job models.ContractVerificationJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)
	job.Address = normalizeAddress(job.Address)
	job.Target.Address = normalizeAddress(job.Target.Address)
	if job.Target.Address != job.Address || job.Target.DeployedCodeSHA256 == "" {
		return errors.New("CreateVerificationJob: incomplete or mismatched verification target")
	}
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

// ClaimVerificationJob is the only pending-to-compiling transition. The full
// target is part of the filter so a caller cannot attach an old job ID to a
// newer contract generation.
func ClaimVerificationJob(jobID string, expected models.VerificationTarget) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := verificationJobTargetFilter(jobID, expected)
	filter["status"] = models.VerificationJobPending
	res, err := configs.ContractVerificationsCollection.UpdateOne(
		ctx,
		filter,
		bson.M{"$set": bson.M{
			"status":    models.VerificationJobCompiling,
			"updatedAt": time.Now().UTC().Format(time.RFC3339),
		}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrVerificationTargetChanged
	}
	return nil
}

// FailVerificationJob records a terminal failure only while the exact job is
// still pending or compiling. A rollback-failed job stays terminal even when
// an old worker reports a later compiler or RPC error.
func FailVerificationJob(jobID string, expected models.VerificationTarget, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := verificationJobTargetFilter(jobID, expected)
	filter["status"] = bson.M{"$in": []models.VerificationJobStatus{
		models.VerificationJobPending,
		models.VerificationJobCompiling,
	}}
	res, err := configs.ContractVerificationsCollection.UpdateOne(
		ctx,
		filter,
		bson.M{"$set": bson.M{
			"status":    models.VerificationJobFailed,
			"error":     message,
			"updatedAt": time.Now().UTC().Format(time.RFC3339),
		}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrVerificationTargetChanged
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

// UpdateVerificationJob applies a targeted $set to a job document, only
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
	pipeline := canonicalContractPipeline(filter)
	pipeline = append(pipeline, bson.M{"$limit": 1})
	cursor, err := configs.ContractInfoCollection.Aggregate(ctx, pipeline)
	if err == nil {
		defer cursor.Close(ctx)
		if cursor.Next(ctx) {
			err = cursor.Decode(&result)
		} else if cursor.Err() != nil {
			err = cursor.Err()
		} else {
			err = mongo.ErrNoDocuments
		}
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No contract created by this tx
		}
		return nil, err
	}

	return &result, nil
}
