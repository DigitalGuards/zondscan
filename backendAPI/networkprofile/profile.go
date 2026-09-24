// Package networkprofile guards the database boundary for one explorer network.
// Keep this package identical in the API and syncer, which build independently.
package networkprofile

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

const IdentityCollection = "explorerNetwork"
const identityKey = "identity"

// Identity is immutable once bound. Empty chain fields identify an unpinned
// legacy v2 installation; new networks always require both chain pins.
type Identity struct {
	NetworkID        string `json:"networkId" bson:"networkId"`
	ChainID          string `json:"chainId" bson:"chainId"`
	GenesisHash      string `json:"genesisHash" bson:"genesisHash"`
	AddressBytes     int    `json:"addressBytes" bson:"addressBytes"`
	IdentityVerified bool   `json:"identityVerified" bson:"identityVerified"`
}

type Profile struct {
	Identity     Identity
	DatabaseName string
	Pinned       bool
}

var databaseNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

func Parse(getenv func(string) string, nativeAddressBytes int) (Profile, error) {
	p := Profile{Identity: Identity{NetworkID: strings.TrimSpace(getenv("EXPLORER_NETWORK")), AddressBytes: nativeAddressBytes}, DatabaseName: strings.TrimSpace(getenv("MONGO_DB_NAME"))}
	if p.Identity.NetworkID == "" {
		p.Identity.NetworkID = "v2"
	}
	switch p.Identity.NetworkID {
	case "v2":
		if p.DatabaseName == "" {
			p.DatabaseName = "qrldata-z"
		}
	case "v3":
		if p.DatabaseName == "" || strings.EqualFold(p.DatabaseName, "qrldata-z") {
			return p, errors.New("v3 requires an explicit MONGO_DB_NAME distinct from the v2 database qrldata-z")
		}
	default:
		return p, errors.New("EXPLORER_NETWORK must be v2 or v3")
	}
	if !databaseNamePattern.MatchString(p.DatabaseName) {
		return p, errors.New("MONGO_DB_NAME must contain 1-63 ASCII letters, digits, hyphens or underscores and begin with a letter or digit")
	}
	switch strings.ToLower(p.DatabaseName) {
	case "admin", "local", "config":
		return p, errors.New("MONGO_DB_NAME must be an explorer database")
	}
	chainID, genesis := strings.TrimSpace(getenv("EXPECTED_CHAIN_ID")), strings.TrimSpace(getenv("EXPECTED_GENESIS_HASH"))
	if (chainID == "") != (genesis == "") {
		return p, errors.New("EXPECTED_CHAIN_ID and EXPECTED_GENESIS_HASH must be configured together")
	}
	if p.Identity.NetworkID == "v3" && chainID == "" {
		return p, errors.New("v3 requires EXPECTED_CHAIN_ID and EXPECTED_GENESIS_HASH")
	}
	if chainID != "" {
		var err error
		p.Identity.ChainID, err = NormalizeChainID(chainID)
		if err != nil {
			return p, err
		}
		p.Identity.GenesisHash, err = NormalizeGenesisHash(genesis)
		if err != nil {
			return p, err
		}
		p.Pinned = true
		p.Identity.IdentityVerified = true
	}
	requiredBytes := 20
	if p.Identity.NetworkID == "v3" {
		requiredBytes = 64
	}
	if nativeAddressBytes != requiredBytes {
		return p, fmt.Errorf("%s requires a reviewed %d-byte protocol build; this binary supports %d-byte addresses", p.Identity.NetworkID, requiredBytes, nativeAddressBytes)
	}
	uri := strings.TrimSpace(getenv("MONGOURI"))
	if uri == "" {
		return p, errors.New("MONGOURI is required")
	}
	parsed, err := connstring.ParseAndValidate(uri)
	if err != nil {
		return p, errors.New("MONGOURI is invalid")
	}
	if parsed.Database != "" && parsed.Database != p.DatabaseName {
		return p, errors.New("MONGOURI database suffix must match MONGO_DB_NAME; use authSource for authentication database selection")
	}
	return p, nil
}

