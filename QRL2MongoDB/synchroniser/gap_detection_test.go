package synchroniser

import (
	"QRL2MongoDB/db"
	"reflect"
	"testing"
)

func gapObservation(height int64, number, hash, state string) observedGapBlock {
	row := observedGapBlock{
		BlockNumberInt: height,
		IngestionState: state,
	}
	row.Result.Number = number
	row.Result.Hash = hash
	return row
}

func TestRepairHeightsIncludesIncompleteAndConflictingRows(t *testing.T) {
	rows := []observedGapBlock{
		gapObservation(1, "0x1", "0xhash1", db.BlockIngestionComplete),
		gapObservation(2, "0x2", "0xhash2", ""),
		gapObservation(3, "0x3", "0xhash3", db.BlockIngestionPending),
		gapObservation(4, "0x4", "0xhash4a", db.BlockIngestionComplete),
		gapObservation(4, "0x4", "0xhash4b", db.BlockIngestionComplete),
		gapObservation(6, "0x6", "0xhash6", "unknown"),
	}
	want := []string{"0x2", "0x3", "0x4", "0x5", "0x6"}
	if got := repairHeights(1, 6, rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("repairHeights() = %v, want %v", got, want)
	}
}

func TestRepairHeightsAcceptsDuplicateCompletedCanonicalIdentity(t *testing.T) {
	rows := []observedGapBlock{
		gapObservation(1, "0x1", "0xhash1", db.BlockIngestionComplete),
		gapObservation(1, "0x1", "0xHASH1", db.BlockIngestionComplete),
	}
	if got := repairHeights(1, 1, rows); len(got) != 0 {
		t.Fatalf("repairHeights() = %v, want no repair", got)
	}
}
