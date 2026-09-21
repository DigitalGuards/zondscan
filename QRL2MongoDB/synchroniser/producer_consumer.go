package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/utils"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Batch size constants for consistent use across sync methods
const (
	// DefaultBatchSize is the standard batch size for normal sync operations
	DefaultBatchSize = 128

	// LargeBatchSize is used when syncing a large number of blocks (>1000)
	LargeBatchSize = 256

	// BatchSyncThreshold is the number of blocks behind after which we switch to batch sync
	BatchSyncThreshold = 64

	// LargeSyncThreshold is the number of blocks that triggers using the larger batch size
	LargeSyncThreshold = 1000 // 0x3e8 in hex

	// MaxProducerConcurrency limits concurrent block fetching goroutines
	MaxProducerConcurrency = 16
)

type blockProducerGroup struct {
	semaphore chan struct{}
	waitGroup sync.WaitGroup
}

func newBlockProducerGroup(concurrency int) *blockProducerGroup {
	return &blockProducerGroup{semaphore: make(chan struct{}, concurrency)}
}

func (group *blockProducerGroup) wait() {
	group.waitGroup.Wait()
}

// Data holds block data and numbers for batch processing
type Data struct {
	blockData    []interface{}
	blockNumbers []int
	err          error
}

type batchBlock struct {
	block  models.ZondDatabaseBlock
	number int
}

type batchParentMismatchError struct {
	blockNumber          int
	expected             string
	actual               string
	canonicalPredecessor bool
}

func (e *batchParentMismatchError) Error() string {
	return fmt.Sprintf("block %d parent mismatch: expected %s, got %s",
		e.blockNumber, e.expected, e.actual)
}

func parseBatchBlockNumber(blockNumber string) (int, error) {
	if !strings.HasPrefix(blockNumber, "0x") || len(blockNumber) == 2 {
		return 0, fmt.Errorf("invalid block number %q", blockNumber)
	}
	value, err := strconv.ParseUint(blockNumber[2:], 16, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("invalid block number %q: %w", blockNumber, err)
	}
	return int(value), nil
}

func validateFetchedBlock(requested string, block *models.ZondDatabaseBlock) error {
	if block == nil {
		return fmt.Errorf("RPC returned a nil block for %s", requested)
	}
	requestedNumber, err := parseBatchBlockNumber(requested)
	if err != nil {
		return err
	}
	returnedNumber, err := parseBatchBlockNumber(block.Result.Number)
	if err != nil {
		return fmt.Errorf("RPC block for %s has %w", requested, err)
	}
	if returnedNumber != requestedNumber {
		return fmt.Errorf("RPC returned block %s for requested height %s", block.Result.Number, requested)
	}
	if block.Result.Number != requested {
		return fmt.Errorf("RPC returned non-canonical block number %s for requested height %s",
			block.Result.Number, requested)
	}
	if block.Result.Hash == "" {
		return fmt.Errorf("RPC block %s has an empty hash", requested)
	}
	if block.Result.ParentHash == "" {
		return fmt.Errorf("RPC block %s has an empty parent hash", requested)
	}
	for index, tx := range block.Result.Transactions {
		if tx.Hash == "" {
			return fmt.Errorf("RPC block %s transaction %d has an empty hash", requested, index)
		}
		if tx.BlockNumber != block.Result.Number {
			return fmt.Errorf("RPC block %s transaction %s reports block number %s",
				requested, tx.Hash, tx.BlockNumber)
		}
		if !strings.EqualFold(tx.BlockHash, block.Result.Hash) {
			return fmt.Errorf("RPC block %s transaction %s reports block hash %s, want %s",
				requested, tx.Hash, tx.BlockHash, block.Result.Hash)
		}
	}
	return nil
}

func prepareBatch(data Data) ([]batchBlock, error) {
	if len(data.blockData) != len(data.blockNumbers) {
		return nil, fmt.Errorf("batch data length mismatch: %d blocks for %d requested heights",
			len(data.blockData), len(data.blockNumbers))
	}
	prepared := make([]batchBlock, 0, len(data.blockData))
	for index, entry := range data.blockData {
		block, ok := entry.(models.ZondDatabaseBlock)
		if !ok {
			return nil, fmt.Errorf("batch entry %d has type %T", index, entry)
		}
		requested := data.blockNumbers[index]
		if requested < 0 {
			return nil, fmt.Errorf("batch entry %d has negative requested height %d", index, requested)
		}
		if err := validateFetchedBlock(utils.IntToHex(requested), &block); err != nil {
			return nil, err
		}
		prepared = append(prepared, batchBlock{block: block, number: requested})
	}
	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].number < prepared[j].number
	})
	for index := 1; index < len(prepared); index++ {
		if prepared[index].number != prepared[index-1].number+1 {
			return nil, fmt.Errorf("batch heights are not contiguous: %d followed by %d",
				prepared[index-1].number, prepared[index].number)
		}
	}
	return prepared, nil
}

