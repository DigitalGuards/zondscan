package rpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"QRL2MongoDB/models"
)

var ErrEpochUnavailable = errors.New("beacon historical state unavailable")

const maxBeaconResponse = 8 << 20

// SlotAssignmentClient is used by one sequential walker. Every HTTP request,
// including pagination and fallback lookups, shares the rate and timeout caps.
type SlotAssignmentClient struct {
	baseURL     string
	client      *http.Client
	interval    time.Duration
	nextRequest time.Time
	pubkeys     map[string]string
}

func NewSlotAssignmentClient(baseURL string) *SlotAssignmentClient {
	return &SlotAssignmentClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   &http.Client{Timeout: 10 * time.Second},
		interval: time.Second,
		pubkeys:  make(map[string]string),
	}
}

type beaconStatusError struct {
	status      int
	unavailable bool
}

func (e *beaconStatusError) Error() string {
	return fmt.Sprintf("beacon HTTP %d (historical state unavailable: %t)", e.status, e.unavailable)
}

// Only explicit missing-history responses may advance the unavailable prefix.
// Generic 404s, server failures, timeouts and invalid JSON remain retryable.
func missingHistoricalState(body string) bool {
	body = strings.ToLower(body)
	for _, phrase := range []string{
		"state not found", "state does not exist", "state is not available",
		"no state available", "no state found", "could not find state",
		"unknown state", "historical state unavailable", "state has been pruned",
		"unknown boundary state", "no blocks found in db below slot",
		"cannot retrieve data for slot", "no data for slot", "outside the historical",
	} {
		if strings.Contains(body, phrase) {
			return true
		}
	}
	return false
}

