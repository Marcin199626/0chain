package event

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type AllocationBlobberTerm struct {
	gorm.Model
	AllocationID            string        `json:"allocation_id" gorm:"primaryKey; not null"` // Foreign Key
	BlobberID               string        `json:"blobber_id" gorm:"primaryKey; not null"`    // Foreign Key
	ReadPrice               int64         `json:"read_price"`
	WritePrice              int64         `json:"write_price"`
	MinLockDemand           float64       `json:"min_lock_demand"`
	MaxOfferDuration        time.Duration `json:"max_offer_duration"`
	ChallengeCompletionTime time.Duration `json:"challenge_completion_time"`
	Allocation              Allocation    `json:"-" gorm:"references:AllocationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Blobber                 Blobber       `json:"-" gorm:"references:BlobberID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (edb *EventDb) GetAllocationBlobberTerm(allocationID string, blobberID string) (*AllocationBlobberTerm, error) {
	if allocationID == "" && blobberID == "" {
		return nil, fmt.Errorf("can not retrieve term with empty Allocation ID and blobber ID")
	}
	var term AllocationBlobberTerm
	if err := edb.Store.Get().Model(&AllocationBlobberTerm{}).
		Where(&AllocationBlobberTerm{AllocationID: allocationID, BlobberID: blobberID}).
		Scan(&term).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve term: AllocationID %v and blobberID %v does not exist, error %v",
			allocationID, blobberID, err)
	}

	return &term, nil
}

func (edb *EventDb) GetAllocationBlobberTerms(allocationID, blobberID string) ([]AllocationBlobberTerm, error) {
	var terms []AllocationBlobberTerm
	return terms, edb.Store.Get().Model(&AllocationBlobberTerm{}).
		Where(AllocationBlobberTerm{AllocationID: allocationID, BlobberID: blobberID}).
		Find(&terms).Error
}

func (edb *EventDb) deleteAllocationBlobberTerm(allocationID string, blobberID string) error {
	return edb.Store.Get().Model(&AllocationBlobberTerm{}).
		Where("allocation_id = ? and blobber_id = ?", allocationID, blobberID).
		Delete(&AllocationBlobberTerm{}).Error
}

func (edb *EventDb) deleteAllocationBlobberTerms(terms []AllocationBlobberTerm) error {
	for _, t := range terms {
		err := edb.deleteAllocationBlobberTerm(t.AllocationID, t.BlobberID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (edb *EventDb) updateAllocationBlobberTerm(term AllocationBlobberTerm) error {
	exists, err := term.exists(edb)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("failed to update term: AllocationID %s and blobberID %s does not exist",
			term.AllocationID, term.BlobberID)
	}
	return edb.Store.Get().Model(&AllocationBlobberTerm{}).
		Where("allocation_id = ? and blobber_id = ?", term.AllocationID, term.BlobberID).Updates(term).Error
}

func (edb *EventDb) updateAllocationBlobberTerms(terms []AllocationBlobberTerm) error {
	for _, t := range terms {
		err := edb.updateAllocationBlobberTerm(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (edb *EventDb) overwriteAllocationBlobberTerm(term AllocationBlobberTerm) error {
	result := edb.Store.Get().
		Model(&AllocationBlobberTerm{}).
		Where(&AllocationBlobberTerm{AllocationID: term.AllocationID, BlobberID: term.BlobberID}).
		Updates(map[string]interface{}{
			"read_price":                term.ReadPrice,
			"write_price":               term.WritePrice,
			"min_lock_demand":           term.MinLockDemand,
			"max_offer_duration":        term.MaxOfferDuration,
			"challenge_completion_time": term.ChallengeCompletionTime,
		})
	return result.Error
}

func (edb *EventDb) addOrOverwriteAllocationBlobberTerms(term []AllocationBlobberTerm) error {
	for _, t := range term {
		err := edb.addOrOverwriteAllocationBlobberTerm(t)
		if err != nil {
			return err
		}
	}

	return nil
}

func (edb *EventDb) addOrOverwriteAllocationBlobberTerm(term AllocationBlobberTerm) error {
	exists, err := term.exists(edb)
	if err != nil {
		return err
	}
	if exists {
		return edb.overwriteAllocationBlobberTerm(term)
	}

	return edb.Store.Get().Create(&term).Error
}

func (t *AllocationBlobberTerm) exists(edb *EventDb) (bool, error) {
	var term AllocationBlobberTerm
	if err := edb.Store.Get().Model(&AllocationBlobberTerm{}).
		Where("allocation_id = ? and blobber_id = ?", t.AllocationID, t.BlobberID).
		First(&term).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check term %v, error %v", t, err)
	}

	return true, nil
}
