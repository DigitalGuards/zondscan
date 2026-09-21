package models

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// AIExplanationLease is backend-owned coordination state for one paid model
// call. It is intentionally excluded from API responses. SourceDigest and
// VerifiedAt bind the lease to one exact verification generation.
type AIExplanationLease struct {
	Token        string    `json:"-" bson:"token"`
	SourceDigest string    `json:"-" bson:"sourceDigest"`
	VerifiedAt   string    `json:"-" bson:"verifiedAt"`
	AcquiredAt   time.Time `json:"-" bson:"acquiredAt"`
	ExpiresAt    time.Time `json:"-" bson:"expiresAt"`
}

const (
	VerificationRecordSchemaV1 = "qrl.contract-verification-record.v1"
	VerificationRecordSchemaV2 = "qrl.contract-verification-record.v2"
)

const (
	CreatorAddressProvenanceDirectDeployment  = "direct-deployment"
	CreatorAddressProvenanceCreateTraceOuter  = "create-trace-outer-sender"
	CreatorAddressProvenanceCreateTraceCaller = "create-trace-caller"
	CreatorAddressProvenanceMintHeuristic     = "mint-heuristic"
	CreatorAddressProvenanceGenesis           = "genesis"
	CreatorAddressProvenanceUnclassified      = "creation-tx-unclassified"
)

func IsAuthoritativeCreatorAddressProvenance(value string) bool {
	return value == CreatorAddressProvenanceDirectDeployment ||
		value == CreatorAddressProvenanceCreateTraceOuter
}

