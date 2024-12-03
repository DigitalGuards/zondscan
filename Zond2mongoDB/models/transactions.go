package models

type TransactionData struct {
	BlockHash           []byte
	BlockNumber         uint64
	From                []byte
	Hash                []byte
	Nonce               uint64
	TransactionIndex    uint64
	BlockProposerReward uint64
	AttestorReward      uint64
	FeeReward           uint64
	TxType              uint8
	ChainId             uint8
	Signature           []byte
	PublicKey           []byte
}
