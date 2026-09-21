package db

import (
	"QRL2MongoDB/models"
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
)

func batchWriteTestBlock(number, hash string) models.ZondDatabaseBlock {
	return models.ZondDatabaseBlock{Result: models.Result{Number: number, Hash: hash}}
}

func observedBlock(hash, ingestionState string) storedBlockState {
	return storedBlockState{hash: hash, ingestionState: ingestionState}
}

func TestSortedBlockWriteCandidatesOrdersAndDeduplicates(t *testing.T) {
	candidates, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x3", "0xhash3"),
		batchWriteTestBlock("0x1", "0xhash1"),
		batchWriteTestBlock("0x2", "0xhash2"),
		batchWriteTestBlock("0x1", "0xHASH1"),
	})
	if err != nil {
		t.Fatalf("sortedBlockWriteCandidates() error = %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidate count = %d, want 3", len(candidates))
	}
	for index, want := range []int64{1, 2, 3} {
		if candidates[index].numberInt != want {
			t.Fatalf("candidate %d height = %d, want %d", index, candidates[index].numberInt, want)
		}
	}
}

func TestSortedBlockWriteCandidatesRejectsNoncontiguousBatch(t *testing.T) {
	_, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x1", "0xhash1"),
		batchWriteTestBlock("0x3", "0xhash3"),
	})
	if err == nil {
		t.Fatal("sortedBlockWriteCandidates() error = nil, want noncontiguous batch error")
	}
}

func TestClassifyConfirmedBlockPrefixRejectsDuplicateCanonicalHash(t *testing.T) {
	candidates, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x1", "0xabcdef"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := classifyConfirmedBlockPrefix(
		candidates,
		map[string]bool{"0x1": true},
		map[string][]storedBlockState{"0x1": {
			observedBlock("0xABCDEF", BlockIngestionComplete),
			observedBlock("0xabcdef", BlockIngestionComplete),
		}},
	)
	if !errors.Is(err, ErrBlockHeightConflict) {
		t.Fatalf("classifyConfirmedBlockPrefix() error = %v, want ErrBlockHeightConflict", err)
	}
	if len(result.Confirmed) != 0 {
		t.Fatalf("confirmed result = %#v, want no confirmed duplicate block", result)
	}
}

func TestClassifyConfirmedBlockPrefixRejectsMarkerlessLegacyRow(t *testing.T) {
	candidates, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x1", "0xhash1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := classifyConfirmedBlockPrefix(
		candidates,
		map[string]bool{"0x1": true},
		map[string][]storedBlockState{"0x1": {observedBlock("0xhash1", "")}},
	)
	if !errors.Is(err, ErrBlockMigrationRequired) {
		t.Fatalf("error = %v, want ErrBlockMigrationRequired", err)
	}
	if len(result.Confirmed) != 0 {
		t.Fatalf("confirmed count = %d, want 0", len(result.Confirmed))
	}
}

func TestBlockMigrationRequiredErrorIsActionable(t *testing.T) {
	err := blockMigrationRequiredError(7)
	if !errors.Is(err, ErrBlockMigrationRequired) {
		t.Fatalf("error = %v, want ErrBlockMigrationRequired", err)
	}
	for _, phrase := range []string{"7 block rows", "companion reindex", "before restart"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Fatalf("error %q does not contain %q", err, phrase)
		}
	}
}

func TestMissingBlockMigrationAttestationErrorIsActionable(t *testing.T) {
	err := missingBlockMigrationAttestationError()
	if !errors.Is(err, ErrBlockMigrationRequired) {
		t.Fatalf("error = %v, want ErrBlockMigrationRequired", err)
	}
	if !strings.Contains(err.Error(), "reindex-block-companions --execute") {
		t.Fatalf("error %q does not identify the migration command", err)
	}
}

func TestHasUniqueBlockHeightIndexRequiresNamedUniqueIndex(t *testing.T) {
	unique := true
	nonUnique := false
	if hasUniqueBlockHeightIndex([]*mongo.IndexSpecification{{
		Name:   BlockHeightUniqueIndex,
		Unique: &nonUnique,
	}}) {
		t.Fatal("non-unique block-height index was accepted")
	}
	if !hasUniqueBlockHeightIndex([]*mongo.IndexSpecification{{
		Name:   BlockHeightUniqueIndex,
		Unique: &unique,
	}}) {
		t.Fatal("named unique block-height index was rejected")
	}
}