func validateBatchParentLinks(blocks []batchBlock, predecessorHash string) error {
	if len(blocks) == 0 {
		return nil
	}
	if blocks[0].number > 0 {
		if predecessorHash == "" {
			return fmt.Errorf("canonical predecessor for block %d is missing", blocks[0].number)
		}
		if !strings.EqualFold(blocks[0].block.Result.ParentHash, predecessorHash) {
			return &batchParentMismatchError{
				blockNumber:          blocks[0].number,
				expected:             predecessorHash,
				actual:               blocks[0].block.Result.ParentHash,
				canonicalPredecessor: true,
			}
		}
	}
	for index := 1; index < len(blocks); index++ {
		previous := blocks[index-1]
		current := blocks[index]
		if current.number != previous.number+1 {
			return fmt.Errorf("batch heights are not contiguous: %d followed by %d",
				previous.number, current.number)
		}
		if !strings.EqualFold(current.block.Result.ParentHash, previous.block.Result.Hash) {
			return &batchParentMismatchError{
				blockNumber: current.number,
				expected:    previous.block.Result.Hash,
				actual:      current.block.Result.ParentHash,
			}
		}
	}
	return nil
}

func ingestBatch(data Data) ([]int, error) {
	return ingestBatchWithReorgPolicy(data, false)
}

func ingestBatchWithReorgPolicy(data Data, recoverCanonicalReorg bool) ([]int, error) {
	prepared, err := prepareBatch(data)
	if err != nil {
		return nil, errors.Join(err, data.err)
	}
	if len(prepared) == 0 {
		return nil, data.err
	}

	lockChainMutation()
	defer chainMutationMu.Unlock()
	return ingestPreparedBatchLockedWithReorgPolicy(prepared, data.err, recoverCanonicalReorg)
}

type blockCompanionOperations struct {
	reset         func(string) error
	process       func(models.ZondDatabaseBlock) error
	updatePending func(*models.ZondDatabaseBlock) error
	markComplete  func(string, string) error
	storeSync     func(string) error
}

func completeConfirmedCompanionPrefix(
	confirmedBlocks []db.ConfirmedBlockWrite,
	operations blockCompanionOperations,
) ([]int, error) {
	confirmedNumbers := make([]int, 0, len(confirmedBlocks))
	var companionErr error
	for _, confirmed := range confirmedBlocks {
		number, parseErr := parseBatchBlockNumber(confirmed.Block.Result.Number)
		if parseErr != nil {
			companionErr = parseErr
			break
		}
		if confirmed.CompanionsComplete {
			configs.Logger.Debug("Block identity and companions were already complete",
				zap.String("block", confirmed.Block.Result.Number),
				zap.String("hash", confirmed.Block.Result.Hash))
			confirmedNumbers = append(confirmedNumbers, number)
			continue
		}

		if err := operations.reset(confirmed.Block.Result.Number); err != nil {
			companionErr = fmt.Errorf("reset interrupted companions for block %s: %w",
				confirmed.Block.Result.Number, err)
			break
		}
		if err := operations.process(confirmed.Block); err != nil {
			companionErr = fmt.Errorf("process companions for block %s: %w",
				confirmed.Block.Result.Number, err)
			break
		}
		block := confirmed.Block
		if err := operations.updatePending(&block); err != nil {
			companionErr = fmt.Errorf("update pending transactions for block %s: %w",
				block.Result.Number, err)
			break
		}
		if err := operations.markComplete(block.Result.Number, block.Result.Hash); err != nil {
			companionErr = fmt.Errorf("mark companions complete for block %s: %w",
				block.Result.Number, err)
			break
		}
		confirmedNumbers = append(confirmedNumbers, number)
	}

	var syncStateErr error
	if len(confirmedNumbers) > 0 {
		lastConfirmed := utils.IntToHex(confirmedNumbers[len(confirmedNumbers)-1])
		syncStateErr = operations.storeSync(lastConfirmed)
	}
	return confirmedNumbers, errors.Join(companionErr, syncStateErr)
}