// ContractInfo is the persisted contract document in the `contractCode`
// MongoDB collection.
//
// The verification fields at the bottom are written **only** by the backend
// verify endpoint (see backendAPI/db/contract.go:MarkContractVerified). The
// syncer (QRL2MongoDB) treats them as opaque pass-through state and must
// never write into them, see QRL2MongoDB/db/contracts.go:StoreContract for
// the field-scoped $set that enforces this invariant.
//
// New verification fields MUST be mirrored in QRL2MongoDB/models/contract.go
// with identical bson and json tags so the syncer's whole-doc reads survive
// round-trips.
type ContractInfo struct {
	ContractCreatorAddress   string `json:"creatorAddress" bson:"creatorAddress"`
	CreatorAddressProvenance string `json:"creatorAddressProvenance,omitempty" bson:"creatorAddressProvenance,omitempty"`
	ContractAddress          string `json:"address" bson:"address"`
	ContractCode             string `json:"contractCode" bson:"contractCode"`
	ContractCodeSHA256       string `json:"contractCodeSha256,omitempty" bson:"contractCodeSha256,omitempty"`
	CreationTransaction      string `json:"creationTransaction" bson:"creationTransaction"`
	CreationBlockNumber      string `json:"creationBlockNumber" bson:"creationBlockNumber"`
	CreationBlockHash        string `json:"creationBlockHash,omitempty" bson:"creationBlockHash,omitempty"`
	ChainID                  string `json:"chainId,omitempty" bson:"chainId,omitempty"`
	IsToken                  bool   `json:"isToken" bson:"isToken"`
	Status                   string `json:"status" bson:"status"`
	TokenDecimals            uint8  `json:"decimals" bson:"decimals"`
	TokenName                string `json:"name" bson:"name"`
	TokenSymbol              string `json:"symbol" bson:"symbol"`
	TotalSupply              string `json:"totalSupply" bson:"totalSupply"`
	UpdatedAt                string `json:"updatedAt" bson:"updatedAt"`

	// NFT / multi-token fields, mirror QRL2MongoDB/models/contract.go.
	// Written exclusively by the syncer; held here only for round-trip
	// preservation and to allow the /contracts endpoint to filter on
	// `?standard=ERC-721`.
	TokenStandard string `json:"tokenStandard,omitempty" bson:"tokenStandard,omitempty"`
	HasERC165     bool   `json:"hasERC165,omitempty" bson:"hasERC165,omitempty"`
	BaseURI       string `json:"baseURI,omitempty" bson:"baseURI,omitempty"`

	// Set by the syncer for contracts baked into the genesis allocation
	// (no creation transaction exists). Pass-through so the frontend can
	// label them; mirror QRL2MongoDB/models/contract.go.
	GenesisContract bool `json:"genesisContract,omitempty" bson:"genesisContract,omitempty"`

	// Collection-level off-chain metadata (Phase 3a). Mirrors
	// QRL2MongoDB/models/contract.go. Written by the syncer's metadata
	// fetcher service; the backend holds them for read-through to clients.
	MetadataURI         string `json:"metadataURI,omitempty" bson:"metadataURI,omitempty"`
	MetadataName        string `json:"metadataName,omitempty" bson:"metadataName,omitempty"`
	MetadataDescription string `json:"metadataDescription,omitempty" bson:"metadataDescription,omitempty"`
	MetadataImage       string `json:"metadataImage,omitempty" bson:"metadataImage,omitempty"`
	MetadataExternalURL string `json:"metadataExternalURL,omitempty" bson:"metadataExternalURL,omitempty"`
	MetadataFetchedAt   string `json:"metadataFetchedAt,omitempty" bson:"metadataFetchedAt,omitempty"`
	MetadataFetchError  string `json:"metadataFetchError,omitempty" bson:"metadataFetchError,omitempty"`

	// Source-verification fields. `verified` defaults to false (omitted from
	// omitempty so it is always present in the JSON shape clients consume).
	// Everything else uses omitempty so unverified contracts stay clean.
	Verified                     bool   `json:"verified" bson:"verified"`
	VerificationRecordSchema     string `json:"verificationRecordSchema,omitempty" bson:"verificationRecordSchema,omitempty"`
	verificationRecordSchemaSeen bool
	verificationRecordSchemaNull bool
	SourceCode                   string              `json:"sourceCode,omitempty" bson:"sourceCode,omitempty"`
	Abi                          string              `json:"abi,omitempty" bson:"abi,omitempty"`
	ContractName                 string              `json:"contractName,omitempty" bson:"contractName,omitempty"`
	CompilerVersion              string              `json:"compilerVersion,omitempty" bson:"compilerVersion,omitempty"`
	CompilerProvenance           *CompilerProvenance `json:"compilerProvenance,omitempty" bson:"compilerProvenance,omitempty"`
	compilerProvenanceSeen       bool
	compilerProvenanceNull       bool
	OptimizationEnabled          bool              `json:"optimizationEnabled" bson:"optimizationEnabled"`
	OptimizationRuns             int               `json:"optimizationRuns" bson:"optimizationRuns"`
	EvmVersion                   string            `json:"evmVersion,omitempty" bson:"evmVersion,omitempty"`
	ConstructorArguments         string            `json:"constructorArguments,omitempty" bson:"constructorArguments,omitempty"`
	Libraries                    map[string]string `json:"libraries,omitempty" bson:"libraries,omitempty"`
	Imports                      map[string]string `json:"imports,omitempty" bson:"imports,omitempty"`
	SourceBundleDigest           string            `json:"sourceBundleDigest,omitempty" bson:"sourceBundleDigest,omitempty"`
	VerificationArtifactDigest   string            `json:"verificationArtifactDigest,omitempty" bson:"verificationArtifactDigest,omitempty"`
	sourceBundleDigestSeen       bool
	sourceBundleDigestNull       bool
	License                      string `json:"license,omitempty" bson:"license,omitempty"`
	VerificationMethod           string `json:"verificationMethod,omitempty" bson:"verificationMethod,omitempty"`
	VerifiedAt                   string `json:"verifiedAt,omitempty" bson:"verifiedAt,omitempty"`

	// M6a AI explanation cache. Populated only when an authorised user has
	// triggered POST /contract/explain/:address. The syncer must NOT write
	// into these, kept off the syncer's allow-list in
	// QRL2MongoDB/db/contracts.go:StoreContract.
	AIExplanation      string `json:"aiExplanation,omitempty" bson:"aiExplanation,omitempty"`
	AIExplanationAt    string `json:"aiExplanationAt,omitempty" bson:"aiExplanationAt,omitempty"`
	AIExplanationModel string `json:"aiExplanationModel,omitempty" bson:"aiExplanationModel,omitempty"`
	// AIExplanationSourceDigest binds the cached text to the exact versioned
	// source bundle used as model input.
	AIExplanationSourceDigest string              `json:"aiExplanationSourceDigest,omitempty" bson:"aiExplanationSourceDigest,omitempty"`
	AIExplanationLease        *AIExplanationLease `json:"-" bson:"aiExplanationLease,omitempty"`

	// Regen cap state. The rolling 7-day window starts when the first
	// regen in a window fires, then resets after the window expires.
	// AIExplanationRegenCount counts regens within the current window
	// (initial generates don't count). See aiexplain.RegenLimitPerWindow.
	AIExplanationRegenCount       int    `json:"aiExplanationRegenCount,omitempty" bson:"aiExplanationRegenCount,omitempty"`
	AIExplanationRegenWindowStart string `json:"aiExplanationRegenWindowStart,omitempty" bson:"aiExplanationRegenWindowStart,omitempty"`
}

