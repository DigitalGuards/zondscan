package db

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestBalanceReconciliationKeysAreCanonicalAndDeterministic(t *testing.T) {
	body := strings.Repeat("ab", 64)
	first, ok := newTokenBalanceReconciliation(
		balanceKindERC1155,
		"0X"+strings.ToUpper(body),
		"q"+body,
		"42",
		"0x10",
	)
	if !ok {
		t.Fatal("expected valid token balance reconciliation")
	}
	second, ok := newTokenBalanceReconciliation(
		balanceKindERC1155,
		"Q"+body,
		"Q"+body,
		"42",
		"0x11",
	)
	if !ok {
		t.Fatal("expected equivalent token balance reconciliation")
	}
	if first.ID != second.ID {
		t.Fatalf("equivalent reconciliation IDs differ: %q and %q", first.ID, second.ID)
	}
	if first.ContractAddress != "Q"+body || first.HolderAddress != "Q"+body {
		t.Fatalf("addresses were not canonicalized: %+v", first)
	}
	if first.RollbackTo == second.RollbackTo {
		t.Fatal("test setup requires distinct rollback targets")
	}
}

func TestERC721ReconciliationIsHolderIndependent(t *testing.T) {
	contract := "Q" + strings.Repeat("12", 64)
	item, ok := newTokenBalanceReconciliation(
		balanceKindERC721,
		contract,
		"",
		"7",
		"0x2",
	)
	if !ok {
		t.Fatal("expected holder-independent ERC-721 reconciliation")
	}
	if item.HolderAddress != "" {
		t.Fatalf("ERC-721 item retained holder: %+v", item)
	}
}

func TestBalanceReconciliationRejectsIncompleteAndZeroHolderKeys(t *testing.T) {
	zero := "Q" + strings.Repeat("0", 128)
	tests := []struct {
		name     string
		kind     string
		contract string
		holder   string
		tokenID  string
	}{
		{name: "missing contract", kind: balanceKindERC20, holder: "Q" + strings.Repeat("1", 128)},
		{name: "missing holder", kind: balanceKindERC20, contract: "Q" + strings.Repeat("2", 128)},
		{name: "zero holder", kind: balanceKindERC20, contract: "Q" + strings.Repeat("2", 128), holder: zero},
		{name: "missing NFT token ID", kind: balanceKindERC721, contract: "Q" + strings.Repeat("2", 128)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if item, ok := newTokenBalanceReconciliation(
				test.kind,
				test.contract,
				test.holder,
				test.tokenID,
				"0x1",
			); ok {
				t.Fatalf("accepted incomplete reconciliation: %+v", item)
			}
		})
	}
}

func TestBalanceReconciliationRejectsMalformedQ128Keys(t *testing.T) {
	valid := "Q" + strings.Repeat("1", 128)
	zero := "Q" + strings.Repeat("0", 128)
	tests := []struct {
		name    string
		address string
	}{
		{name: "unprefixed", address: strings.Repeat("1", 128)},
		{name: "short", address: "Q" + strings.Repeat("1", 127)},
		{name: "long", address: "Q" + strings.Repeat("1", 129)},
		{name: "non-hex", address: "Q" + strings.Repeat("g", 128)},
		{name: "leading whitespace", address: " " + valid},
		{name: "trailing whitespace", address: valid + " "},
		{name: "zero", address: zero},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if item, ok := newNativeBalanceReconciliation(test.address, "0x1"); ok {
				t.Fatalf("accepted malformed Q128 key: %+v", item)
			}
		})
	}

	if item, ok := newTokenBalanceReconciliation(
		balanceKindERC20,
		zero,
		valid,
		"",
		"0x1",
	); ok {
		t.Fatalf("accepted zero contract key: %+v", item)
	}
	if item, ok := newTokenBalanceReconciliation(
		"unknown",
		valid,
		valid,
		"",
		"0x1",
	); ok {
		t.Fatalf("accepted unsupported key kind: %+v", item)
	}
	if item, ok := newTokenBalanceReconciliation(
		balanceKindERC20,
		valid,
		valid,
		"1",
		"0x1",
	); ok {
		t.Fatalf("accepted tokenID on ERC-20 key: %+v", item)
	}
	if item, ok := newTokenBalanceReconciliation(
		balanceKindERC721,
		valid,
		valid,
		"1",
		"0x1",
	); ok {
		t.Fatalf("accepted holder on ERC-721 key: %+v", item)
	}
}