func ingestPreparedBatchLocked(prepared []batchBlock, batchErr error) ([]int, error) {
	return ingestPreparedBatchLockedWithReorgPolicy(prepared, batchErr, false)
}

func recoverBatchCanonicalReorgLocked(
	mismatch *batchParentMismatchError,
	durableCursor string,
	rollback func(string) error,
) error {
	if mismatch == nil {
		return errors.New("batch parent mismatch is required for reorg recovery")
	}
	if !mismatch.canonicalPredecessor {
		return mismatch
	}
	if mismatch.blockNumber <= 1 {
		return fmt.Errorf("%w: immutable genesis predecessor cannot be replaced", mismatch)
	}
	predecessorNumber := utils.IntToHex(mismatch.blockNumber - 1)
	if durableCursor != predecessorNumber {
		return fmt.Errorf("%w: durable cursor %s does not match predecessor %s",
			mismatch, durableCursor, predecessorNumber)
	}
	rollbackTarget := utils.IntToHex(mismatch.blockNumber - 2)
	if err := rollback(rollbackTarget); err != nil {
		return errors.Join(mismatch, fmt.Errorf("rollback stale canonical suffix to %s: %w",
			rollbackTarget, err))
	}
	return fmt.Errorf("%w: rolled back stale canonical suffix to %s; retry with fresh blocks",
		mismatch, rollbackTarget)
}

func ingestPreparedBatchLockedWithReorgPolicy(
	prepared []batchBlock,
	batchErr error,
	recoverCanonicalReorg bool,
) ([]int, error) {
	predecessorHash := ""
	var err error
	if prepared[0].number > 0 {
		predecessorNumber := utils.IntToHex(prepared[0].number - 1)
		predecessorHash, err = db.GetCanonicalBlockHash(predecessorNumber)
		if err != nil {
			return nil, fmt.Errorf("read canonical predecessor %s: %w", predecessorNumber, err)
		}
	}
	if err := validateBatchParentLinks(prepared, predecessorHash); err != nil {
		var mismatch *batchParentMismatchError
		if recoverCanonicalReorg && errors.As(err, &mismatch) && mismatch.canonicalPredecessor {
			durableCursor, cursorErr := db.GetLastKnownBlockNumberStrict()
			if cursorErr != nil {
				return nil, errors.Join(err,
					fmt.Errorf("read durable cursor before batch reorg recovery: %w", cursorErr))
			}
			recoveryErr := recoverBatchCanonicalReorgLocked(mismatch, durableCursor, db.Rollback)
			configs.Logger.Warn("Batch parent mismatch stopped sync for canonical recovery",
				zap.Int("block", mismatch.blockNumber),
				zap.String("durable_cursor", durableCursor),
				zap.Error(recoveryErr))
			return nil, recoveryErr
		}
		return nil, err
	}

	blocks := make([]models.ZondDatabaseBlock, len(prepared))
	for index := range prepared {
		blocks[index] = prepared[index].block
	}
	writeResult, writeErr := db.InsertManyBlockDocuments(blocks)
	confirmedNumbers, companionErr := completeConfirmedCompanionPrefix(
		writeResult.Confirmed,
		blockCompanionOperations{
			reset:         db.ResetBlockCompanionRows,
			process:       db.ProcessTransactions,
			updatePending: UpdatePendingTransactionsInBlock,
			markComplete:  db.MarkBlockCompanionsComplete,
			storeSync:     db.StoreLastKnownBlockNumber,
		},
	)
	var conflictRecoveryErr error
	var conflict *db.BlockHeightConflictError
	if errors.As(writeErr, &conflict) {
		pendingOnly, stateErr := db.BlockHeightHasOnlyPendingRows(conflict.Number)
		if stateErr != nil {
			conflictRecoveryErr = fmt.Errorf("inspect conflicting block %s: %w", conflict.Number, stateErr)
		} else if pendingOnly {
			conflictNumber, parseErr := parseBatchBlockNumber(conflict.Number)
			if parseErr != nil {
				conflictRecoveryErr = parseErr
			} else if conflictNumber == 0 {
				conflictRecoveryErr = fmt.Errorf("pending genesis conflicts with fetched canonical identity")
			} else {
				rollbackTarget := utils.IntToHex(conflictNumber - 1)
				if err := db.Rollback(rollbackTarget); err != nil {
					conflictRecoveryErr = fmt.Errorf("remove pending conflicting suffix from %s: %w",
						conflict.Number, err)
				} else {
					configs.Logger.Warn("Removed pending conflicting block suffix for canonical refetch",
						zap.String("conflict_height", conflict.Number),
						zap.String("durable_cursor", rollbackTarget))
				}
			}
		}
	}
	configs.Logger.Info("Confirmed canonical block batch prefix",
		zap.Int("attempted", writeResult.Attempted),
		zap.Int("confirmed", len(writeResult.Confirmed)),
		zap.Int("companions_complete", len(confirmedNumbers)),
		zap.Ints("block_numbers", confirmedNumbers))
	return confirmedNumbers, errors.Join(writeErr, batchErr, companionErr, conflictRecoveryErr)
}

