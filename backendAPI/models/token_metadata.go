package models

// TokenMetadata mirrors QRL2MongoDB/models/token_metadata.go for
// read-through on the backend. Same bson tags so whole-doc reads survive
// the cross-process round trip.
type TokenMetadata struct {
	ContractAddress string           `json:"contractAddress" bson:"contractAddress"`
	TokenID         string           `json:"tokenID" bson:"tokenID"`
	TokenStandard   string           `json:"tokenStandard,omitempty" bson:"tokenStandard,omitempty"`
	URI             string           `json:"uri,omitempty" bson:"uri,omitempty"`
	Name            string           `json:"name,omitempty" bson:"name,omitempty"`
	Description     string           `json:"description,omitempty" bson:"description,omitempty"`
	Image           string           `json:"image,omitempty" bson:"image,omitempty"`
	ExternalURL     string           `json:"externalURL,omitempty" bson:"externalURL,omitempty"`
	Attributes      []TokenAttribute `json:"attributes,omitempty" bson:"attributes,omitempty"`
	FetchedAt       string           `json:"fetchedAt,omitempty" bson:"fetchedAt,omitempty"`
	FetchError      string           `json:"fetchError,omitempty" bson:"fetchError,omitempty"`
	UpdatedAt       string           `json:"updatedAt" bson:"updatedAt"`
}

type TokenAttribute struct {
	TraitType   string `json:"trait_type" bson:"trait_type"`
	Value       string `json:"value" bson:"value"`
	DisplayType string `json:"display_type,omitempty" bson:"display_type,omitempty"`
}
