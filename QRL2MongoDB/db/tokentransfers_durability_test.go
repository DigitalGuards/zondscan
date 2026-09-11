package db

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"errors"
	"strings"
	"testing"
)

func durabilityAddress(hexDigit string) string {
	return "Q" + strings.Repeat(hexDigit, 128)
}

func durabilityTopic(address string) string {
	return "0x" + strings.TrimPrefix(strings.ToLower(address), "q")
}

func durabilityABIWord(value string) string {
	return strings.Repeat("0", 128-len(value)) + value
}

func durabilityBlock() models.ZondDatabaseBlock {
	blockHash := "0x" + strings.Repeat("a", 64)
	txHash := "0x" + strings.Repeat("b", 64)
	block := models.ZondDatabaseBlock{}
	block.Result.Number = "0x2a"
	block.Result.Hash = blockHash
	block.Result.Timestamp = "0x100"
	block.Result.TransactionsRoot = "0x" + strings.Repeat("c", 64)
	block.Result.Transactions = []models.Transaction{{
		Hash:             txHash,
		BlockNumber:      "0x2a",
		BlockHash:        blockHash,
		TransactionIndex: "0x0",
	}}
	return block
}

func durabilityLog() models.Log {
	block := durabilityBlock()
	return models.Log{
		Address:          durabilityAddress("d"),
		Topics:           []string{rpc.TransferEventSignature, durabilityTopic(durabilityAddress("1")), durabilityTopic(durabilityAddress("2"))},
		Data:             "0x" + strings.Repeat("0", 127) + "1",
		BlockNumber:      block.Result.Number,
		BlockHash:        block.Result.Hash,
		TransactionHash:  block.Result.Transactions[0].Hash,
		TransactionIndex: "0x0",
		LogIndex:         "0x0",
	}
}

func TestCanonicalTokenBlockIdentityBindsTransactionSet(t *testing.T) {
	block := durabilityBlock()
	identity, err := canonicalTokenBlockIdentity(block)
	if err != nil {
		t.Fatal(err)
	}
	if identity.number != block.Result.Number || identity.hash != block.Result.Hash ||
		identity.transactionIndex[block.Result.Transactions[0].Hash] != "0x0" ||
		len(identity.transactionSet) != 1 {
		t.Fatalf("unexpected identity: %+v", identity)
	}

	wrongHash := block
	wrongHash.Result.Transactions = append([]models.Transaction(nil), block.Result.Transactions...)
	wrongHash.Result.Transactions[0].BlockHash = "0x" + strings.Repeat("e", 64)
	if _, err := canonicalTokenBlockIdentity(wrongHash); err == nil ||
		!strings.Contains(err.Error(), "does not match block") {
		t.Fatalf("wrong transaction block hash error = %v", err)
	}

	wrongIndex := block
	wrongIndex.Result.Transactions = append([]models.Transaction(nil), block.Result.Transactions...)
	wrongIndex.Result.Transactions[0].TransactionIndex = "0x1"
	if _, err := canonicalTokenBlockIdentity(wrongIndex); err == nil ||
		!strings.Contains(err.Error(), "does not match position") {
		t.Fatalf("wrong transaction index error = %v", err)
	}
}