func ingestFetchedBlock(requested string, block *models.ZondDatabaseBlock) error {
	if err := validateFetchedBlock(requested, block); err != nil {
		return err
	}
	number, err := parseBatchBlockNumber(requested)
	if err != nil {
		return err
	}
	if err := db.UpdateTransactionStatuses(block); err != nil {
		return fmt.Errorf("enrich transaction statuses for block %s: %w", requested, err)
	}
	confirmed, err := ingestBatch(Data{
		blockData:    []interface{}{*block},
		blockNumbers: []int{number},
	})
	if err != nil {
		return err
	}
	if len(confirmed) != 1 || confirmed[0] != number {
		return fmt.Errorf("block %s did not reach durable companion completion", requested)
	}
	return nil
}

// ReindexBlockCompanions replays one exact legacy or pending block through the
// durable companion boundary. Callers must process candidates in ascending
// canonical order.
func ReindexBlockCompanions(requested string, block *models.ZondDatabaseBlock) error {
	if err := validateFetchedBlock(requested, block); err != nil {
		return err
	}
	if err := db.UpdateTransactionStatuses(block); err != nil {
		return fmt.Errorf("enrich transaction statuses for block %s: %w", requested, err)
	}
	number, err := parseBatchBlockNumber(requested)
	if err != nil {
		return err
	}
	lockChainMutation()
	defer chainMutationMu.Unlock()
	needsReplay, err := db.PrepareBlockCompanionReindex(requested, block.Result.Hash)
	if err != nil && !errors.Is(err, db.ErrCanonicalBlockNotFound) {
		return err
	}
	if err == nil && !needsReplay {
		return nil
	}
	confirmed, err := ingestPreparedBatchLocked([]batchBlock{{
		block:  *block,
		number: number,
	}}, nil)
	if err != nil {
		return err
	}
	if len(confirmed) != 1 || confirmed[0] != number {
		return fmt.Errorf("block %s did not reach durable companion completion", requested)
	}
	return nil
}

type consumerReport struct {
	err                   error
	highestProcessedBlock int
}

// consumeBatches drains producer channels in range order. Producers still
// fetch in parallel, while database mutations remain deterministic and
// parent-linked. The first unresolved prefix cancels outstanding fetch work.
func consumeBatches(ch <-chan (<-chan Data), cancel func()) consumerReport {
	processedBlocks := make([]int, 0)
	highestProcessedBlock := -1
	halted := false
	var haltErr error

	for producer := range ch {
		for data := range producer {
			if halted {
				continue
			}
			confirmed, err := ingestBatchWithReorgPolicy(data, true)
			processedBlocks = append(processedBlocks, confirmed...)
			for _, blockNumber := range confirmed {
				if blockNumber > highestProcessedBlock {
					highestProcessedBlock = blockNumber
				}
			}
			if err != nil {
				halted = true
				haltErr = err
				cancel()
				configs.Logger.Error("Batch ingestion halted at the first unresolved canonical prefix",
					zap.Error(err),
					zap.Ints("confirmed_prefix", confirmed))
			}
		}
	}

	if highestProcessedBlock >= 0 {
		configs.Logger.Info("Batch processing finished at confirmed canonical height",
			zap.String("block", utils.IntToHex(highestProcessedBlock)))
	}
	if len(processedBlocks) > 1 {
		sort.Ints(processedBlocks)
		minBlock := processedBlocks[0]
		maxBlock := processedBlocks[len(processedBlocks)-1]
		expectedCount := maxBlock - minBlock + 1
		if len(processedBlocks) < expectedCount {
			configs.Logger.Warn("Potential gaps detected during batch processing",
				zap.Int("expected_blocks", expectedCount),
				zap.Int("processed_blocks", len(processedBlocks)),
				zap.Int("min_block", minBlock),
				zap.Int("max_block", maxBlock))
		}
	}
	return consumerReport{err: haltErr, highestProcessedBlock: highestProcessedBlock}
}

func waitForProducerContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// producerWithContext fetches blocks in a range and sends them to a channel.
func (group *blockProducerGroup) producerWithContext(
	ctx context.Context,
	start string,
	end string,
) <-chan Data {
	// Create a channel which we will send our data.
	Datas := make(chan Data, 32)

	var blockData []interface{}
	var blockNumbers []int

	// Start the goroutine that produces data.
	group.waitGroup.Add(1)
	go func(ch chan<- Data) {
		defer group.waitGroup.Done()
		// Acquire a token from the producer semaphore
		select {
		case group.semaphore <- struct{}{}:
		case <-ctx.Done():
			close(ch)
			return
		}
		// Ensure the token is released when this goroutine finishes
		defer func() {
			<-group.semaphore
			close(ch) // Close the channel when done producing
		}()

		// Produce data.
		var batchErr error
		currentBlock := start
		for utils.CompareHexNumbers(currentBlock, end) < 0 {
			// Add reduced delay for bulk sync operations (5-7ms instead of 50-76ms)
			if !waitForProducerContext(ctx, getRPCDelay(true)) {
				return
			}

			// Try to fetch block with retry logic
			var data *models.ZondDatabaseBlock
			var err error
			maxRetries := 3

			for attempt := 1; attempt <= maxRetries; attempt++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				data, err = rpc.GetBlockByNumberMainnet(currentBlock)
				if err == nil {
					err = validateFetchedBlock(currentBlock, data)
				}
				if err == nil {
					err = db.UpdateTransactionStatuses(data)
				}
				if err == nil {
					break // Success
				}

				if attempt < maxRetries {
					backoffDelay := time.Duration(attempt*100) * time.Millisecond
					configs.Logger.Warn("Block fetch failed, retrying",
						zap.String("block", currentBlock),
						zap.Int("attempt", attempt),
						zap.Duration("backoff", backoffDelay),
						zap.Error(err))
					if !waitForProducerContext(ctx, backoffDelay) {
						return
					}
				}
			}

			if err != nil {
				trackFailedBlock(currentBlock, err)
				configs.Logger.Error("Failed to get block data after retries",
					zap.String("block", currentBlock),
					zap.Int("max_retries", maxRetries),
					zap.Error(err))
				batchErr = fmt.Errorf("fetch block %s: %w", currentBlock, err)
				break
			}

			// Success - clear any previous failure tracking
			clearFailedBlock(currentBlock)

			blockData = append(blockData, *data)
			blockNumbers = append(blockNumbers, int(utils.HexToInt(currentBlock).Int64()))
			currentBlock = utils.AddHexNumbers(currentBlock, "0x1")
		}
		if len(blockData) > 0 || batchErr != nil {
			select {
			case ch <- Data{blockData: blockData, blockNumbers: blockNumbers, err: batchErr}:
			case <-ctx.Done():
			}
		}
	}(Datas)

	return Datas
}

