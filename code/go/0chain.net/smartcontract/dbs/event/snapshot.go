package event

import (
	"time"

	"gorm.io/gorm"
)

// swagger:model Block
type Snapshot struct {
	gorm.Model
	Round           int64     `json:"round"`
	BlockHash       string    `json:"hash"`
	MintTotalAmount int64     `json:"mint_total_amount"`
	CreatedAt       time.Time `json:"created_at"`
}

func (edb *EventDb) GetRoundsMintTotal(from, to int64) (int64, error) {
	var total int64
	return total, edb.Store.Get().Model(&Snapshot{}).Where("round between ? and ?", from, to).Select("sum(mint_total_amount)").Scan(&total).Error
}

func (edb *EventDb) addOrUpdateTotalMint(mint Mint) error {
	snapshot, err := edb.getRoundSnapshot(mint.Round)
	if err != nil {
		return err
	}
	snapshot.MintTotalAmount = mint.Amount
	edb.Store.Get().Save(&snapshot)
	return nil
}

func (edb *EventDb) getRoundSnapshot(round int64) (*Snapshot, error) {
	snapshot := &Snapshot{Round: round}
	res := edb.Store.Get().Table("snapshots").Find(&snapshot)
	return snapshot, res.Error
}

func (edb *EventDb) addSnapshot(snapshot Snapshot) error {
	result := edb.Store.Get().Create(&snapshot)
	return result.Error
}