func TestPendingOnlyBlockStateRequiresEveryRowPending(t *testing.T) {
	pending, err := pendingOnlyBlockState("0x2", []storedBlockState{
		observedBlock("0xhashA", BlockIngestionPending),
		observedBlock("0xhashB", BlockIngestionPending),
	})
	if err != nil || !pending {
		t.Fatalf("pendingOnlyBlockState() = %v, %v, want true, nil", pending, err)
	}

	pending, err = pendingOnlyBlockState("0x2", []storedBlockState{
		observedBlock("0xhashA", BlockIngestionPending),
		observedBlock("0xhashA", BlockIngestionComplete),
	})
	if err != nil || pending {
		t.Fatalf("pendingOnlyBlockState() = %v, %v, want false, nil", pending, err)
	}

	_, err = pendingOnlyBlockState("0x2", []storedBlockState{
		observedBlock("0xhashA", ""),
	})
	if !errors.Is(err, ErrBlockMigrationRequired) {
		t.Fatalf("error = %v, want ErrBlockMigrationRequired", err)
	}
}

func TestBlockNeedsCompanionReindexSupportsResumeStates(t *testing.T) {
	tests := []struct {
		name      string
		found     []storedBlockState
		want      bool
		wantError error
	}{
		{
			name:  "markerless",
			found: []storedBlockState{observedBlock("0xhash", "")},
			want:  true,
		},
		{
			name:  "pending checkpoint",
			found: []storedBlockState{observedBlock("0xhash", BlockIngestionPending)},
			want:  true,
		},
		{
			name:  "complete",
			found: []storedBlockState{observedBlock("0xhash", BlockIngestionComplete)},
			want:  false,
		},
		{
			name:      "unsupported",
			found:     []storedBlockState{observedBlock("0xhash", "other")},
			wantError: ErrBlockMigrationRequired,
		},
		{
			name:      "hash conflict",
			found:     []storedBlockState{observedBlock("0xother", BlockIngestionPending)},
			wantError: ErrBlockHeightConflict,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := blockNeedsCompanionReindex("0x1", "0xhash", test.found)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
			if got != test.want {
				t.Fatalf("needs reindex = %v, want %v", got, test.want)
			}
		})
	}
}

func auditRow(height int64, hash, parent, state string) canonicalBlockAuditRow {
	row := canonicalBlockAuditRow{
		BlockNumberInt: height,
		IngestionState: state,
	}
	row.Result.Number = canonicalStoredBlockNumber(height)
	row.Result.Hash = hash
	row.Result.ParentHash = parent
	return row
}

