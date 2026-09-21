package main

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/synchroniser"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

const reindexLeaseTTL = 2 * time.Minute

type renewableLease struct {
	mu    sync.Mutex
	lease configs.SyncerLease
}

type candidateLister func(int) ([]db.BlockCompanionReindexCandidate, error)
type candidateProcessor func(string) error
type missingHeightLister func() ([]string, error)

func parseWorkHeight(number string) (uint64, error) {
	if !strings.HasPrefix(number, "0x") || len(number) == 2 {
		return 0, fmt.Errorf("invalid migration work height %q", number)
	}
	height, err := strconv.ParseUint(number[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid migration work height %q: %w", number, err)
	}
	if number != "0x"+strconv.FormatUint(height, 16) {
		return 0, fmt.Errorf("non-canonical migration work height %q", number)
	}
	return height, nil
}

func sortAndValidateMissingHeights(missing []string) ([]string, error) {
	heights := make(map[uint64]string, len(missing))
	for _, number := range missing {
		height, err := parseWorkHeight(number)
		if err != nil {
			return nil, err
		}
		if _, exists := heights[height]; exists {
			return nil, fmt.Errorf("duplicate missing migration height %s", number)
		}
		heights[height] = number
	}
	ordered := append([]string(nil), missing...)
	sort.Slice(ordered, func(i, j int) bool {
		left, _ := parseWorkHeight(ordered[i])
		right, _ := parseWorkHeight(ordered[j])
		return left < right
	})
	return ordered, nil
}

func processMigrationWork(
	limit int,
	pageSize int,
	listCandidates candidateLister,
	listMissing missingHeightLister,
	process candidateProcessor,
) (int, string, error) {
	if pageSize <= 0 {
		return 0, "", errors.New("page size must be positive")
	}
	missing, err := listMissing()
	if err != nil {
		return 0, "", err
	}
	missing, err = sortAndValidateMissingHeights(missing)
	if err != nil {
		return 0, "", err
	}

	processed := 0
	var lastCandidateHeight uint64
	hasLastCandidate := false
	var candidates []db.BlockCompanionReindexCandidate
	for {
		if len(candidates) == 0 {
			candidates, err = listCandidates(pageSize)
			if err != nil {
				return processed, "", err
			}
		}

		next := ""
		fromCandidate := false
		if len(candidates) > 0 {
			next = candidates[0].Number
			if _, err := parseWorkHeight(next); err != nil {
				return processed, next, err
			}
			fromCandidate = true
		}
		if len(missing) > 0 {
			missingHeight, _ := parseWorkHeight(missing[0])
			if next == "" {
				next = missing[0]
				fromCandidate = false
			} else {
				candidateHeight, _ := parseWorkHeight(next)
				if missingHeight < candidateHeight {
					next = missing[0]
					fromCandidate = false
				} else if missingHeight == candidateHeight {
					return processed, next,
						fmt.Errorf("height %s is both stored and reported missing", next)
				}
			}
		}
		if next == "" {
			return processed, "", nil
		}
		if fromCandidate {
			candidateHeight, _ := parseWorkHeight(next)
			if hasLastCandidate && candidateHeight <= lastCandidateHeight {
				return processed, next,
					fmt.Errorf("block companion candidate %s is not above the last processed candidate",
						next)
			}
		}
		if limit > 0 && processed >= limit {
			return processed, next, nil
		}
		if err := process(next); err != nil {
			return processed, next, err
		}
		processed++
		if fromCandidate {
			lastCandidateHeight, _ = parseWorkHeight(next)
			hasLastCandidate = true
			candidates = candidates[1:]
		} else {
			missing = missing[1:]
		}
	}
}

func processCandidatePages(
	limit int,
	pageSize int,
	list candidateLister,
	process candidateProcessor,
) (int, string, error) {
	return processMigrationWork(
		limit,
		pageSize,
		list,
		func() ([]string, error) { return nil, nil },
		process,
	)
}

func (r *renewableLease) current() configs.SyncerLease {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lease
}

func (r *renewableLease) replace(lease configs.SyncerLease) {
	r.mu.Lock()
	r.lease = lease
	r.mu.Unlock()
}

func startLeaseRenewal(lease configs.SyncerLease) func() error {
	state := &renewableLease{lease: lease}
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(reindexLeaseTTL / 3)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				renewed, err := configs.RenewSyncerLease(ctx, state.current(), reindexLeaseTTL)
				cancel()
				if err != nil {
					configs.Logger.Fatal("Lost exclusive companion reindex lease; terminating immediately",
						zap.Error(err))
				}
				state.replace(renewed)
				synchroniser.ConfigureChainMutationLease(renewed.ExpiresAt)
			}
		}
	}()
	stop := func() error {
		close(stopCh)
		<-doneCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return configs.ReleaseSyncerLease(ctx, state.current())
	}
	return stop
}

