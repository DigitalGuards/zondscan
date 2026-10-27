package synchroniser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var errChainTimingConflict = errors.New("chain timing conflict; inspect chain database")

type chainTimingSource interface {
	ChainTiming(context.Context) (models.ChainTiming, error)
}

type chainTimingStore interface {
	Load(context.Context) (models.ChainTiming, bool, error)
	CursorGenesis(context.Context) (int64, bool, error)
	Save(context.Context, models.ChainTiming) error
}

// Closing ready publishes immutable timing and its client to every worker.
type chainTimingState struct {
	ready    chan struct{}
	once     sync.Once
	timing   models.ChainTiming
	beacon   *rpc.SlotAssignmentClient
	verified bool // Owned by initialization and then the single retry worker.
}

func newChainTimingState() *chainTimingState {
	return &chainTimingState{ready: make(chan struct{})}
}

var chainClock = newChainTimingState()

// initialize permits execution indexing while the beacon is unavailable.
// Stored timing and the cursor are checked under the chain writer lease.
func (s *chainTimingState) initialize(ctx context.Context, source chainTimingSource, store chainTimingStore) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	fetched, fetchErr := source.ChainTiming(fetchCtx)
	cancel()
	var timing models.ChainTiming
	var known bool
	err := slotAssignmentMutation(ctx, func(ctx context.Context) error {
		stored, exists, err := store.Load(ctx)
		if err != nil {
			return err
		}
		if exists {
			if err := stored.Validate(); err != nil {
				return fmt.Errorf("%w: invalid stored timing", errChainTimingConflict)
			}
		}
		genesis, cursorExists, err := store.CursorGenesis(ctx)
		if err != nil {
			return err
		}
		timing, known = stored, exists
		if fetchErr == nil {
			if err := fetched.Validate(); err != nil {
				return err
			}
			if exists && stored != fetched {
				return fmt.Errorf("%w: stored values disagree with beacon", errChainTimingConflict)
			}
			timing, known = fetched, true
		}
		if cursorExists && (genesis <= 0 || (known && genesis != timing.GenesisTime)) {
			return fmt.Errorf("%w: slot assignment genesis disagrees", errChainTimingConflict)
		}
		if fetchErr == nil {
			return store.Save(ctx, timing)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("load chain timing: %w", err)
	}
	if fetchErr != nil {
		if known {
			configs.Logger.Warn("Beacon timing unavailable; using stored chain timing", zap.Error(fetchErr))
		} else {
			configs.Logger.Warn("Beacon timing unavailable; execution indexing will continue while beacon workers wait", zap.Error(fetchErr))
		}
	}
	if known {
		s.once.Do(func() {
			s.timing = timing
			s.beacon = rpc.NewSlotAssignmentClient(os.Getenv("BEACONCHAIN_API"), timing)
			close(s.ready)
			configs.Logger.Info("Loaded chain timing", zap.Int64("slotsPerEpoch", timing.SlotsPerEpoch), zap.Int64("secondsPerSlot", timing.SecondsPerSlot), zap.Int64("genesisTime", timing.GenesisTime))
		})
	}
	s.verified = fetchErr == nil
	return nil
}

func InitializeChainTiming() error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return chainClock.initialize(ctx, rpc.NewSlotAssignmentClient(os.Getenv("BEACONCHAIN_API"), models.ChainTiming{}), mongoChainTimingStore{})
}

// Retry discovery and verify any cached clock once the beacon recovers. A
// conflict discovered later is delivered to main for orderly shutdown.
func StartChainTimingSync(stopCh <-chan struct{}) <-chan error {
	errorsCh := make(chan error, 1)
	startBackgroundWorker(func() {
		ctx, cancel := beaconWorkerContext(stopCh)
		defer cancel()
		source := rpc.NewSlotAssignmentClient(os.Getenv("BEACONCHAIN_API"), models.ChainTiming{})
		select {
		case <-chainClock.ready:
			// Cached timing workers and revalidation share the request limiter.
			source = chainClock.beacon
		default:
		}
		if err := chainClock.retry(ctx, 30*time.Second, source, mongoChainTimingStore{}); err != nil {
			errorsCh <- err
		}
	})
	return errorsCh
}

func (s *chainTimingState) retry(ctx context.Context, interval time.Duration, source chainTimingSource, store chainTimingStore) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for !s.verified {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		if ctx.Err() != nil {
			return nil
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		err := s.initialize(attemptCtx, source, store)
		cancel()
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errChainTimingConflict) {
			return err
		}
		if err != nil {
			configs.Logger.Warn("Chain timing discovery will retry", zap.Error(err))
		}
	}
	return nil
}

func (s *chainTimingState) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ready:
		return ctx.Err()
	}
}

type mongoChainTimingStore struct{}

func (mongoChainTimingStore) Load(ctx context.Context) (models.ChainTiming, bool, error) {
	var timing models.ChainTiming
	err := configs.GetCollection(configs.DB, "sync_state").FindOne(ctx, bson.M{"_id": models.ChainTimingID}).Decode(&timing)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return timing, false, nil
	}
	return timing, err == nil, err
}

func (mongoChainTimingStore) CursorGenesis(ctx context.Context) (int64, bool, error) {
	var cursor models.SlotAssignmentCursor
	err := configs.GetCollection(configs.DB, "sync_state").FindOne(ctx, bson.M{"_id": models.SlotAssignmentCursorID}).Decode(&cursor)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, false, nil
	}
	return cursor.GenesisTime, err == nil, err
}

func (mongoChainTimingStore) Save(ctx context.Context, timing models.ChainTiming) error {
	_, err := configs.GetCollection(configs.DB, "sync_state").UpdateOne(ctx,
		bson.M{"_id": models.ChainTimingID}, bson.M{"$set": timing}, options.Update().SetUpsert(true))
	return err
}

func beaconWorkerContext(stopCh <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	select {
	case <-stopCh:
		cancel()
	default:
		go func() {
			select {
			case <-stopCh:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	return ctx, cancel
}
