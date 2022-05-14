package state

import (
	"encoding/json"

	"0chain.net/core/common"
	"0chain.net/core/datastore"
)

var ErrInvalidMint = common.NewError("invalid_mint", "invalid minter")

type Mint struct {
	Minter     datastore.Key `json:"minter"`
	ToClientID datastore.Key `json:"to"`
	Amount     int64         `json:"amount"`
}

func NewMint(minter, toClientID datastore.Key, amount int64) *Mint {
	m := &Mint{Minter: minter, ToClientID: toClientID, Amount: amount}
	return m
}

func (m *Mint) Encode() []byte {
	buff, _ := json.Marshal(m)
	return buff
}

func (m *Mint) Decode(input []byte) error {
	err := json.Unmarshal(input, m)
	return err
}