func TestNormalizeTokenBlockLogsRejectsEveryIdentityMismatch(t *testing.T) {
	identity, err := canonicalTokenBlockIdentity(durabilityBlock())
	if err != nil {
		t.Fatal(err)
	}
	canonical := durabilityLog()
	got, err := normalizeTokenBlockLogs([]models.Log{canonical}, identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BlockHash != identity.hash || got[0].Address != durabilityAddress("d") {
		t.Fatalf("normalized log = %+v", got)
	}

	tests := []struct {
		name   string
		mutate func(*models.Log)
		want   string
	}{
		{name: "removed", mutate: func(log *models.Log) { log.Removed = true }, want: "marked removed"},
		{name: "block number", mutate: func(log *models.Log) { log.BlockNumber = "0x2b" }, want: "block number"},
		{name: "block hash", mutate: func(log *models.Log) { log.BlockHash = "0x" + strings.Repeat("e", 64) }, want: "block hash"},
		{name: "transaction member", mutate: func(log *models.Log) { log.TransactionHash = "0x" + strings.Repeat("e", 64) }, want: "absent from block"},
		{name: "transaction index", mutate: func(log *models.Log) { log.TransactionIndex = "0x1" }, want: "transaction index"},
		{name: "log index", mutate: func(log *models.Log) { log.LogIndex = "one" }, want: "log index"},
		{name: "emitter", mutate: func(log *models.Log) { log.Address = "Qbad" }, want: "invalid emitter"},
		{name: "event", mutate: func(log *models.Log) { log.Topics[0] = "0x" + strings.Repeat("f", 128) }, want: "unexpected event"},
		{name: "non-topic0 schema", mutate: func(log *models.Log) { log.Topics[1] = "0x01" }, want: "topic 1"},
		{name: "event data", mutate: func(log *models.Log) { log.Data = "bad" }, want: "invalid data"},
		{name: "odd-length event data", mutate: func(log *models.Log) { log.Data = "0x1" }, want: "invalid data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := canonical
			log.Topics = append([]string(nil), canonical.Topics...)
			test.mutate(&log)
			if _, err := normalizeTokenBlockLogs([]models.Log{log}, identity); err == nil ||
				!strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}

	duplicate := canonical
	duplicate.Topics = append([]string(nil), canonical.Topics...)
	if _, err := normalizeTokenBlockLogs([]models.Log{canonical, duplicate}, identity); err == nil ||
		!strings.Contains(err.Error(), "duplicate token log index") {
		t.Fatalf("duplicate log error = %v", err)
	}
	malformedPayload := canonical
	malformedPayload.Topics = append([]string(nil), canonical.Topics[:2]...)
	malformedPayload.Data = "0x01"
	if _, err := normalizeTokenBlockLogs([]models.Log{malformedPayload}, identity); err != nil {
		t.Fatalf("identity normalization rejected payload before classification: %v", err)
	}
}

func TestDecodeTransferLogCarriesCanonicalBlockHash(t *testing.T) {
	block := durabilityBlock()
	log := durabilityLog()
	contract := &models.ContractInfo{
		Address:       log.Address,
		TokenStandard: rpc.StandardERC20,
		Symbol:        "QTA",
		Name:          "Quanta",
		Decimals:      18,
	}
	rows, err := decodeTransferLog(
		log,
		contract,
		rpc.StandardERC20,
		block.Result.Number,
		block.Result.Hash,
		block.Result.Timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].BlockHash != block.Result.Hash {
		t.Fatalf("decoded rows = %+v", rows)
	}
}

func TestDecodeERC1155BatchAggregatesRepeatedTokenIDs(t *testing.T) {
	block := durabilityBlock()
	log := durabilityLog()
	log.Topics = []string{
		rpc.TransferBatchEventSignature,
		durabilityTopic(durabilityAddress("3")),
		durabilityTopic(durabilityAddress("1")),
		durabilityTopic(durabilityAddress("2")),
	}
	batchData := func(firstValue, secondValue string) string {
		return "0x" +
			durabilityABIWord("80") +
			durabilityABIWord("140") +
			durabilityABIWord("2") +
			durabilityABIWord("7") +
			durabilityABIWord("7") +
			durabilityABIWord("2") +
			durabilityABIWord(firstValue) +
			durabilityABIWord(secondValue)
	}
	log.Data = batchData("1", "2")
	contract := &models.ContractInfo{
		Address:       log.Address,
		TokenStandard: rpc.StandardERC1155,
		Name:          "Repeated IDs",
	}
	rows, err := decodeTransferLog(
		log,
		contract,
		rpc.StandardERC1155,
		block.Result.Number,
		block.Result.Hash,
		block.Result.Timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TokenID != "7" || rows[0].Amount != "3" {
		t.Fatalf("aggregated rows = %+v", rows)
	}

	log.Data = batchData(strings.Repeat("f", 64), "1")
	if _, err := decodeTransferLog(
		log,
		contract,
		rpc.StandardERC1155,
		block.Result.Number,
		block.Result.Hash,
		block.Result.Timestamp,
	); err == nil || !strings.Contains(err.Error(), "exceeds uint256") {
		t.Fatalf("overflow error = %v", err)
	}
}

func TestPlanTokenBlockLogsQuarantinesDeterministicMalformedTokenEvent(t *testing.T) {
	identity, err := canonicalTokenBlockIdentity(durabilityBlock())
	if err != nil {
		t.Fatal(err)
	}
	valid := durabilityLog()
	malformed := durabilityLog()
	malformed.LogIndex = "0x1"
	malformed.Data = "0x01"
	persistCalls := 0
	deadLetterStoreCalls := 0
	plans, deadLetters, err := planTokenBlockLogsWithClassification(
		[]models.Log{valid, malformed},
		identity,
		nil,
		func(address, _, _, _ string) (preparedContractClassification, error) {
			return preparedContractClassification{
				contract: &models.ContractInfo{
					Address:       address,
					TokenStandard: rpc.StandardERC20,
					Name:          "Deferred",
				},
				standard: rpc.StandardERC20,
				persist:  true,
			}, nil
		},
		func(preparedContractClassification) error {
			persistCalls++
			return nil
		},
		func(models.TokenEventDeadLetter) error {
			deadLetterStoreCalls++
			if persistCalls != 0 {
				t.Fatal("classification persisted before deterministic rejection was audited")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if persistCalls != 1 {
		t.Fatalf("classification persistence calls = %d, want 1 after complete planning", persistCalls)
	}
	if deadLetterStoreCalls != 1 {
		t.Fatalf("dead-letter store calls = %d, want 1", deadLetterStoreCalls)
	}
	if len(plans) != 1 || len(plans[0].rows) != 1 {
		t.Fatalf("valid plans = %+v", plans)
	}
	if len(deadLetters) != 1 {
		t.Fatalf("dead-letters = %+v", deadLetters)
	}
	deadLetter := deadLetters[0]
	if deadLetter.BlockHash != identity.hash ||
		deadLetter.TxHash != malformed.TransactionHash ||
		deadLetter.LogIndex != malformed.LogIndex ||
		deadLetter.Emitter != malformed.Address ||
		deadLetter.Topic0 != malformed.Topics[0] ||
		deadLetter.TokenStandard != rpc.StandardERC20 ||
		!strings.Contains(deadLetter.Reason, "decode rejected") {
		t.Fatalf("dead-letter identity = %+v", deadLetter)
	}
	if len(deadLetter.Reason) > tokenEventDeadLetterReasonLimit {
		t.Fatalf("dead-letter reason length = %d", len(deadLetter.Reason))
	}
}

func TestPlanTokenBlockLogsSkipsMalformedPayloadFromClassifiedNonToken(t *testing.T) {
	identity, err := canonicalTokenBlockIdentity(durabilityBlock())
	if err != nil {
		t.Fatal(err)
	}
	malformed := durabilityLog()
	malformed.Data = "0x01"
	malformed.Topics = malformed.Topics[:2]
	plans, deadLetters, err := planTokenBlockLogsWithClassification(
		[]models.Log{malformed},
		identity,
		nil,
		func(address, _, _, _ string) (preparedContractClassification, error) {
			return preparedContractClassification{contract: &models.ContractInfo{Address: address}}, nil
		},
		func(preparedContractClassification) error {
			t.Fatal("non-token classification unexpectedly requested persistence")
			return nil
		},
		func(models.TokenEventDeadLetter) error {
			t.Fatal("malformed non-token payload unexpectedly requested dead-letter persistence")
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 || len(deadLetters) != 0 {
		t.Fatalf("non-token malformed payload planned effects: plans=%+v deadLetters=%+v", plans, deadLetters)
	}
}

func TestPlanTokenBlockLogsAuditFailurePreventsClassificationPersistence(t *testing.T) {
	identity, err := canonicalTokenBlockIdentity(durabilityBlock())
	if err != nil {
		t.Fatal(err)
	}
	malformed := durabilityLog()
	malformed.Data = "0x01"
	auditErr := errors.New("dead-letter collection unavailable")
	persistCalls := 0
	_, _, err = planTokenBlockLogsWithClassification(
		[]models.Log{malformed},
		identity,
		nil,
		func(address, _, _, _ string) (preparedContractClassification, error) {
			return preparedContractClassification{
				contract: &models.ContractInfo{Address: address},
				standard: rpc.StandardERC20,
				persist:  true,
			}, nil
		},
		func(preparedContractClassification) error {
			persistCalls++
			return nil
		},
		func(models.TokenEventDeadLetter) error {
			return auditErr
		},
	)
	if !errors.Is(err, auditErr) {
		t.Fatalf("audit error = %v, want %v", err, auditErr)
	}
	if persistCalls != 0 {
		t.Fatalf("classification persisted %d times after audit failure", persistCalls)
	}
}