func TestCanonicalBalanceTokenIDEnforcesUint256Decimal(t *testing.T) {
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	overflow := new(big.Int).Lsh(big.NewInt(1), 256).String()
	valid := []string{"0", "1", "42", maxUint256}
	for _, tokenID := range valid {
		if canonical, ok := canonicalBalanceTokenID(tokenID); !ok || canonical != tokenID {
			t.Fatalf("valid tokenID %q canonicalized to %q, ok=%v", tokenID, canonical, ok)
		}
	}
	invalid := []string{"", "00", "01", "-1", "+1", " 1", "1 ", "1.0", "0x1", overflow}
	for _, tokenID := range invalid {
		if canonical, ok := canonicalBalanceTokenID(tokenID); ok {
			t.Fatalf("invalid tokenID %q canonicalized to %q", tokenID, canonical)
		}
	}
}

func TestValidateBalanceReconciliationRejectsTamperedKey(t *testing.T) {
	item, ok := newNativeBalanceReconciliation("Q"+strings.Repeat("1", 128), "0x1")
	if !ok {
		t.Fatal("expected valid native key")
	}
	item.ID = "balance:tampered"
	if err := validateBalanceReconciliation(item); err == nil {
		t.Fatal("accepted reconciliation whose ID does not bind its canonical key")
	}
}

func TestBalanceReconciliationRetryBackoffIsBounded(t *testing.T) {
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{attempts: -1, want: 5 * time.Second},
		{attempts: 1, want: 5 * time.Second},
		{attempts: 2, want: 10 * time.Second},
		{attempts: 3, want: 20 * time.Second},
		{attempts: 7, want: 5 * time.Minute},
		{attempts: 1000, want: 5 * time.Minute},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("attempts_%d", test.attempts), func(t *testing.T) {
			if got := balanceReconciliationRetryBackoff(test.attempts); got != test.want {
				t.Fatalf("backoff = %s, want %s", got, test.want)
			}
		})
	}
}

func TestBoundedBalanceReconciliationError(t *testing.T) {
	message := strings.Repeat("failure", balanceReconciliationErrorMax)
	got := []rune(boundedBalanceReconciliationError(fmt.Errorf("%s", message)))
	if len(got) != balanceReconciliationErrorMax {
		t.Fatalf("bounded error length = %d, want %d", len(got), balanceReconciliationErrorMax)
	}
}

func TestDedupeBalanceReconciliationsUsesStableIDOrder(t *testing.T) {
	a, ok := newNativeBalanceReconciliation("Q"+strings.Repeat("1", 128), "0x1")
	if !ok {
		t.Fatal("expected first native item")
	}
	b, ok := newNativeBalanceReconciliation("Q"+strings.Repeat("2", 128), "0x1")
	if !ok {
		t.Fatal("expected second native item")
	}
	items := dedupeBalanceReconciliations([]balanceReconciliation{b, a, b})
	if len(items) != 2 {
		t.Fatalf("deduped length = %d, want 2", len(items))
	}
	if items[0].ID > items[1].ID {
		t.Fatalf("deduped IDs are not sorted: %q then %q", items[0].ID, items[1].ID)
	}
}

func TestBalanceReconciliationsForOrphanedTokenTransfers(t *testing.T) {
	contract20 := "Q" + strings.Repeat("1", 128)
	contract721 := "Q" + strings.Repeat("2", 128)
	contract1155 := "Q" + strings.Repeat("3", 128)
	holderA := "Q" + strings.Repeat("a", 128)
	holderB := "Q" + strings.Repeat("b", 128)
	holderC := "Q" + strings.Repeat("c", 128)
	zero := "Q" + strings.Repeat("0", 128)

	items := balanceReconciliationsForTokenTransfers([]models.TokenTransfer{
		{
			ContractAddress: contract20,
			From:            holderA,
			To:              holderB,
		},
		{
			ContractAddress: contract20,
			From:            holderB,
			To:              holderA,
			TokenStandard:   rpc.StandardERC20,
		},
		{
			ContractAddress: contract721,
			From:            holderA,
			To:              holderB,
			TokenStandard:   rpc.StandardERC721,
			TokenID:         "7",
		},
		{
			ContractAddress: contract1155,
			From:            zero,
			To:              holderC,
			TokenStandard:   rpc.StandardERC1155,
			TokenID:         "9",
		},
	}, "0x20")

	counts := map[string]int{}
	for _, item := range items {
		counts[item.Kind]++
		if item.RollbackTo != "0x20" {
			t.Fatalf("rollback target = %q, want 0x20", item.RollbackTo)
		}
		if item.HolderAddress == zero {
			t.Fatalf("zero-address holder was queued: %+v", item)
		}
	}
	if counts[balanceKindERC20] != 2 {
		t.Fatalf("ERC-20 reconciliation count = %d, want 2", counts[balanceKindERC20])
	}
	if counts[balanceKindERC721] != 1 {
		t.Fatalf("ERC-721 reconciliation count = %d, want 1", counts[balanceKindERC721])
	}
	if counts[balanceKindERC1155] != 1 {
		t.Fatalf("ERC-1155 reconciliation count = %d, want 1", counts[balanceKindERC1155])
	}
}