// UnmarshalBSON tracks whether verificationRecordSchema was absent or was
// explicitly stored as BSON null. Both decode to an empty Go string, while the
// distinction is required to keep genuinely absent legacy records separate
// from malformed schema-marked records.
func (c *ContractInfo) UnmarshalBSON(data []byte) error {
	type contractInfoAlias ContractInfo

	var decoded contractInfoAlias
	if err := bson.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*c = ContractInfo(decoded)
	raw := bson.Raw(data)
	verificationRecordSchema := raw.Lookup("verificationRecordSchema")
	compilerProvenance := raw.Lookup("compilerProvenance")
	sourceBundleDigest := raw.Lookup("sourceBundleDigest")
	c.verificationRecordSchemaSeen = verificationRecordSchema.Type != 0
	c.verificationRecordSchemaNull = verificationRecordSchema.Type == bsontype.Null
	c.compilerProvenanceSeen = compilerProvenance.Type != 0
	c.compilerProvenanceNull = compilerProvenance.Type == bsontype.Null
	c.sourceBundleDigestSeen = sourceBundleDigest.Type != 0
	c.sourceBundleDigestNull = sourceBundleDigest.Type == bsontype.Null
	return nil
}

// MarshalJSON preserves BSON null versus absent semantics for the three trust
// fields. Ordinary values retain their existing JSON form, genuinely absent
// legacy fields remain omitted, and explicit BSON null remains JSON null.
func (c ContractInfo) MarshalJSON() ([]byte, error) {
	type contractInfoAlias ContractInfo
	type contractInfoJSON struct {
		contractInfoAlias
		VerificationRecordSchema **string             `json:"verificationRecordSchema,omitempty"`
		CompilerProvenance       **CompilerProvenance `json:"compilerProvenance,omitempty"`
		SourceBundleDigest       **string             `json:"sourceBundleDigest,omitempty"`
	}

	return json.Marshal(contractInfoJSON{
		contractInfoAlias: contractInfoAlias(c),
		VerificationRecordSchema: jsonStringField(
			c.VerificationRecordSchema,
			c.verificationRecordSchemaSeen,
			c.verificationRecordSchemaNull,
		),
		CompilerProvenance: jsonCompilerProvenanceField(
			c.CompilerProvenance,
			c.compilerProvenanceSeen,
			c.compilerProvenanceNull,
		),
		SourceBundleDigest: jsonStringField(
			c.SourceBundleDigest,
			c.sourceBundleDigestSeen,
			c.sourceBundleDigestNull,
		),
	})
}

func jsonStringField(value string, seen, explicitNull bool) **string {
	if !seen && value == "" {
		return nil
	}
	if explicitNull {
		var nullValue *string
		return &nullValue
	}
	valueCopy := value
	valuePointer := &valueCopy
	return &valuePointer
}

func jsonCompilerProvenanceField(
	value *CompilerProvenance,
	seen bool,
	explicitNull bool,
) **CompilerProvenance {
	if !seen && value == nil {
		return nil
	}
	if explicitNull {
		var nullValue *CompilerProvenance
		return &nullValue
	}
	valuePointer := value
	return &valuePointer
}

// HasVerificationRecordSchema reports BSON field presence as well as a schema
// assigned directly by an in-memory writer.
func (c ContractInfo) HasVerificationRecordSchema() bool {
	return c.verificationRecordSchemaSeen || c.VerificationRecordSchema != ""
}

// HasCompilerProvenanceField distinguishes an absent legacy field from an
// explicitly stored BSON null.
func (c ContractInfo) HasCompilerProvenanceField() bool {
	return c.compilerProvenanceSeen || c.CompilerProvenance != nil
}

// HasSourceBundleDigestField distinguishes an absent legacy field from an
// explicitly stored BSON null.
func (c ContractInfo) HasSourceBundleDigestField() bool {
	return c.sourceBundleDigestSeen || c.SourceBundleDigest != ""
}
