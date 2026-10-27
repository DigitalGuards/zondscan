package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"QRL2MongoDB/models"
)

const maxBeaconBlockResponse = 64 << 20

type slashingHeader struct {
	Slot          string `json:"slot"`
	ProposerIndex string `json:"proposer_index"`
	ParentRoot    string `json:"parent_root"`
	StateRoot     string `json:"state_root"`
	BodyRoot      string `json:"body_root"`
}

type signedSlashingHeader struct {
	Message slashingHeader `json:"message"`
}

type proposerSlashing struct {
	Header1 *signedSlashingHeader `json:"signed_header_1"`
	Header2 *signedSlashingHeader `json:"signed_header_2"`
}

type legacySignedSlashingHeader struct {
	Header struct {
		Slot          string `json:"slot"`
		ProposerIndex string `json:"proposerIndex"`
		ParentRoot    string `json:"parentRoot"`
		StateRoot     string `json:"stateRoot"`
		BodyRoot      string `json:"bodyRoot"`
	} `json:"header"`
}

type legacySlashingBlock struct {
	Slot          string `json:"slot"`
	ProposerIndex string `json:"proposerIndex"`
	Body          *struct {
		ProposerSlashings json.RawMessage `json:"proposerSlashings"`
	} `json:"body"`
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
				ProposerSlashings []proposerSlashing `json:"proposer_slashings"`
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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := c.slashingsAtSlot(ctx, slot)
		events = append(events, rows...)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err != nil {
			err = fmt.Errorf("slashing evidence slot %d: %w", slot, err)
			evidenceErr = errors.Join(evidenceErr, err)
		}
	}
	return events, evidenceErr
}

func (c *SlotAssignmentClient) slashingsAtSlot(ctx context.Context, slot int64) ([]models.SlashingEvent, error) {
	var response slashingBlockResponse
	if err := c.getWithLimit(ctx, "/qrl/v1/beacon/blocks/"+strconv.FormatInt(slot, 10), &response, maxBeaconBlockResponse); err != nil {
		var status *beaconStatusError
		if errors.As(err, &status) && status.missingBlock {
			return nil, nil
		}
		return nil, err
	}
	if response.Optimistic {
		return nil, errors.New("optimistic slashing block")
	}
	var fallbackErr error
	for _, proof := range response.Data.Message.Body.ProposerSlashings {
		if proof.Header1 == nil || proof.Header2 == nil {
			var proofs []proposerSlashing
			proofs, fallbackErr = c.legacyProposerSlashings(ctx, slot, response.Data.Message.ProposerIndex)
			if fallbackErr == nil && len(proofs) != len(response.Data.Message.Body.ProposerSlashings) {
				fallbackErr = errors.New("proposer slashing count mismatch")
			}
			if fallbackErr == nil {
				response.Data.Message.Body.ProposerSlashings = proofs
			} else {
				fallbackErr = fmt.Errorf("proposer slashing fallback: %w", fallbackErr)
			}
			break
		}
	}
	events, err := c.parseSlashingBlock(response, slot)
	return events, errors.Join(fallbackErr, err)
}

// Qrysm's v1 conversion can drop a proposer header. The legacy route retains
// both headers; its fork-specific wrapper is identified by block.body.
func (c *SlotAssignmentClient) legacyProposerSlashings(ctx context.Context, slot int64, proposer string) ([]proposerSlashing, error) {
	proposer, err := canonicalValidatorIndex(proposer)
	if err != nil {
		return nil, err
	}
	var response struct {
		BlockContainers []map[string]json.RawMessage `json:"blockContainers"`
		NextPageToken   string                       `json:"nextPageToken"`
	}
	if err := c.getWithLimit(ctx, "/qrl/v1alpha2/beacon/blocks?slot="+strconv.FormatInt(slot, 10), &response, maxBeaconBlockResponse); err != nil {
		return nil, err
	}
	if response.NextPageToken != "" {
		return nil, errors.New("incomplete fallback block page")
	}
	var selected *legacySlashingBlock
	for _, container := range response.BlockContainers {
		var canonical bool
		if err := json.Unmarshal(container["canonical"], &canonical); err != nil {
			return nil, errors.New("invalid fallback block canonical flag")
		}
		if !canonical {
			continue
		}
		wrappers := 0
		for _, value := range container {
			var wrapper struct {
				Block *legacySlashingBlock `json:"block"`
			}
			if err := json.Unmarshal(value, &wrapper); err != nil || wrapper.Block == nil || wrapper.Block.Body == nil {
				continue
			}
			wrappers++
			if wrappers > 1 {
				return nil, errors.New("ambiguous fallback block wrapper")
			}
			block := wrapper.Block
			n, slotErr := strconv.ParseInt(block.Slot, 10, 64)
			index, indexErr := canonicalValidatorIndex(block.ProposerIndex)
			if slotErr != nil || n != slot || indexErr != nil || index != proposer {
				continue
			}
			if selected != nil {
				return nil, errors.New("multiple canonical fallback blocks")
			}
			selected = block
		}
	}
	if selected == nil {
		return nil, errors.New("missing canonical fallback slashing block")
	}
	var legacyProofs []struct {
		Header1 *legacySignedSlashingHeader `json:"header1"`
		Header2 *legacySignedSlashingHeader `json:"header2"`
	}
	if err := json.Unmarshal(selected.Body.ProposerSlashings, &legacyProofs); err != nil || legacyProofs == nil {
		return nil, errors.New("invalid fallback proposer slashings")
	}
	proofs := make([]proposerSlashing, 0, len(legacyProofs))
	for _, proof := range legacyProofs {
		a, err := proof.Header1.toSlashingHeader()
		if err != nil {
			return nil, err
		}
		b, err := proof.Header2.toSlashingHeader()
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proposerSlashing{Header1: a, Header2: b})
	}
	return proofs, nil
}

func (h *legacySignedSlashingHeader) toSlashingHeader() (*signedSlashingHeader, error) {
	if h == nil {
		return nil, errors.New("missing fallback proposer slashing header")
	}
	header := slashingHeader{Slot: h.Header.Slot, ProposerIndex: h.Header.ProposerIndex}
	for _, field := range []struct {
		value string
		hex   *string
	}{
		{h.Header.ParentRoot, &header.ParentRoot},
		{h.Header.StateRoot, &header.StateRoot},
		{h.Header.BodyRoot, &header.BodyRoot},
	} {
		root, err := base64.StdEncoding.DecodeString(field.value)
		if err != nil || len(root) != 32 {
			return nil, errors.New("invalid fallback slashing root")
		}
		*field.hex = "0x" + hex.EncodeToString(root)
	}
	return &signedSlashingHeader{Message: header}, nil
}

func (c *SlotAssignmentClient) parseSlashingBlock(response slashingBlockResponse, slot int64) ([]models.SlashingEvent, error) {
	block := response.Data.Message
	n, err := strconv.ParseInt(block.Slot, 10, 64)
	if err != nil || n != slot ||
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
			if proof.Header1 == nil || proof.Header2 == nil {
				return errors.New("missing proposer slashing header")
			}
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
