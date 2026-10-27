package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"QRL2MongoDB/models"
)

type slashingHeader struct {
	Slot          string `json:"slot"`
	ProposerIndex string `json:"proposer_index"`
	ParentRoot    string `json:"parent_root"`
	StateRoot     string `json:"state_root"`
	BodyRoot      string `json:"body_root"`
}

type slashingCheckpoint struct {
	Epoch string `json:"epoch"`
	Root  string `json:"root"`
}

type slashingAttestationData struct {
	Slot            string             `json:"slot"`
	Index           string             `json:"index"`
	BeaconBlockRoot string             `json:"beacon_block_root"`
	Source          slashingCheckpoint `json:"source"`
	Target          slashingCheckpoint `json:"target"`
}

type slashingAttestation struct {
	Indices []string                `json:"attesting_indices"`
	Data    slashingAttestationData `json:"data"`
}

type slashingBlockResponse struct {
	Version    string `json:"version"`
	Optimistic bool   `json:"execution_optimistic"`
	Data       struct {
		Message struct {
			Slot          string `json:"slot"`
			ProposerIndex string `json:"proposer_index"`
			Body          struct {
				ProposerSlashings []struct {
					Header1 struct {
						Message slashingHeader `json:"message"`
					} `json:"signed_header_1"`
					Header2 struct {
						Message slashingHeader `json:"message"`
					} `json:"signed_header_2"`
				} `json:"proposer_slashings"`
				AttesterSlashings []struct {
					Attestation1 slashingAttestation `json:"attestation_1"`
					Attestation2 slashingAttestation `json:"attestation_2"`
				} `json:"attester_slashings"`
			} `json:"body"`
		} `json:"message"`
	} `json:"data"`
}

func missingBeaconBlock(body string) bool {
	body = strings.ToLower(body)
	for _, phrase := range []string{"could not find requested block", "block not found", "could not find block", "no block found"} {
		if strings.Contains(body, phrase) {
			return true
		}
	}
	return false
}

// SlashingEvidence reads the disjoint interval between two first-slot states:
// (start(epoch-1), start(epoch)]. This includes a slashing in the boundary block
// and keeps every requested slot at or below the finalized checkpoint.
func (c *SlotAssignmentClient) SlashingEvidence(ctx context.Context, observedEpoch int64) ([]models.SlashingEvent, error) {
	end, _, err := c.timing.EpochSlots(observedEpoch)
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if observedEpoch > 0 {
		start = end - c.timing.SlotsPerEpoch + 1
	}
	events := make([]models.SlashingEvent, 0)
	var evidenceErr error
	for slot := start; slot <= end; slot++ {
		rows, err := c.slashingsAtSlot(ctx, slot)
		events = append(events, rows...)
		if err != nil {
			err = fmt.Errorf("slashing evidence slot %d: %w", slot, err)
			if !errors.Is(err, models.ErrStructuralSlashingEvidence) {
				return nil, err
			}
			evidenceErr = errors.Join(evidenceErr, err)
		}
	}
	return events, evidenceErr
}

func (c *SlotAssignmentClient) slashingsAtSlot(ctx context.Context, slot int64) ([]models.SlashingEvent, error) {
	var response slashingBlockResponse
	if err := c.get(ctx, "/qrl/v1/beacon/blocks/"+strconv.FormatInt(slot, 10), &response); err != nil {
		var status *beaconStatusError
		if errors.As(err, &status) && status.missingBlock {
			return nil, nil
		}
		return nil, err
	}
	if response.Optimistic {
		return nil, errors.New("optimistic slashing block")
	}
	events, err := c.parseSlashingBlock(response, slot)
	if err != nil {
		return events, fmt.Errorf("%w: %v", models.ErrStructuralSlashingEvidence, err)
	}
	return events, nil
}

