package db

import (
	"QRL2MongoDB/configs"
	dbmodels "QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	balanceKindNative  = "native"
	balanceKindERC20   = "erc20"
	balanceKindERC721  = "erc721"
	balanceKindERC1155 = "erc1155"

	balanceReconciliationClaimTTL  = 2 * time.Minute
	balanceReconciliationRetryBase = 5 * time.Second
	balanceReconciliationRetryMax  = 5 * time.Minute
	balanceReconciliationErrorMax  = 1024
)

var errBalanceReconciliationClaimLost = errors.New("balance reconciliation claim lost")

type balanceReconciliation struct {
	ID              string    `bson:"_id"`
	Kind            string    `bson:"kind"`
	Address         string    `bson:"address,omitempty"`
	ContractAddress string    `bson:"contractAddress,omitempty"`
	HolderAddress   string    `bson:"holderAddress,omitempty"`
	TokenID         string    `bson:"tokenID,omitempty"`
	RollbackTo      string    `bson:"rollbackTo"`
	CreatedAt       time.Time `bson:"createdAt"`
	Attempts        int       `bson:"attempts"`
	LastError       string    `bson:"lastError,omitempty"`
	LastAttemptAt   time.Time `bson:"lastAttemptAt,omitempty"`
	NextAttemptAt   time.Time `bson:"nextAttemptAt,omitempty"`
	AvailableAt     time.Time `bson:"availableAt,omitempty"`
	ClaimToken      string    `bson:"claimToken,omitempty"`
	ClaimedAt       time.Time `bson:"claimedAt,omitempty"`
	ClaimExpiresAt  time.Time `bson:"claimExpiresAt,omitempty"`
}

func newNativeBalanceReconciliation(address, rollbackTo string) (balanceReconciliation, bool) {
	address, ok := canonicalBalanceAddress(address)
	if !ok || validation.IsZeroAddress(address) {
		return balanceReconciliation{}, false
	}
	item := balanceReconciliation{
		Kind:       balanceKindNative,
		Address:    address,
		RollbackTo: rollbackTo,
		CreatedAt:  time.Now().UTC(),
	}
	item.ID = balanceReconciliationID(item)
	return item, true
}

func newTokenBalanceReconciliation(
	kind, contractAddress, holderAddress, tokenID, rollbackTo string,
) (balanceReconciliation, bool) {
	contractAddress, ok := canonicalBalanceAddress(contractAddress)
	if !ok || validation.IsZeroAddress(contractAddress) {
		return balanceReconciliation{}, false
	}

	switch kind {
	case balanceKindERC20:
		if tokenID != "" {
			return balanceReconciliation{}, false
		}
		holderAddress, ok = canonicalBalanceAddress(holderAddress)
		if !ok || validation.IsZeroAddress(holderAddress) {
			return balanceReconciliation{}, false
		}
	case balanceKindERC721:
		if holderAddress != "" {
			return balanceReconciliation{}, false
		}
		tokenID, ok = canonicalBalanceTokenID(tokenID)
		if !ok {
			return balanceReconciliation{}, false
		}
	case balanceKindERC1155:
		holderAddress, ok = canonicalBalanceAddress(holderAddress)
		if !ok || validation.IsZeroAddress(holderAddress) {
			return balanceReconciliation{}, false
		}
		tokenID, ok = canonicalBalanceTokenID(tokenID)
		if !ok {
			return balanceReconciliation{}, false
		}
	default:
		return balanceReconciliation{}, false
	}
	item := balanceReconciliation{
		Kind:            kind,
		ContractAddress: contractAddress,
		HolderAddress:   holderAddress,
		TokenID:         tokenID,
		RollbackTo:      rollbackTo,
		CreatedAt:       time.Now().UTC(),
	}
	item.ID = balanceReconciliationID(item)
	return item, true
}

func canonicalBalanceAddress(address string) (string, bool) {
	if address == "" || address != strings.TrimSpace(address) || !validation.IsValidAddress(address) {
		return "", false
	}
	return validation.ConvertToQAddress(address), true
}

func canonicalBalanceTokenID(tokenID string) (string, bool) {
	if tokenID == "" || (len(tokenID) > 1 && tokenID[0] == '0') {
		return "", false
	}
	for _, character := range tokenID {
		if character < '0' || character > '9' {
			return "", false
		}
	}
	value, ok := new(big.Int).SetString(tokenID, 10)
	if !ok || value.Sign() < 0 || value.BitLen() > 256 || value.String() != tokenID {
		return "", false
	}
	return tokenID, true
}