func TestCanonicalChainAuditFindsMissingHeightAndValidatesLinks(t *testing.T) {
	audit := &canonicalChainAudit{previousHeight: -1}
	for _, row := range []canonicalBlockAuditRow{
		auditRow(0, "0xhash0", "0xzero", BlockIngestionComplete),
		auditRow(2, "0xhash2", "0xhash1", BlockIngestionComplete),
	} {
		if err := audit.add(row); err != nil {
			t.Fatal(err)
		}
	}
	missing, err := audit.finish()
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "0x1" {
		t.Fatalf("missing = %v, want [0x1]", missing)
	}

	audit = &canonicalChainAudit{previousHeight: -1}
	if err := audit.add(auditRow(0, "0xhash0", "0xzero", BlockIngestionComplete)); err != nil {
		t.Fatal(err)
	}
	missing, err = audit.finishThrough(2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(missing, ",") != "0x1,0x2" {
		t.Fatalf("missing through cursor = %v, want [0x1 0x2]", missing)
	}

	audit = &canonicalChainAudit{previousHeight: -1}
	if err := audit.add(auditRow(0, "0xhash0", "0xzero", BlockIngestionComplete)); err != nil {
		t.Fatal(err)
	}
	if err := audit.add(auditRow(1, "0xhash1", "0xwrong", BlockIngestionComplete)); err != nil {
		t.Fatal(err)
	}
	if _, err := audit.finish(); err == nil || !strings.Contains(err.Error(), "parent mismatch") {
		t.Fatalf("error = %v, want parent mismatch", err)
	}
}

func TestCanonicalChainAuditRejectsPendingAndConflictingRows(t *testing.T) {
	audit := &canonicalChainAudit{previousHeight: -1}
	if err := audit.add(auditRow(0, "0xhash0", "0xzero", BlockIngestionPending)); err == nil {
		t.Fatal("pending audit row was accepted")
	}

	audit = &canonicalChainAudit{previousHeight: -1}
	if err := audit.add(auditRow(0, "0xhash0", "0xzero", BlockIngestionComplete)); err != nil {
		t.Fatal(err)
	}
	if err := audit.add(auditRow(0, "0xother", "0xzero", BlockIngestionComplete)); !errors.Is(err, ErrBlockHeightConflict) {
		t.Fatalf("error = %v, want ErrBlockHeightConflict", err)
	}

	audit = &canonicalChainAudit{previousHeight: -1}
	if err := audit.add(auditRow(0, "0xhash0", "0xzero", BlockIngestionComplete)); err != nil {
		t.Fatal(err)
	}
	if err := audit.add(auditRow(0, "0xhash0", "0xzero", BlockIngestionComplete)); !errors.Is(err, ErrBlockHeightConflict) {
		t.Fatalf("duplicate identity error = %v, want ErrBlockHeightConflict", err)
	}
}

func TestClassifyConfirmedBlockPrefixRejectsDifferentHash(t *testing.T) {
	candidates, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x1", "0xexpected"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := classifyConfirmedBlockPrefix(
		candidates,
		map[string]bool{},
		map[string][]storedBlockState{"0x1": {observedBlock("0xdifferent", BlockIngestionPending)}},
	)
	if !errors.Is(err, ErrBlockHeightConflict) {
		t.Fatalf("error = %v, want ErrBlockHeightConflict", err)
	}
	if len(result.Confirmed) != 0 {
		t.Fatalf("confirmed count = %d, want 0", len(result.Confirmed))
	}
}

func TestClassifyConfirmedBlockPrefixStopsAtPartialOrUnknownWrite(t *testing.T) {
	candidates, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x1", "0xhash1"),
		batchWriteTestBlock("0x2", "0xhash2"),
		batchWriteTestBlock("0x3", "0xhash3"),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		observed map[string][]storedBlockState
	}{
		{
			name: "partial ordered write",
			observed: map[string][]storedBlockState{
				"0x1": {observedBlock("0xhash1", BlockIngestionPending)},
			},
		},
		{
			name: "unknown outcome with later row visible",
			observed: map[string][]storedBlockState{
				"0x1": {observedBlock("0xhash1", BlockIngestionPending)},
				"0x3": {observedBlock("0xhash3", BlockIngestionPending)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := classifyConfirmedBlockPrefix(candidates, map[string]bool{}, test.observed)
			if !errors.Is(err, ErrBlockWriteUnresolved) {
				t.Fatalf("error = %v, want ErrBlockWriteUnresolved", err)
			}
			if len(result.Confirmed) != 1 {
				t.Fatalf("confirmed count = %d, want 1", len(result.Confirmed))
			}
			if result.Confirmed[0].Block.Result.Number != "0x1" {
				t.Fatalf("confirmed block = %s, want 0x1", result.Confirmed[0].Block.Result.Number)
			}
		})
	}
}

func TestClassifyConfirmedBlockPrefixKeepsPendingCompanionsRecoverable(t *testing.T) {
	candidates, err := sortedBlockWriteCandidates([]models.ZondDatabaseBlock{
		batchWriteTestBlock("0x1", "0xhash1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := classifyConfirmedBlockPrefix(
		candidates,
		map[string]bool{"0x1": true},
		map[string][]storedBlockState{"0x1": {
			observedBlock("0xhash1", BlockIngestionPending),
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Confirmed) != 1 || result.Confirmed[0].CompanionsComplete {
		t.Fatalf("confirmed result = %#v, want one recoverable pending block", result)
	}
}
