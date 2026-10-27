package services

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"QRL2MongoDB/models"
)

// ValidatorChanges derives lifecycle milestones from the fetched state.
// Ejection uses the effective balance observed at the exit epoch. A zero
// threshold classifies unslashed exits as exited while the spec is incomplete.
func ValidatorChanges(epoch int64, validators []models.StateValidator, ejectionBalance uint64) ([]models.ValidatorSetChange, models.SlashingSnapshot, error) {
	changes := make([]models.ValidatorSetChange, 0)
	snapshot := models.SlashingSnapshot{Epoch: epoch, Validators: make([]models.SlashedValidator, 0)}
	if epoch < 0 {
		return nil, snapshot, fmt.Errorf("invalid validator change epoch")
	}
	seen := make(map[string]bool)
	for _, v := range validators {
		if _, err := v.SummaryStatus(); err != nil {
			return nil, snapshot, err
		}
		index, _ := strconv.ParseUint(v.Index, 10, 64)
		id := strconv.FormatUint(index, 10)
		key, err := hex.DecodeString(strings.TrimPrefix(v.Validator.Pubkey, "0x"))
		if err != nil || len(key) == 0 || seen[id] {
			return nil, snapshot, fmt.Errorf("invalid or duplicate validator identity")
		}
		seen[id] = true
		pubkey := "0x" + hex.EncodeToString(key)
		exitKind := "exited"
		if !v.Validator.Slashed && ejectionBalance > 0 {
			balance, _ := strconv.ParseUint(v.Validator.EffectiveBalance, 10, 64)
			if balance <= ejectionBalance {
				exitKind = "ejected"
			}
		}
		for _, milestone := range []struct{ kind, epoch string }{
			{"activated", v.Validator.ActivationEpoch},
			{exitKind, v.Validator.ExitEpoch},
		} {
			n, _ := strconv.ParseUint(milestone.epoch, 10, 64)
			if n == uint64(epoch) {
				changes = append(changes, validatorChange(epoch, milestone.kind, id, pubkey))
			}
		}
		if v.Validator.Slashed {
			activation, _ := strconv.ParseUint(v.Validator.ActivationEpoch, 10, 64)
			// A slashed validator has already activated at a representable epoch.
			if activation > uint64(epoch) {
				return nil, snapshot, fmt.Errorf("slashed validator has a future activation")
			}
			snapshot.Validators = append(snapshot.Validators, models.SlashedValidator{Index: id, ActivationEpoch: activation})
		}
	}
	return changes, snapshot, nil
}

func validatorChange(epoch int64, kind, index, pubkey string) models.ValidatorSetChange {
	return models.ValidatorSetChange{
		ID: fmt.Sprintf("%d:%s:%s", epoch, kind, index), Epoch: epoch,
		Kind: kind, ValidatorIndex: index, Pubkey: pubkey,
	}
}

// SlashingChanges assigns the first observed slashed flag to the newer epoch.
func SlashingChanges(older, newer models.SlashingSnapshot, validators []models.StateValidator) ([]models.ValidatorSetChange, error) {
	if older.Epoch+1 != newer.Epoch {
		return nil, fmt.Errorf("slashing comparison requires adjacent epochs")
	}
	pubkeys := make(map[string]string, len(validators))
	for _, v := range validators {
		index, err := strconv.ParseUint(v.Index, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid validator index")
		}
		pubkeys[strconv.FormatUint(index, 10)] = strings.ToLower(v.Validator.Pubkey)
	}
	previous := make(map[string]bool, len(older.Validators))
	for _, v := range older.Validators {
		previous[v.Index] = true
	}
	changes := make([]models.ValidatorSetChange, 0)
	for _, v := range newer.Validators {
		if previous[v.Index] {
			delete(previous, v.Index)
		} else {
			pubkey, exists := pubkeys[v.Index]
			if !exists {
				return nil, fmt.Errorf("slashed validator missing from epoch registry")
			}
			changes = append(changes, validatorChange(newer.Epoch, "slashed", v.Index, "0x"+strings.TrimPrefix(pubkey, "0x")))
		}
	}
	if len(previous) != 0 {
		return nil, fmt.Errorf("slashed flag regressed between epochs")
	}
	return changes, nil
}

// ResolveSlashingEvents restricts attestation intersections to validators whose
// flags changed in this interval. Consensus processes proposer operations first;
// an index is attributed to its first eligible operation only.
func ResolveSlashingEvents(events []models.SlashingEvent, changes []models.ValidatorSetChange, snapshot models.SlashingSnapshot) ([]models.SlashingEvent, error) {
	remaining := make(map[string]bool, len(changes))
	activation := make(map[string]uint64, len(snapshot.Validators))
	for _, c := range changes {
		remaining[c.ValidatorIndex] = true
	}
	for _, v := range snapshot.Validators {
		activation[v.Index] = v.ActivationEpoch
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Slot < events[j].Slot })
	result := make([]models.SlashingEvent, 0)
	for _, event := range events {
		indices := make([]string, 0)
		for _, id := range event.ValidatorIndices {
			if remaining[id] && activation[id] <= uint64(event.Epoch) {
				indices = append(indices, id)
				delete(remaining, id)
			}
		}
		if len(indices) > 0 {
			event.ValidatorIndices = indices
			result = append(result, event)
		}
	}
	if len(remaining) != 0 {
		return result, fmt.Errorf("slashing evidence incomplete for %d changed validators", len(remaining))
	}
	return result, nil
}