func (c *SlotAssignmentClient) parseSlashingBlock(response slashingBlockResponse, slot int64) ([]models.SlashingEvent, error) {
	block := response.Data.Message
	n, err := strconv.ParseInt(block.Slot, 10, 64)
	if err != nil || n != slot || response.Version != "zond" ||
		block.Body.ProposerSlashings == nil || block.Body.AttesterSlashings == nil {
		return nil, errors.New("invalid slashing block")
	}
	proposer, err := canonicalValidatorIndex(block.ProposerIndex)
	if err != nil {
		return nil, err
	}
	epoch, err := c.timing.EpochAtSlot(slot)
	if err != nil {
		return nil, err
	}
	events := make([]models.SlashingEvent, 0)
	add := func(kind string, ordinal int, indices []string, a, b [32]byte) error {
		if a == b || len(indices) == 0 {
			return errors.New("invalid slashing conflict")
		}
		events = append(events, models.SlashingEvent{
			ID: fmt.Sprintf("%d:%s:%d", slot, kind, ordinal), Slot: slot, Epoch: epoch,
			Type: kind, ValidatorIndices: indices, Whistleblower: proposer,
			ConflictingRoots: []string{"0x" + hex.EncodeToString(a[:]), "0x" + hex.EncodeToString(b[:])},
		})
		return nil
	}
	var evidenceErr error
	for i, proof := range block.Body.ProposerSlashings {
		err := func() error {
			a, b := proof.Header1.Message, proof.Header2.Message
			index, err := canonicalValidatorIndex(a.ProposerIndex)
			if err != nil || a.ProposerIndex != b.ProposerIndex || a.Slot != b.Slot {
				return errors.New("inconsistent proposer slashing headers")
			}
			root1, err := headerRoot(a)
			if err != nil {
				return err
			}
			root2, err := headerRoot(b)
			if err != nil {
				return err
			}
			return add("proposer", i, []string{index}, root1, root2)
		}()
		evidenceErr = errors.Join(evidenceErr, err)
	}
	for i, proof := range block.Body.AttesterSlashings {
		err := func() error {
			indices, err := intersectAttesters(proof.Attestation1.Indices, proof.Attestation2.Indices)
			if err != nil {
				return err
			}
			root1, err := attestationRoot(proof.Attestation1.Data)
			if err != nil {
				return err
			}
			root2, err := attestationRoot(proof.Attestation2.Data)
			if err != nil {
				return err
			}
			return add("attester", i, indices, root1, root2)
		}()
		evidenceErr = errors.Join(evidenceErr, err)
	}
	return events, evidenceErr
}

func canonicalValidatorIndex(value string) (string, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return "", errors.New("invalid slashing validator index")
	}
	return strconv.FormatUint(n, 10), nil
}

func intersectAttesters(a, b []string) ([]string, error) {
	sets := make([]map[string]bool, 2)
	for i, values := range [][]string{a, b} {
		sets[i] = make(map[string]bool, len(values))
		for _, value := range values {
			id, err := canonicalValidatorIndex(value)
			if err != nil || sets[i][id] {
				return nil, errors.New("invalid attesting indices")
			}
			sets[i][id] = true
		}
	}
	indices := make([]string, 0)
	for id := range sets[0] {
		if sets[1][id] {
			indices = append(indices, id)
		}
	}
	sort.Slice(indices, func(i, j int) bool {
		a, _ := strconv.ParseUint(indices[i], 10, 64)
		b, _ := strconv.ParseUint(indices[j], 10, 64)
		return a < b
	})
	return indices, nil
}

// SSZ roots of the unsigned Header and AttestationData containers. Each field
// occupies a 32-byte chunk; uint64 values use little endian encoding.
func uint64Root(value string) ([32]byte, error) {
	var root [32]byte
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return root, errors.New("invalid slashing numeric field")
	}
	binary.LittleEndian.PutUint64(root[:8], n)
	return root, nil
}

func hexRoot(value string) ([32]byte, error) {
	var root [32]byte
	bytes, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil || len(bytes) != 32 {
		return root, errors.New("invalid slashing root")
	}
	copy(root[:], bytes)
	return root, nil
}

func containerRoot(fields ...[32]byte) [32]byte {
	width := 1
	for width < len(fields) {
		width *= 2
	}
	leaves := make([][32]byte, width)
	copy(leaves, fields)
	for width > 1 {
		for i := 0; i < width; i += 2 {
			var pair [64]byte
			copy(pair[:32], leaves[i][:])
			copy(pair[32:], leaves[i+1][:])
			leaves[i/2] = sha256.Sum256(pair[:])
		}
		width /= 2
	}
	return leaves[0]
}

func headerRoot(header slashingHeader) ([32]byte, error) {
	var fields [5][32]byte
	for i, value := range []string{header.Slot, header.ProposerIndex, header.ParentRoot, header.StateRoot, header.BodyRoot} {
		var err error
		if i < 2 {
			fields[i], err = uint64Root(value)
		} else {
			fields[i], err = hexRoot(value)
		}
		if err != nil {
			return [32]byte{}, err
		}
	}
	return containerRoot(fields[:]...), nil
}

func attestationRoot(data slashingAttestationData) ([32]byte, error) {
	slot, err := uint64Root(data.Slot)
	if err != nil {
		return [32]byte{}, err
	}
	index, err := uint64Root(data.Index)
	if err != nil {
		return [32]byte{}, err
	}
	block, err := hexRoot(data.BeaconBlockRoot)
	if err != nil {
		return [32]byte{}, err
	}
	checkpoints := make([][32]byte, 0, 2)
	for _, checkpoint := range []slashingCheckpoint{data.Source, data.Target} {
		epoch, err := uint64Root(checkpoint.Epoch)
		if err != nil {
			return [32]byte{}, err
		}
		root, err := hexRoot(checkpoint.Root)
		if err != nil {
			return [32]byte{}, err
		}
		checkpoints = append(checkpoints, containerRoot(epoch, root))
	}
	return containerRoot(slot, index, block, checkpoints[0], checkpoints[1]), nil
}
