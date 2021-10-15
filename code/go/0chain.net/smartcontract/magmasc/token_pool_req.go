package magmasc

import (
	"encoding/json"

	"github.com/0chain/gosdk/core/util"
	"github.com/0chain/gosdk/zmagmacore/errors"
	zmc "github.com/0chain/gosdk/zmagmacore/magmasc"

	tx "0chain.net/chaincore/transaction"
)

type (
	// tokenPoolReq represents lock pool request implementation.
	tokenPoolReq struct {
		ID       string        `json:"id"`
		Provider *zmc.Provider `json:"provider"`
		txn      *tx.Transaction
	}
)

var (
	// Make sure tokenPoolReq implements Serializable interface.
	_ util.Serializable = (*tokenPoolReq)(nil)

	// Make sure tokenPoolReq implements PoolConfigurator interface.
	_ zmc.PoolConfigurator = (*tokenPoolReq)(nil)
)

// Decode implements util.Serializable interface.
func (m *tokenPoolReq) Decode(blob []byte) error {
	req := tokenPoolReq{txn: m.txn}
	if err := json.Unmarshal(blob, &req); err != nil {
		return zmc.ErrDecodeData.Wrap(err)
	}
	if err := req.Validate(); err != nil {
		return err
	}

	m.ID = req.ID
	m.Provider = req.Provider

	return nil
}

// Encode implements util.Serializable interface.
func (m *tokenPoolReq) Encode() []byte {
	blob, _ := json.Marshal(m)
	return blob
}

// PoolBalance implements PoolConfigurator interface.
func (m *tokenPoolReq) PoolBalance() int64 {
	return m.txn.Value
}

// PoolID implements PoolConfigurator interface.
func (m *tokenPoolReq) PoolID() string {
	return m.ID
}

// PoolHolderID implements PoolConfigurator interface.
func (m *tokenPoolReq) PoolHolderID() string {
	return zmc.Address
}

// PoolPayerID implements PoolConfigurator interface.
func (m *tokenPoolReq) PoolPayerID() string {
	return m.txn.ClientID
}

// PoolPayeeID implements PoolConfigurator interface.
func (m *tokenPoolReq) PoolPayeeID() string {
	return m.Provider.Id
}

// Validate checks tokenPoolReq for correctness.
func (m *tokenPoolReq) Validate() (err error) {
	switch { // is invalid
	case m.txn == nil:
		err = errors.New(zmc.ErrCodeInternal, "transaction data is required")

	case m.txn.Value <= 0:
		err = errors.New(zmc.ErrCodeInternal, "transaction value is required")

	case m.ID == "":
		err = errors.New(zmc.ErrCodeBadRequest, "pool id is required")

	case m.Provider == nil || m.Provider.ExtId == "":
		err = errors.New(zmc.ErrCodeBadRequest, "provider external id is required")
	}

	return err
}