func (c *SlotAssignmentClient) get(ctx context.Context, path string, result any) error {
	if c.baseURL == "" {
		return errors.New("BEACONCHAIN_API is required for slot assignments")
	}
	if wait := time.Until(c.nextRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.nextRequest = time.Now().Add(c.interval)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return errors.New("invalid beacon API URL")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		// Transport errors can include private URLs. Keep logs endpoint-free.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("beacon request failed or timed out")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBeaconResponse+1))
	if err != nil {
		return errors.New("read beacon response failed")
	}
	if len(body) > maxBeaconResponse {
		return errors.New("beacon response exceeds size limit")
	}
	if resp.StatusCode != http.StatusOK {
		return &beaconStatusError{status: resp.StatusCode,
			unavailable: (resp.StatusCode == 400 || resp.StatusCode == 404 || resp.StatusCode == 500) && missingHistoricalState(string(body))}
	}
	if err := json.Unmarshal(body, result); err != nil {
		return errors.New("invalid beacon JSON response")
	}
	return nil
}

func (c *SlotAssignmentClient) GenesisTime(ctx context.Context) (int64, error) {
	var response struct {
		Data struct {
			GenesisTime string `json:"genesis_time"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/qrl/v1/beacon/genesis", &response); err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(response.Data.GenesisTime, 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid beacon genesis time")
	}
	return n, nil
}

// FinalizedEpoch is the latest finalized checkpoint epoch for duty indexing.
func (c *SlotAssignmentClient) FinalizedEpoch(ctx context.Context) (int64, error) {
	var response models.BeaconChainHeadResponse
	if err := c.get(ctx, "/qrl/v1alpha1/beacon/chainhead", &response); err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(response.FinalizedEpoch, 10, 64)
	if err != nil || n < 0 || n >= math.MaxInt64/models.SlotsPerEpoch {
		return 0, errors.New("invalid finalized epoch")
	}
	if response.OptimisticStatus {
		return 0, errors.New("beacon head is optimistic")
	}
	return n, nil
}

func normalizeDutyKey(key string) (string, error) {
	key = strings.TrimPrefix(key, "0x")
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) == 0 {
		return "", errors.New("invalid proposer pubkey")
	}
	return "0x" + hex.EncodeToString(decoded), nil
}

// ValidateSlotAssignments requires a complete, unique duty set for the epoch.
// Genesis slot zero is represented explicitly with an unknown proposer.
func ValidateSlotAssignments(epoch int64, rows []models.SlotAssignment) ([]models.SlotAssignment, error) {
	if epoch < 0 || epoch >= math.MaxInt64/models.SlotsPerEpoch {
		return nil, errors.New("invalid duty epoch")
	}
	start := epoch * models.SlotsPerEpoch
	seen := make(map[int64]bool)
	result := make([]models.SlotAssignment, 0, models.SlotsPerEpoch)
	for _, row := range rows {
		if row.Epoch != epoch || row.Slot < start || row.Slot >= start+models.SlotsPerEpoch || seen[row.Slot] {
			return nil, errors.New("duplicate or out-of-range proposer slot")
		}
		seen[row.Slot] = true
		if row.Slot == 0 && row.ValidatorIndex == "" && row.Pubkey == "" {
			result = append(result, row)
			continue
		}
		index, err := strconv.ParseUint(row.ValidatorIndex, 10, 64)
		if err != nil {
			return nil, errors.New("invalid proposer validator index")
		}
		row.ValidatorIndex = strconv.FormatUint(index, 10)
		row.Pubkey, err = normalizeDutyKey(row.Pubkey)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if epoch == 0 && !seen[0] {
		result = append(result, models.SlotAssignment{Epoch: 0, Slot: 0})
	}
	if len(result) != int(models.SlotsPerEpoch) {
		return nil, errors.New("incomplete proposer duties")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Slot < result[j].Slot })
	return result, nil
}

func (c *SlotAssignmentClient) Assignments(ctx context.Context, epoch int64) ([]models.SlotAssignment, error) {
	if epoch < 0 || epoch >= math.MaxInt64/models.SlotsPerEpoch {
		return nil, errors.New("invalid duty epoch")
	}
	rows, primaryErr := c.proposerDuties(ctx, epoch)
	if primaryErr == nil {
		return rows, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	rows, fallbackErr := c.legacyAssignments(ctx, epoch)
	if fallbackErr == nil {
		return rows, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var primary *beaconStatusError
	if errors.As(primaryErr, &primary) && errors.Is(fallbackErr, ErrEpochUnavailable) &&
		(primary.unavailable || primary.status == http.StatusNotFound || primary.status == http.StatusNotImplemented) {
		return nil, ErrEpochUnavailable
	}
	// Keep the unavailable sentinel exclusive to agreement between duty sources.
	return nil, fmt.Errorf("proposer duties: %v; assignments fallback: %v", primaryErr, fallbackErr)
}

func (c *SlotAssignmentClient) proposerDuties(ctx context.Context, epoch int64) ([]models.SlotAssignment, error) {
	var response struct {
		Optimistic bool `json:"execution_optimistic"`
		Data       []struct {
			Pubkey         string `json:"pubkey"`
			ValidatorIndex string `json:"validator_index"`
			Slot           string `json:"slot"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/qrl/v1/validator/duties/proposer/"+strconv.FormatInt(epoch, 10), &response); err != nil {
		return nil, err
	}
	if response.Optimistic {
		return nil, errors.New("proposer duties are optimistic")
	}
	rows := make([]models.SlotAssignment, 0, len(response.Data))
	for _, duty := range response.Data {
		slot, err := strconv.ParseInt(duty.Slot, 10, 64)
		if err != nil {
			return nil, errors.New("invalid proposer slot")
		}
		rows = append(rows, models.SlotAssignment{Epoch: epoch, Slot: slot, ValidatorIndex: duty.ValidatorIndex, Pubkey: duty.Pubkey})
	}
	return ValidateSlotAssignments(epoch, rows)
}

func (c *SlotAssignmentClient) legacyAssignments(ctx context.Context, epoch int64) ([]models.SlotAssignment, error) {
	var rows []models.SlotAssignment
	pageToken := ""
	seenTokens := make(map[string]bool)
	for page := 0; page < 100; page++ {
		var response struct {
			Epoch         string `json:"epoch"`
			NextPageToken string `json:"nextPageToken"`
			Assignments   []struct {
				ValidatorIndex string   `json:"validatorIndex"`
				ProposerSlots  []string `json:"proposerSlots"`
			} `json:"assignments"`
		}
		query := url.Values{"epoch": {strconv.FormatInt(epoch, 10)}, "page_size": {"100"}}
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		if err := c.get(ctx, "/qrl/v1alpha1/validators/assignments?"+query.Encode(), &response); err != nil {
			var status *beaconStatusError
			if page == 0 && errors.As(err, &status) && status.unavailable {
				return nil, ErrEpochUnavailable
			}
			return nil, err
		}
		if response.Epoch != strconv.FormatInt(epoch, 10) {
			return nil, errors.New("assignment response epoch mismatch")
		}
		for _, assignment := range response.Assignments {
			for _, rawSlot := range assignment.ProposerSlots {
				slot, err := strconv.ParseInt(rawSlot, 10, 64)
				if err != nil {
					return nil, errors.New("invalid assignment slot")
				}
				rows = append(rows, models.SlotAssignment{Epoch: epoch, Slot: slot, ValidatorIndex: assignment.ValidatorIndex})
				if len(rows) > int(models.SlotsPerEpoch) {
					return nil, errors.New("too many proposer assignments")
				}
			}
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
		if seenTokens[pageToken] || page == 99 {
			return nil, errors.New("assignment pagination did not complete")
		}
		seenTokens[pageToken] = true
	}
	for i := range rows {
		index, err := strconv.ParseUint(rows[i].ValidatorIndex, 10, 64)
		if err != nil {
			return nil, errors.New("invalid assignment validator index")
		}
		id := strconv.FormatUint(index, 10)
		key, ok := c.pubkeys[id]
		if !ok {
			var response struct {
				Data struct {
					Index     string `json:"index"`
					Validator struct {
						Pubkey string `json:"pubkey"`
					} `json:"validator"`
				} `json:"data"`
			}
			// Validator indices and pubkeys are immutable, including after exit.
			if err := c.get(ctx, "/qrl/v1/beacon/states/head/validators/"+id, &response); err != nil {
				return nil, err
			}
			if response.Data.Index != id {
				return nil, errors.New("validator pubkey lookup index mismatch")
			}
			key, err = normalizeDutyKey(response.Data.Validator.Pubkey)
			if err != nil {
				return nil, err
			}
			if len(c.pubkeys) >= 10000 {
				clear(c.pubkeys)
			}
			c.pubkeys[id] = key
		}
		rows[i].Pubkey = key
	}
	return ValidateSlotAssignments(epoch, rows)
}