func balanceReconciliationID(item balanceReconciliation) string {
	key := strings.Join([]string{
		item.Kind,
		item.Address,
		item.ContractAddress,
		item.HolderAddress,
		item.TokenID,
	}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return "balance:" + hex.EncodeToString(digest[:])
}

func dedupeBalanceReconciliations(items []balanceReconciliation) []balanceReconciliation {
	byID := make(map[string]balanceReconciliation, len(items))
	for _, item := range items {
		if item.ID != "" {
			byID[item.ID] = item
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]balanceReconciliation, 0, len(ids))
	for _, id := range ids {
		result = append(result, byID[id])
	}
	return result
}

// balanceReconciliationsForTokenTransfers derives every token snapshot that
// can have been changed by a set of orphaned transfer events. Legacy transfer
// rows without tokenStandard are ERC-20 rows from the original ingestion
// path. Mint and burn zero-address endpoints are discarded by the item
// constructors.
func balanceReconciliationsForTokenTransfers(
	transfers []dbmodels.TokenTransfer,
	rollbackTo string,
) []balanceReconciliation {
	items := make([]balanceReconciliation, 0, len(transfers)*2)
	appendItem := func(kind, contract, holder, tokenID string) {
		item, ok := newTokenBalanceReconciliation(
			kind,
			contract,
			holder,
			tokenID,
			rollbackTo,
		)
		if ok {
			items = append(items, item)
		}
	}

	for _, transfer := range transfers {
		switch transfer.TokenStandard {
		case "", rpc.StandardERC20:
			appendItem(balanceKindERC20, transfer.ContractAddress, transfer.From, "")
			appendItem(balanceKindERC20, transfer.ContractAddress, transfer.To, "")
		case rpc.StandardERC721:
			appendItem(balanceKindERC721, transfer.ContractAddress, "", transfer.TokenID)
		case rpc.StandardERC1155:
			appendItem(balanceKindERC1155, transfer.ContractAddress, transfer.From, transfer.TokenID)
			appendItem(balanceKindERC1155, transfer.ContractAddress, transfer.To, transfer.TokenID)
		}
	}

	return dedupeBalanceReconciliations(items)
}

// enqueueBalanceReconciliations writes rollback-owned invalidation work in the
// same Mongo transaction as the orphan deletions. Queue timestamps come from
// MongoDB so host clock skew cannot make new work wait or jump the queue.
func enqueueBalanceReconciliations(
	ctx context.Context,
	items []balanceReconciliation,
) error {
	items = dedupeBalanceReconciliations(items)
	if len(items) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, 0, len(items))
	for _, item := range items {
		if err := validateBalanceReconciliation(item); err != nil {
			return err
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": item.ID}).
			SetUpdate(mongo.Pipeline{
				bson.D{{Key: "$set", Value: bson.D{
					{Key: "kind", Value: item.Kind},
					{Key: "address", Value: item.Address},
					{Key: "contractAddress", Value: item.ContractAddress},
					{Key: "holderAddress", Value: item.HolderAddress},
					{Key: "tokenID", Value: item.TokenID},
					{Key: "rollbackTo", Value: item.RollbackTo},
					{Key: "createdAt", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$createdAt", "$$NOW"}}}},
					{Key: "availableAt", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$availableAt", "$$NOW"}}}},
					{Key: "attempts", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$attempts", 0}}}},
				}}},
			}).
			SetUpsert(true))
	}
	_, err := configs.GetCollection(
		configs.DB,
		configs.BALANCE_RECONCILIATIONS_COLLECTION,
	).BulkWrite(ctx, models, options.BulkWrite().SetOrdered(true))
	return err
}

func markNativeBalancesStale(ctx context.Context, addresses []string) error {
	unique := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		if item, ok := newNativeBalanceReconciliation(address, ""); ok {
			unique[item.Address] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}
	canonical := make([]string, 0, len(unique))
	for address := range unique {
		canonical = append(canonical, address)
	}
	_, err := configs.AddressesCollections.UpdateMany(
		ctx,
		bson.M{"id": bson.M{"$in": canonical}},
		bson.M{"$set": bson.M{
			"balanceStale":   true,
			"balanceStaleAt": time.Now().UTC(),
		}},
	)
	return err
}

func markTokenBalancesStale(ctx context.Context, items []balanceReconciliation) error {
	branches := make(bson.A, 0, len(items))
	for _, item := range dedupeBalanceReconciliations(items) {
		var filter bson.M
		switch item.Kind {
		case balanceKindERC20:
			filter = bson.M{
				"contractAddress": item.ContractAddress,
				"holderAddress":   item.HolderAddress,
				"$or": bson.A{
					bson.M{"tokenStandard": rpc.StandardERC20},
					bson.M{"tokenStandard": bson.M{"$exists": false}},
				},
			}
		case balanceKindERC721:
			filter = bson.M{
				"contractAddress": item.ContractAddress,
				"tokenID":         item.TokenID,
				"tokenStandard":   rpc.StandardERC721,
			}
		case balanceKindERC1155:
			filter = bson.M{
				"contractAddress": item.ContractAddress,
				"holderAddress":   item.HolderAddress,
				"tokenID":         item.TokenID,
				"tokenStandard":   rpc.StandardERC1155,
			}
		}
		if len(filter) > 0 {
			branches = append(branches, filter)
		}
	}
	if len(branches) == 0 {
		return nil
	}
	_, err := configs.GetTokenBalancesCollection().UpdateMany(
		ctx,
		bson.M{"$or": branches},
		bson.M{"$set": bson.M{
			"balanceStale":   true,
			"balanceStaleAt": time.Now().UTC(),
		}},
	)
	return err
}

// ReconcileStaleBalances refreshes a bounded fair batch from canonical RPC
// state. Each item is claimed with a crash-recoverable ownership token. Failed
// items move to bounded exponential backoff so newer work stays eligible.
func ReconcileStaleBalances(limit int) error {
	if limit <= 0 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	collection := configs.GetCollection(configs.DB, configs.BALANCE_RECONCILIATIONS_COLLECTION)
	head := GetLatestBlockFromDB()
	if head == nil {
		return fmt.Errorf("load canonical balance reconciliation head")
	}
	blockNumber, err := canonicalHexQuantity(head.Result.Number, "balance reconciliation block number")
	if err != nil {
		return err
	}
	blockHash, err := canonicalFixedHash(head.Result.Hash, "balance reconciliation block hash")
	if err != nil {
		return err
	}
	return reconcileStaleBalanceQueue(
		ctx,
		collection,
		limit,
		blockNumber,
		func(item balanceReconciliation, _ string) error {
			return reconcileBalanceItem(item, blockNumber, blockHash)
		},
	)
}

type balanceItemReconciler func(balanceReconciliation, string) error

func reconcileStaleBalanceQueue(
	ctx context.Context,
	collection *mongo.Collection,
	limit int,
	blockNumber string,
	reconcile balanceItemReconciler,
) error {
	if collection == nil {
		return fmt.Errorf("balance reconciliation collection is nil")
	}
	if reconcile == nil {
		return fmt.Errorf("balance reconciliation function is nil")
	}
	if limit <= 0 {
		return nil
	}

	for processed := 0; processed < limit; processed++ {
		item, err := claimBalanceReconciliation(ctx, collection)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim stale balance reconciliation: %w", err)
		}

		itemErr := validateBalanceReconciliation(item)
		if itemErr == nil {
			itemErr = reconcile(item, blockNumber)
		}
		if itemErr != nil {
			if err := recordBalanceReconciliationFailure(ctx, collection, item, itemErr); err != nil {
				return fmt.Errorf("record balance reconciliation failure: %w", err)
			}
			continue
		}

		if err := completeBalanceReconciliation(ctx, collection, item); err != nil {
			return fmt.Errorf("complete stale balance reconciliation: %w", err)
		}
	}
	return nil
}

func claimBalanceReconciliation(
	ctx context.Context,
	collection *mongo.Collection,
) (balanceReconciliation, error) {
	claimToken, err := newBalanceReconciliationClaimToken()
	if err != nil {
		return balanceReconciliation{}, err
	}
	claimDeadline := mongoDateAfterNow(balanceReconciliationClaimTTL)
	filter := bson.M{
		"$or": bson.A{
			bson.M{"availableAt": bson.M{"$exists": false}},
			bson.M{"$expr": bson.M{"$lte": bson.A{"$availableAt", "$$NOW"}}},
		},
	}
	update := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "claimToken", Value: claimToken},
			{Key: "claimedAt", Value: "$$NOW"},
			{Key: "claimExpiresAt", Value: claimDeadline},
			{Key: "availableAt", Value: claimDeadline},
		}}},
	}
	claimOptions := options.FindOneAndUpdate().
		SetSort(bson.D{
			{Key: "availableAt", Value: 1},
			{Key: "createdAt", Value: 1},
			{Key: "_id", Value: 1},
		}).
		SetReturnDocument(options.After)

	var item balanceReconciliation
	err = collection.FindOneAndUpdate(ctx, filter, update, claimOptions).Decode(&item)
	return item, err
}