// batchSync ingests the inclusive range [fromBlock, toBlock] and returns the
// confirmed durable high-water mark.
func batchSync(fromBlock string, toBlock string) (string, error) {
	if utils.CompareHexNumbers(fromBlock, toBlock) > 0 {
		configs.Logger.Error("Invalid block range for batch sync",
			zap.String("from_block", fromBlock),
			zap.String("to_block", toBlock))
		return db.GetLastKnownBlockNumber(), fmt.Errorf("invalid batch range %s through %s", fromBlock, toBlock)
	}

	configs.Logger.Info("Starting batch sync",
		zap.String("from_block", fromBlock),
		zap.String("to_block", toBlock))

	lastKnownBlock := db.GetLastKnownBlockNumber()
	if utils.CompareHexNumbers(lastKnownBlock, fromBlock) >= 0 {
		if utils.CompareHexNumbers(lastKnownBlock, toBlock) >= 0 {
			configs.Logger.Info("Requested batch range is already durably processed",
				zap.String("last_known_block", lastKnownBlock),
				zap.String("requested_to_block", toBlock))
			return lastKnownBlock, nil
		}
		fromBlock = utils.AddHexNumbers(lastKnownBlock, "0x1")
		configs.Logger.Info("Trimmed batch range to the durable sync state",
			zap.String("last_known_block", lastKnownBlock),
			zap.String("next_block", fromBlock))
	}

	wg := sync.WaitGroup{}

	producerGroup := newBlockProducerGroup(MaxProducerConcurrency)

	// Create buffered channel for producers
	producers := make(chan (<-chan Data), 32)
	batchCtx, cancelBatch := context.WithCancel(context.Background())
	defer cancelBatch()

	// Start the consumer
	var report consumerReport
	wg.Add(1)
	go func() {
		defer wg.Done()
		report = consumeBatches(producers, cancelBatch)
	}()

	// Use larger batch size when far behind
	batchSize := DefaultBatchSize
	if utils.CompareHexNumbers(utils.SubtractHexNumbers(toBlock, fromBlock), utils.IntToHex(LargeSyncThreshold)) > 0 {
		batchSize = LargeBatchSize
	}

	// Start producers in batches with retry logic
	currentBlock := fromBlock
	toBlockExclusive := utils.AddHexNumbers(toBlock, "0x1")
	lastScheduledBlock := utils.SubtractHexNumbers(fromBlock, "0x1")

scheduleBatches:
	for utils.CompareHexNumbers(currentBlock, toBlockExclusive) < 0 {
		select {
		case <-batchCtx.Done():
			break scheduleBatches
		default:
		}
		endBlock := utils.AddHexNumbers(currentBlock, utils.IntToHex(batchSize))
		if utils.CompareHexNumbers(endBlock, toBlockExclusive) > 0 {
			endBlock = toBlockExclusive
		}

		producerChan := producerGroup.producerWithContext(batchCtx, currentBlock, endBlock)
		select {
		case producers <- producerChan:
		case <-batchCtx.Done():
			break scheduleBatches
		}
		configs.Logger.Info("Processing block range",
			zap.String("from", currentBlock),
			zap.String("to", utils.SubtractHexNumbers(endBlock, "0x1")))

		lastScheduledBlock = utils.SubtractHexNumbers(endBlock, "0x1")
		currentBlock = endBlock
	}

	close(producers)
	wg.Wait()
	cancelBatch()
	producerGroup.wait()
	if report.err != nil {
		lastKnownBlock = db.GetLastKnownBlockNumber()
		configs.Logger.Error("batchSync stopped at its confirmed canonical prefix",
			zap.String("confirmed_sync_state", lastKnownBlock),
			zap.Error(report.err))
		return lastKnownBlock, report.err
	}

	// After batch sync completes, verify what the actual last synced block is
	lastKnownBlock = db.GetLastKnownBlockNumber()
	configs.Logger.Info("batchSync completed",
		zap.String("requested_to_block", toBlock),
		zap.String("last_scheduled_block", lastScheduledBlock),
		zap.String("db_last_known_block", lastKnownBlock))

	if utils.CompareHexNumbers(lastKnownBlock, toBlock) < 0 {
		unresolvedErr := fmt.Errorf("%w: batch ended at %s before requested height %s",
			db.ErrBlockWriteUnresolved, lastKnownBlock, toBlock)
		configs.Logger.Error("Batch scheduling ended beyond the confirmed sync state",
			zap.String("scheduled_end", lastScheduledBlock),
			zap.String("confirmed_sync_state", lastKnownBlock),
			zap.Error(unresolvedErr))
		return lastKnownBlock, unresolvedErr
	}

	// Process all token transfers once after all batches are completed
	tokenQueueErr := processQueuedTokenContracts()
	if tokenQueueErr != nil {
		configs.Logger.Error("Queued token processing completed with retryable failures",
			zap.Error(tokenQueueErr))
	}

	configs.Logger.Info("Final sync state verification")
	highestBlock := findHighestProcessedBlock()
	if utils.CompareHexNumbers(highestBlock, lastKnownBlock) > 0 {
		configs.Logger.Warn("Found block rows beyond the confirmed sync state",
			zap.String("current_sync_state", lastKnownBlock),
			zap.String("highest_processed_block", highestBlock))
	}

	return lastKnownBlock, nil
}
