package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"QRL2MongoDB/models"
)

var ErrInvalidEjectionBalance = errors.New("invalid beacon EJECTION_BALANCE")

// EjectionBalance is loaded by the history worker independently of chain timing.
func (c *SlotAssignmentClient) EjectionBalance(ctx context.Context) (uint64, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, "/qrl/v1/config/spec", &response); err != nil {
		return 0, fmt.Errorf("read beacon ejection balance: %w", err)
	}
	var spec struct {
		EjectionBalance string `json:"EJECTION_BALANCE"`
	}
	if err := json.Unmarshal(response.Data, &spec); err != nil {
		return 0, ErrInvalidEjectionBalance
	}
	n, err := strconv.ParseUint(spec.EjectionBalance, 10, 64)
	if err != nil || n == 0 {
		return 0, ErrInvalidEjectionBalance
	}
	return n, nil
}

// ChainTiming reads both startup prerequisites with the client's timeout caps.
func (c *SlotAssignmentClient) ChainTiming(ctx context.Context) (models.ChainTiming, error) {
	var response struct {
		Data map[string]string `json:"data"`
	}
	var timing models.ChainTiming
	if err := c.get(ctx, "/qrl/v1/config/spec", &response); err != nil {
		return timing, fmt.Errorf("read beacon spec: %w", err)
	}
	var err error
	timing.SlotsPerEpoch, err = strconv.ParseInt(response.Data["SLOTS_PER_EPOCH"], 10, 64)
	if err != nil {
		return timing, errors.New("invalid beacon SLOTS_PER_EPOCH")
	}
	timing.SecondsPerSlot, err = strconv.ParseInt(response.Data["SECONDS_PER_SLOT"], 10, 64)
	if err != nil {
		return timing, errors.New("invalid beacon SECONDS_PER_SLOT")
	}
	timing.GenesisTime, err = c.GenesisTime(ctx)
	if err != nil {
		return timing, fmt.Errorf("read beacon genesis: %w", err)
	}
	return timing, timing.Validate()
}

type finalityResponse struct {
	Optimistic bool `json:"execution_optimistic"`
	Data       struct {
		Previous struct {
			Epoch string `json:"epoch"`
		} `json:"previous_justified"`
		Current struct {
			Epoch string `json:"epoch"`
		} `json:"current_justified"`
		Finalized struct {
			Epoch string `json:"epoch"`
		} `json:"finalized"`
	} `json:"data"`
}

func (c *SlotAssignmentClient) finality(ctx context.Context) (finalityResponse, error) {
	var response finalityResponse
	if err := c.get(ctx, "/qrl/v1/beacon/states/head/finality_checkpoints", &response); err != nil {
		return response, err
	}
	if response.Optimistic {
		return response, errors.New("beacon finality is optimistic")
	}
	for _, raw := range []string{response.Data.Previous.Epoch, response.Data.Current.Epoch, response.Data.Finalized.Epoch} {
		epoch, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return response, errors.New("invalid beacon checkpoint epoch")
		}
		if _, _, err := c.timing.EpochSlots(epoch); err != nil {
			return response, err
		}
	}
	previous, _ := strconv.ParseInt(response.Data.Previous.Epoch, 10, 64)
	current, _ := strconv.ParseInt(response.Data.Current.Epoch, 10, 64)
	finalized, _ := strconv.ParseInt(response.Data.Finalized.Epoch, 10, 64)
	if previous > current || finalized > current {
		return response, errors.New("inconsistent beacon finality checkpoints")
	}
	return response, nil
}

func (c *SlotAssignmentClient) FinalizedEpoch(ctx context.Context) (int64, error) {
	response, err := c.finality(ctx)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(response.Data.Finalized.Epoch, 10, 64)
}

// ChainHead derives the epoch solely from the beacon header slot.
func (c *SlotAssignmentClient) ChainHead(ctx context.Context) (*models.BeaconChainHeadResponse, error) {
	var response struct {
		Optimistic bool `json:"execution_optimistic"`
		Data       struct {
			Header struct {
				Message struct {
					Slot string `json:"slot"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/qrl/v1/beacon/headers/head", &response); err != nil {
		return nil, err
	}
	if response.Optimistic {
		return nil, errors.New("beacon head is optimistic")
	}
	slot, err := strconv.ParseInt(response.Data.Header.Message.Slot, 10, 64)
	if err != nil {
		return nil, errors.New("invalid beacon head slot")
	}
	epoch, err := c.timing.EpochAtSlot(slot)
	if err != nil {
		return nil, err
	}
	checkpoints, err := c.finality(ctx)
	if err != nil {
		return nil, err
	}
	finalized, _ := strconv.ParseInt(checkpoints.Data.Finalized.Epoch, 10, 64)
	justified, _ := strconv.ParseInt(checkpoints.Data.Current.Epoch, 10, 64)
	if finalized > justified || justified > epoch {
		return nil, errors.New("inconsistent beacon head and finality; retry snapshot")
	}
	return &models.BeaconChainHeadResponse{
		HeadSlot: strconv.FormatInt(slot, 10), HeadEpoch: strconv.FormatInt(epoch, 10),
		FinalizedEpoch: checkpoints.Data.Finalized.Epoch, JustifiedEpoch: checkpoints.Data.Current.Epoch,
		PreviousJustifiedEpoch: checkpoints.Data.Previous.Epoch,
		FinalizedSlot:          strconv.FormatInt(finalized*c.timing.SlotsPerEpoch, 10),
		JustifiedSlot:          strconv.FormatInt(justified*c.timing.SlotsPerEpoch, 10),
	}, nil
}

// ValidatorsAtSlot fetches one complete state, including empty genesis sets.
func (c *SlotAssignmentClient) ValidatorsAtSlot(ctx context.Context, slot int64) ([]models.StateValidator, error) {
	if slot < 0 {
		return nil, errors.New("invalid validator state slot")
	}
	var response struct {
		Optimistic bool                    `json:"execution_optimistic"`
		Data       []models.StateValidator `json:"data"`
	}
	if err := c.get(ctx, "/qrl/v1/beacon/states/"+strconv.FormatInt(slot, 10)+"/validators", &response); err != nil {
		return nil, err
	}
	if response.Optimistic || response.Data == nil {
		return nil, errors.New("invalid or optimistic beacon validator state")
	}
	seen := make(map[string]bool, len(response.Data))
	for _, v := range response.Data {
		if _, err := v.SummaryStatus(); err != nil {
			return nil, err
		}
		index, _ := strconv.ParseUint(v.Index, 10, 64)
		key := strconv.FormatUint(index, 10)
		if seen[key] {
			return nil, errors.New("duplicate beacon validator index")
		}
		seen[key] = true
	}
	return response.Data, nil
}