func recordBalanceReconciliationFailure(
	ctx context.Context,
	collection *mongo.Collection,
	item balanceReconciliation,
	reconciliationErr error,
) error {
	attempts := item.Attempts + 1
	if attempts < 1 {
		attempts = 1
	}
	retryAt := mongoDateAfterNow(balanceReconciliationRetryBackoff(attempts))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": item.ID, "claimToken": item.ClaimToken},
		mongo.Pipeline{
			bson.D{{Key: "$set", Value: bson.D{
				{Key: "attempts", Value: attempts},
				{Key: "lastError", Value: boundedBalanceReconciliationError(reconciliationErr)},
				{Key: "lastAttemptAt", Value: "$$NOW"},
				{Key: "nextAttemptAt", Value: retryAt},
				{Key: "availableAt", Value: retryAt},
				{Key: "claimToken", Value: "$$REMOVE"},
				{Key: "claimedAt", Value: "$$REMOVE"},
				{Key: "claimExpiresAt", Value: "$$REMOVE"},
			}}},
		},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errBalanceReconciliationClaimLost
	}
	return nil
}

func completeBalanceReconciliation(
	ctx context.Context,
	collection *mongo.Collection,
	item balanceReconciliation,
) error {
	result, err := collection.DeleteOne(
		ctx,
		bson.M{"_id": item.ID, "claimToken": item.ClaimToken},
	)
	if err != nil {
		return err
	}
	if result.DeletedCount != 1 {
		return errBalanceReconciliationClaimLost
	}
	return nil
}

func newBalanceReconciliationClaimToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate balance reconciliation claim token: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func mongoDateAfterNow(delay time.Duration) bson.D {
	return bson.D{{Key: "$dateAdd", Value: bson.D{
		{Key: "startDate", Value: "$$NOW"},
		{Key: "unit", Value: "millisecond"},
		{Key: "amount", Value: delay.Milliseconds()},
	}}}
}

func balanceReconciliationRetryBackoff(attempts int) time.Duration {
	if attempts <= 1 {
		return balanceReconciliationRetryBase
	}
	delay := balanceReconciliationRetryBase
	for retry := 1; retry < attempts && delay < balanceReconciliationRetryMax; retry++ {
		delay *= 2
		if delay >= balanceReconciliationRetryMax {
			return balanceReconciliationRetryMax
		}
	}
	return delay
}

func boundedBalanceReconciliationError(err error) string {
	if err == nil {
		return ""
	}
	message := []rune(err.Error())
	if len(message) <= balanceReconciliationErrorMax {
		return string(message)
	}
	return string(message[:balanceReconciliationErrorMax])
}

func validateBalanceReconciliation(item balanceReconciliation) error {
	var expected balanceReconciliation
	var ok bool
	switch item.Kind {
	case balanceKindNative:
		expected, ok = newNativeBalanceReconciliation(item.Address, item.RollbackTo)
	case balanceKindERC20, balanceKindERC721, balanceKindERC1155:
		expected, ok = newTokenBalanceReconciliation(
			item.Kind,
			item.ContractAddress,
			item.HolderAddress,
			item.TokenID,
			item.RollbackTo,
		)
	}
	if !ok {
		return fmt.Errorf("invalid %q balance reconciliation key", item.Kind)
	}
	if item.ID != expected.ID || item.Address != expected.Address ||
		item.ContractAddress != expected.ContractAddress ||
		item.HolderAddress != expected.HolderAddress || item.TokenID != expected.TokenID {
		return fmt.Errorf("non-canonical %q balance reconciliation key", item.Kind)
	}
	return nil
}

func reconcileBalanceItem(item balanceReconciliation, blockNumber, blockHash string) error {
	switch item.Kind {
	case balanceKindNative:
		return RefreshAddressBalance(item.Address, false)
	case balanceKindERC20:
		return StoreTokenBalanceAtBlock(
			item.ContractAddress,
			item.HolderAddress,
			"",
			blockNumber,
			blockHash,
		)
	case balanceKindERC721:
		id, ok := new(big.Int).SetString(item.TokenID, 10)
		if !ok {
			return fmt.Errorf("invalid ERC-721 tokenID %q", item.TokenID)
		}
		return StoreERC721OwnershipAtBlock(item.ContractAddress, id, blockNumber, blockHash)
	case balanceKindERC1155:
		id, ok := new(big.Int).SetString(item.TokenID, 10)
		if !ok {
			return fmt.Errorf("invalid ERC-1155 tokenID %q", item.TokenID)
		}
		return StoreERC1155BalanceAtBlock(
			item.ContractAddress,
			item.HolderAddress,
			id,
			blockNumber,
			blockHash,
		)
	default:
		return fmt.Errorf("unsupported balance reconciliation kind %q", item.Kind)
	}
}