func NormalizeChainID(value string) (string, error) {
	base := 10
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		value = value[2:]
	}
	if value == "" || len(value) > 78 {
		return "", errors.New("invalid chain ID")
	}
	for _, c := range value {
		if !((c >= '0' && c <= '9') || (base == 16 && ((c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')))) {
			return "", errors.New("invalid chain ID")
		}
	}
	n, ok := new(big.Int).SetString(value, base)
	if !ok || n.Sign() <= 0 || n.BitLen() > 256 {
		return "", errors.New("invalid chain ID")
	}
	return n.String(), nil
}

func NormalizeGenesisHash(value string) (string, error) {
	if len(value) != 66 || !strings.EqualFold(value[:2], "0x") {
		return "", errors.New("genesis hash must be a 0x-prefixed 32-byte hash")
	}
	if _, err := hex.DecodeString(value[2:]); err != nil {
		return "", errors.New("genesis hash must be a 0x-prefixed 32-byte hash")
	}
	if strings.Trim(value[2:], "0") == "" {
		return "", errors.New("genesis hash must be nonzero")
	}
	return strings.ToLower(value), nil
}

// Bind runs before indexes, seed documents, or collection handles are exposed.
// Concurrent API/syncer starts compete on the same unique singleton key.
func Bind(ctx context.Context, database *mongo.Database, profile Profile) error {
	if database.Name() != profile.DatabaseName {
		return errors.New("selected database does not match the network profile")
	}
	collection := database.Collection(IdentityCollection)
	stored, err := readIdentity(ctx, collection)
	if err == nil {
		return matchIdentity(stored, profile.Identity)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return errors.New("cannot read explorer database identity")
	}
	names, err := database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return errors.New("cannot inspect explorer database identity")
	}
	if profile.Identity.NetworkID == "v3" {
		for _, name := range names {
			if name != IdentityCollection {
				return matchConcurrentIdentity(ctx, collection, profile.Identity, "refusing to adopt an unmarked nonempty database as v3; initialize a separate empty database")
			}
			count, countErr := collection.CountDocuments(ctx, bson.D{}, options.Count().SetLimit(1))
			if countErr != nil || count != 0 {
				return matchConcurrentIdentity(ctx, collection, profile.Identity, "refusing to adopt an unmarked v3 database")
			}
		}
	}
	// A legacy v2 database can already contain genesis. Never label that data
	// with pins from a different chain, including chains sharing a chain ID.
	if profile.Pinned {
		var genesis struct {
			Result struct {
				Hash string `bson:"hash"`
			} `bson:"result"`
		}
		err := database.Collection("blocks").FindOne(ctx, bson.M{"$or": bson.A{bson.M{"blockNumberInt": 0}, bson.M{"result.number": "0x0"}}}).Decode(&genesis)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("cannot inspect stored genesis block")
		}
		if errors.Is(err, mongo.ErrNoDocuments) && len(names) != 0 {
			return matchConcurrentIdentity(ctx, collection, profile.Identity, "populated unmarked database requires stored genesis evidence before pinned adoption")
		}
		if err == nil && !strings.EqualFold(genesis.Result.Hash, profile.Identity.GenesisHash) {
			return errors.New("stored genesis block does not match the configured network")
		}
	}
	document := bson.M{"_id": identityKey, "networkId": profile.Identity.NetworkID, "chainId": profile.Identity.ChainID, "genesisHash": profile.Identity.GenesisHash, "addressBytes": profile.Identity.AddressBytes, "identityVerified": profile.Identity.IdentityVerified}
	if _, err := collection.InsertOne(ctx, document); err != nil && !mongo.IsDuplicateKeyError(err) {
		return errors.New("cannot bind explorer database identity")
	}
	stored, err = readIdentity(ctx, collection)
	if err != nil {
		return errors.New("cannot verify explorer database identity")
	}
	return matchIdentity(stored, profile.Identity)
}

func matchIdentity(stored, expected Identity) error {
	if stored != expected {
		return errors.New("explorer database identity mismatch; choose the database assigned to this network and build")
	}
	return nil
}

func readIdentity(ctx context.Context, collection *mongo.Collection) (Identity, error) {
	var raw bson.Raw
	if err := collection.FindOne(ctx, bson.M{"_id": identityKey}).Decode(&raw); err != nil {
		return Identity{}, err
	}
	if _, ok := raw.Lookup("identityVerified").BooleanOK(); !ok {
		return Identity{}, errors.New("database identity has no explicit verification state")
	}
	for _, key := range []string{"networkId", "chainId", "genesisHash"} {
		if _, ok := raw.Lookup(key).StringValueOK(); !ok {
			return Identity{}, errors.New("database identity fields malformed")
		}
	}
	if _, ok := raw.Lookup("addressBytes").AsInt64OK(); !ok {
		return Identity{}, errors.New("database identity address width malformed")
	}
	var identity Identity
	if err := bson.Unmarshal(raw, &identity); err != nil {
		return Identity{}, errors.New("database identity malformed")
	}
	return identity, nil
}

func matchConcurrentIdentity(ctx context.Context, collection *mongo.Collection, expected Identity, reason string) error {
	// A matching peer can insert the marker between our first read and the
	// emptiness check. Re-read before rejecting a concurrent bootstrap.
	stored, err := readIdentity(ctx, collection)
	if err == nil {
		return matchIdentity(stored, expected)
	}
	return errors.New(reason)
}