func run() error {
	execute := flag.Bool("execute", false, "rebuild companions and stamp completion markers")
	limit := flag.Int("limit", 0, "maximum blocks to process in this resumable run (0 means all)")
	flag.Parse()
	if *limit < 0 {
		return errors.New("limit cannot be negative")
	}

	configs.LoadEnv()
	if err := configs.ConnectDB(); err != nil {
		return fmt.Errorf("connect to MongoDB: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = configs.DB.Disconnect(ctx)
	}()

	if !*execute {
		rowCount, err := db.CountBlockCompanionReindexRows()
		if err != nil {
			return fmt.Errorf("inventory block companion reindex: %w", err)
		}
		first, err := db.ListBlockCompanionReindexCandidates(1)
		if err != nil {
			return fmt.Errorf("inventory first block companion reindex candidate: %w", err)
		}
		missing, err := db.AuditStoredCanonicalBlockGaps()
		if err != nil {
			return fmt.Errorf("audit stored canonical gaps: %w", err)
		}
		firstWork := ""
		if len(first) > 0 {
			firstWork = first[0].Number
		}
		if len(missing) > 0 {
			if firstWork == "" {
				firstWork = missing[0]
			} else {
				candidateHeight, _ := parseWorkHeight(firstWork)
				missingHeight, _ := parseWorkHeight(missing[0])
				if missingHeight < candidateHeight {
					firstWork = missing[0]
				}
			}
		}
		fmt.Printf("block companion reindex dry run: %d non-complete rows, %d missing heights",
			rowCount, len(missing))
		if firstWork != "" {
			fmt.Printf(", first block=%s", firstWork)
		}
		fmt.Println()
		fmt.Println("dry run only: rerun with --execute while the normal syncer is stopped")
		return nil
	}
	if os.Getenv("NODE_URLS") == "" && os.Getenv("NODE_URL") == "" {
		return errors.New("NODE_URLS or NODE_URL is required for --execute")
	}

	owner, err := configs.NewSyncerLeaseOwner()
	if err != nil {
		return err
	}
	leaseCtx, leaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	lease, err := configs.AcquireSyncerLease(leaseCtx, owner, reindexLeaseTTL)
	leaseCancel()
	if err != nil {
		return fmt.Errorf("acquire exclusive syncer lease: %w", err)
	}
	synchroniser.ConfigureChainMutationLease(lease.ExpiresAt)
	stopRenewal := startLeaseRenewal(lease)
	defer func() {
		if err := stopRenewal(); err != nil {
			configs.Logger.Error("Failed to release companion reindex lease", zap.Error(err))
		}
	}()
	if err := configs.BootstrapDB(); err != nil {
		return fmt.Errorf("bootstrap migration indexes under exclusive lease: %w", err)
	}
	runCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	const pageSize = 256
	processBlock := func(blockNumber string) error {
		select {
		case <-runCtx.Done():
			return runCtx.Err()
		default:
		}
		block, err := rpc.GetBlockByNumberMainnet(blockNumber)
		if err != nil {
			return fmt.Errorf("fetch block %s: %w", blockNumber, err)
		}
		if err := synchroniser.ReindexBlockCompanions(blockNumber, block); err != nil {
			return fmt.Errorf("reindex block %s: %w", blockNumber, err)
		}
		configs.Logger.Info("Reindexed block companions",
			zap.String("block", blockNumber))
		return nil
	}

	_, nextCandidate, err := processMigrationWork(
		*limit,
		pageSize,
		db.ListBlockCompanionReindexCandidates,
		db.AuditStoredCanonicalBlockGaps,
		processBlock,
	)
	if err != nil {
		return fmt.Errorf("process block companion reindex pages: %w", err)
	}
	if nextCandidate != "" {
		fmt.Printf("block companion reindex checkpoint complete; next block=%s (rerun to continue)\n",
			nextCandidate)
		return nil
	}
	missing, err := db.AuditCompletedCanonicalBlockChain()
	if err != nil {
		return fmt.Errorf("final canonical chain audit: %w", err)
	}
	if len(missing) > 0 {
		return fmt.Errorf("final canonical chain audit still has %d missing heights", len(missing))
	}
	if err := db.EnsureUniqueBlockHeightIndex(); err != nil {
		return fmt.Errorf("enforce unique canonical block heights: %w", err)
	}
	if err := db.MarkBlockIngestionMigrationComplete(db.GetLatestBlockNumberFromDB()); err != nil {
		return fmt.Errorf("write canonical companion audit attestation: %w", err)
	}
	if err := db.ValidateBlockIngestionMigration(); err != nil {
		return err
	}
	fmt.Println("block companion reindex complete: migration gate is clear")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
