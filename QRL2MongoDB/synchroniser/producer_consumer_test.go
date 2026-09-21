package synchroniser

import (
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func batchTestBlock(number, hash, parentHash string) models.ZondDatabaseBlock {
	return models.ZondDatabaseBlock{Result: models.Result{
		Number:     number,
		Hash:       hash,
		ParentHash: parentHash,
	}}
}

func TestPrepareBatchSortsAndValidatesRequestedHeights(t *testing.T) {
	block2 := batchTestBlock("0x2", "0xhash2", "0xhash1")
	block3 := batchTestBlock("0x3", "0xhash3", "0xhash2")
	prepared, err := prepareBatch(Data{
		blockData:    []interface{}{block3, block2},
		blockNumbers: []int{3, 2},
	})
	if err != nil {
		t.Fatalf("prepareBatch() error = %v", err)
	}
	if len(prepared) != 2 || prepared[0].number != 2 || prepared[1].number != 3 {
		t.Fatalf("prepared order = %#v, want heights 2, 3", prepared)
	}
	if err := validateBatchParentLinks(prepared, "0xhash1"); err != nil {
		t.Fatalf("validateBatchParentLinks() error = %v", err)
	}
}

func TestValidateBatchParentLinksRejectsWrongFirstParent(t *testing.T) {
	prepared := []batchBlock{{
		block:  batchTestBlock("0x2", "0xhash2", "0xwrong"),
		number: 2,
	}}
	err := validateBatchParentLinks(prepared, "0xhash1")
	if err == nil || !strings.Contains(err.Error(), "block 2 parent mismatch") {
		t.Fatalf("error = %v, want first-parent mismatch", err)
	}
}

func TestValidateBatchParentLinksRejectsInternalMismatch(t *testing.T) {
	prepared := []batchBlock{
		{block: batchTestBlock("0x2", "0xhash2", "0xhash1"), number: 2},
		{block: batchTestBlock("0x3", "0xhash3", "0xwrong"), number: 3},
	}
	err := validateBatchParentLinks(prepared, "0xhash1")
	if err == nil || !strings.Contains(err.Error(), "block 3 parent mismatch") {
		t.Fatalf("error = %v, want internal-parent mismatch", err)
	}
}

func TestBatchModeRecoversShallowReorgAtLargeLag(t *testing.T) {
	const durableHead int64 = 100
	const networkHead int64 = durableHead + BatchSyncThreshold + 1
	if !shouldUseBatchSync(durableHead, networkHead) {
		t.Fatal("test setup does not select batch sync")
	}

	mismatch := &batchParentMismatchError{
		blockNumber:          int(durableHead + 1),
		expected:             "0xstale-parent",
		actual:               "0xcanonical-parent",
		canonicalPredecessor: true,
	}
	rollbackTarget := ""
	err := recoverBatchCanonicalReorgLocked(
		mismatch,
		"0x64",
		func(target string) error {
			rollbackTarget = target
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "retry with fresh blocks") {
		t.Fatalf("error = %v, want retry signal", err)
	}
	if rollbackTarget != "0x63" {
		t.Fatalf("rollback target = %s, want 0x63", rollbackTarget)
	}
}

func TestBatchModeDoesNotRollbackInternalParentMismatch(t *testing.T) {
	rollbackCalled := false
	err := recoverBatchCanonicalReorgLocked(
		&batchParentMismatchError{
			blockNumber: 3,
			expected:    "0xhash2",
			actual:      "0xwrong",
		},
		"0x2",
		func(string) error {
			rollbackCalled = true
			return nil
		},
	)
	if err == nil {
		t.Fatal("internal mismatch was accepted")
	}
	if rollbackCalled {
		t.Fatal("internal batch mismatch triggered canonical rollback")
	}
}

func TestBatchModePropagatesRollbackFailure(t *testing.T) {
	rollbackFailure := errors.New("rollback failed")
	err := recoverBatchCanonicalReorgLocked(
		&batchParentMismatchError{
			blockNumber:          101,
			expected:             "0xstale-parent",
			actual:               "0xcanonical-parent",
			canonicalPredecessor: true,
		},
		"0x64",
		func(string) error { return rollbackFailure },
	)
	if !errors.Is(err, rollbackFailure) {
		t.Fatalf("error = %v, want rollback failure", err)
	}
}

func TestBatchModeRefusesRollbackWhenCursorIsNotPredecessor(t *testing.T) {
	rollbackCalled := false
	err := recoverBatchCanonicalReorgLocked(
		&batchParentMismatchError{
			blockNumber:          101,
			expected:             "0xstale-parent",
			actual:               "0xcanonical-parent",
			canonicalPredecessor: true,
		},
		"0x63",
		func(string) error {
			rollbackCalled = true
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "does not match predecessor") {
		t.Fatalf("error = %v, want cursor mismatch", err)
	}
	if rollbackCalled {
		t.Fatal("cursor mismatch triggered rollback")
	}
}

func TestBatchModePreservesGenesisOnBlockOneMismatch(t *testing.T) {
	rollbackCalled := false
	err := recoverBatchCanonicalReorgLocked(
		&batchParentMismatchError{
			blockNumber:          1,
			expected:             "0xgenesis",
			actual:               "0xother",
			canonicalPredecessor: true,
		},
		"0x0",
		func(string) error {
			rollbackCalled = true
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "immutable genesis") {
		t.Fatalf("error = %v, want immutable genesis refusal", err)
	}
	if rollbackCalled {
		t.Fatal("block one mismatch triggered genesis rollback")
	}
}

func TestConsumerCancelsLaterProducerAfterUnresolvedPrefix(t *testing.T) {
	producerGroup := newBlockProducerGroup(1)
	producerGroup.semaphore <- struct{}{}
	defer func() {
		<-producerGroup.semaphore
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan Data, 1)
	first <- Data{
		blockData:    []interface{}{batchTestBlock("0x2", "0xhash2", "0xhash1")},
		blockNumbers: []int{1},
	}
	close(first)
	later := producerGroup.producerWithContext(ctx, "0x2", "0x3")
	producers := make(chan (<-chan Data), 2)
	producers <- first
	producers <- later
	close(producers)

	done := make(chan consumerReport, 1)
	go func() {
		done <- consumeBatches(producers, cancel)
	}()
	select {
	case report := <-done:
		if report.err == nil {
			t.Fatal("consumer accepted unresolved first prefix")
		}
		if ctx.Err() == nil {
			t.Fatal("consumer did not cancel later producer work")
		}
		producerGroup.wait()
	case <-time.After(time.Second):
		t.Fatal("later producer did not stop after batch cancellation")
	}
}

func TestCanceledUnenqueuedProducerDoesNotDependOnNextBatchSemaphore(t *testing.T) {
	firstGroup := newBlockProducerGroup(1)
	firstGroup.semaphore <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	orphaned := firstGroup.producerWithContext(ctx, "0x2", "0x3")
	cancel()

	// A new batch owns a distinct semaphore. Replacing batch state cannot
	// strand the producer created by the canceled batch.
	secondGroup := newBlockProducerGroup(1)
	if firstGroup.semaphore == secondGroup.semaphore {
		t.Fatal("semaphore was shared across batch producer groups")
	}

	done := make(chan struct{})
	go func() {
		firstGroup.wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled unenqueued producer did not exit")
	}
	select {
	case _, open := <-orphaned:
		if open {
			t.Fatal("canceled producer channel remained open")
		}
	default:
		t.Fatal("canceled producer channel was not closed")
	}
	<-firstGroup.semaphore
}

func TestValidateFetchedBlockRejectsRequestResultMismatch(t *testing.T) {
	block := batchTestBlock("0x3", "0xhash3", "0xhash2")
	err := validateFetchedBlock("0x2", &block)
	if err == nil || !strings.Contains(err.Error(), "requested height 0x2") {
		t.Fatalf("error = %v, want request-result mismatch", err)
	}
}

func TestValidateFetchedBlockRejectsTransactionBlockMismatch(t *testing.T) {
	block := batchTestBlock("0x2", "0xhash2", "0xhash1")
	block.Result.Transactions = []models.Transaction{{
		Hash:        "0xtx",
		BlockNumber: "0x3",
		BlockHash:   "0xhash2",
	}}
	err := validateFetchedBlock("0x2", &block)
	if err == nil || !strings.Contains(err.Error(), "reports block number 0x3") {
		t.Fatalf("error = %v, want transaction block-number mismatch", err)
	}

	block.Result.Transactions[0].BlockNumber = "0x2"
	block.Result.Transactions[0].BlockHash = "0xwrong"
	err = validateFetchedBlock("0x2", &block)
	if err == nil || !strings.Contains(err.Error(), "reports block hash 0xwrong") {
		t.Fatalf("error = %v, want transaction block-hash mismatch", err)
	}
}

func TestCompleteConfirmedCompanionPrefixStopsBeforeFailedBlock(t *testing.T) {
	companionFailure := errors.New("companion write failed")
	var calls []string
	operations := blockCompanionOperations{
		reset: func(number string) error {
			calls = append(calls, "reset:"+number)
			return nil
		},
		process: func(block models.ZondDatabaseBlock) error {
			calls = append(calls, "process:"+block.Result.Number)
			if block.Result.Number == "0x2" {
				return companionFailure
			}
			return nil
		},
		updatePending: func(block *models.ZondDatabaseBlock) error {
			calls = append(calls, "pending:"+block.Result.Number)
			return nil
		},
		markComplete: func(number, _ string) error {
			calls = append(calls, "mark:"+number)
			return nil
		},
		storeSync: func(number string) error {
			calls = append(calls, "sync:"+number)
			return nil
		},
	}
	confirmed, err := completeConfirmedCompanionPrefix([]db.ConfirmedBlockWrite{
		{
			Block:              batchTestBlock("0x1", "0xhash1", "0xgenesis"),
			CompanionsComplete: true,
		},
		{Block: batchTestBlock("0x2", "0xhash2", "0xhash1")},
		{Block: batchTestBlock("0x3", "0xhash3", "0xhash2")},
	}, operations)
	if !errors.Is(err, companionFailure) {
		t.Fatalf("error = %v, want companion failure", err)
	}
	if len(confirmed) != 1 || confirmed[0] != 1 {
		t.Fatalf("confirmed prefix = %v, want [1]", confirmed)
	}
	wantCalls := []string{"reset:0x2", "process:0x2", "sync:0x1"}
	if strings.Join(calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
}

func TestCompleteConfirmedCompanionPrefixDoesNotAdvanceOnFirstFailure(t *testing.T) {
	resetFailure := errors.New("reset failed")
	syncCalled := false
	confirmed, err := completeConfirmedCompanionPrefix(
		[]db.ConfirmedBlockWrite{{Block: batchTestBlock("0x1", "0xhash1", "0xgenesis")}},
		blockCompanionOperations{
			reset:         func(string) error { return resetFailure },
			process:       func(models.ZondDatabaseBlock) error { return nil },
			updatePending: func(*models.ZondDatabaseBlock) error { return nil },
			markComplete:  func(string, string) error { return nil },
			storeSync: func(string) error {
				syncCalled = true
				return nil
			},
		},
	)
	if !errors.Is(err, resetFailure) {
		t.Fatalf("error = %v, want reset failure", err)
	}
	if len(confirmed) != 0 {
		t.Fatalf("confirmed prefix = %v, want empty", confirmed)
	}
	if syncCalled {
		t.Fatal("sync state advanced past the failed first companion")
	}
}
