package db

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"backendAPI/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type inMemoryVerificationUpdater struct {
	contract *models.ContractInfo
	verified bool
}

func (u *inMemoryVerificationUpdater) UpdateOne(
	_ context.Context,
	filterValue interface{},
	_ interface{},
	_ ...*options.UpdateOptions,
) (*mongo.UpdateResult, error) {
	filter, ok := filterValue.(bson.M)
	if !ok || u.contract == nil || !contractMatchesVerificationFilter(*u.contract, filter) {
		return &mongo.UpdateResult{}, nil
	}
	u.verified = true
	return &mongo.UpdateResult{MatchedCount: 1, ModifiedCount: 1}, nil
}

func contractMatchesVerificationFilter(contract models.ContractInfo, filter bson.M) bool {
	return contract.ContractAddress == filter["address"] &&
		contract.CreationTransaction == filter["creationTransaction"] &&
		contract.CreationBlockNumber == filter["creationBlockNumber"] &&
		contract.CreationBlockHash == filter["creationBlockHash"] &&
		contract.ChainID == filter["chainId"] &&
		contract.ContractCodeSHA256 == filter["contractCodeSha256"] &&
		!contract.Verified && !contract.GenesisContract
}

func verificationFenceFixture() (models.VerificationTarget, models.ContractInfo) {
	address := "Q" + strings.Repeat("a", 128)
	target := models.VerificationTarget{
		Address:             address,
		CreationTransaction: "0x" + strings.Repeat("b", 64),
		CreationBlockNumber: "0x10",
		CreationBlockHash:   "0x" + strings.Repeat("c", 64),
		ChainID:             "0x539",
		DeployedCodeSHA256:  strings.Repeat("d", 64),
	}
	contract := models.ContractInfo{
		ContractAddress:     address,
		CreationTransaction: target.CreationTransaction,
		CreationBlockNumber: target.CreationBlockNumber,
		CreationBlockHash:   target.CreationBlockHash,
		ChainID:             target.ChainID,
		ContractCodeSHA256:  target.DeployedCodeSHA256,
	}
	return target, contract
}

func TestVerificationTargetFilterBindsCanonicalDeployment(t *testing.T) {
	target, _ := verificationFenceFixture()
	got := verificationContractTargetFilter(target)
	want := bson.M{
		"address":             target.Address,
		"creationTransaction": target.CreationTransaction,
		"creationBlockNumber": target.CreationBlockNumber,
		"creationBlockHash":   target.CreationBlockHash,
		"chainId":             target.ChainID,
		"contractCodeSha256":  target.DeployedCodeSHA256,
		"verified":            bson.M{"$ne": true},
		"genesisContract":     bson.M{"$ne": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verification target filter = %#v, want %#v", got, want)
	}
}

func TestVerificationCommitFailsClosedWhenMatchedRowIsReplaced(t *testing.T) {
	target, matched := verificationFenceFixture()
	if !contractMatchesVerificationFilter(matched, verificationContractTargetFilter(target)) {
		t.Fatal("precondition: original row did not match the captured target")
	}

	cases := map[string]func(*models.ContractInfo) *models.ContractInfo{
		"deleted": func(*models.ContractInfo) *models.ContractInfo { return nil },
		"recreated at a new canonical block": func(contract *models.ContractInfo) *models.ContractInfo {
			recreated := *contract
			recreated.CreationBlockHash = "0x" + strings.Repeat("e", 64)
			return &recreated
		},
		"recreated with different runtime code": func(contract *models.ContractInfo) *models.ContractInfo {
			recreated := *contract
			recreated.ContractCodeSHA256 = strings.Repeat("f", 64)
			return &recreated
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			// Compilation and byte matching completed against matched. A rollback
			// now deletes or recreates the row before the conditional write.
			updater := &inMemoryVerificationUpdater{contract: mutate(&matched)}
			err := updateContractVerification(
				context.Background(),
				updater,
				target,
				bson.M{"$set": bson.M{"verified": true}},
			)
			if !errors.Is(err, ErrVerificationTargetChanged) {
				t.Fatalf("update error = %v, want verification target change", err)
			}
			if updater.verified {
				t.Fatal("stale worker published verification onto a replacement row")
			}
		})
	}
}

func TestVerificationCommitAcceptsUnchangedMatchedRow(t *testing.T) {
	target, matched := verificationFenceFixture()
	updater := &inMemoryVerificationUpdater{contract: &matched}
	if err := updateContractVerification(
		context.Background(),
		updater,
		target,
		bson.M{"$set": bson.M{"verified": true}},
	); err != nil {
		t.Fatalf("unchanged target update: %v", err)
	}
	if !updater.verified {
		t.Fatal("unchanged target was not verified")
	}
}
