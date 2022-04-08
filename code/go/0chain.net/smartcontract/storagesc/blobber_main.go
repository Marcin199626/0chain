//go:build !integration_tests
// +build !integration_tests

// todo: it's a legacy ugly approach; refactor later

package storagesc

import (
	"errors"
	"fmt"

	"0chain.net/core/util"
	"0chain.net/smartcontract/partitions"
	"0chain.net/smartcontract/stakepool/spenum"

	cstate "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/transaction"
)

// insert new blobber, filling its stake pool
func (sc *StorageSmartContract) insertBlobber(t *transaction.Transaction,
	conf *Config, blobber *StorageNode,
	balances cstate.StateContextI,
) (err error) {

	blobbers, err := getBlobbersPartitions(balances)
	if err != nil {
		return err // todo
	}

	err = balances.GetTrieNode(blobber.GetKey(sc.ID), &StorageNode{})
	switch {
	case err != nil && !errors.Is(err, util.ErrValueNotPresent):
		return err // todo

	case err == nil:
		err = sc.updateBlobber(t, conf, blobber, balances)
		if err != nil {
			return err // todo
		}
		if err = blobbers.Save(balances); err != nil {
			return err // todo
		}

		return nil
	}

	// check params
	if err = blobber.validate(conf); err != nil {
		return fmt.Errorf("invalid blobber params: %v", err)
	}

	blobber.LastHealthCheck = t.CreationDate // set to now

	// create stake pool
	var sp *stakePool
	sp, err = sc.getOrUpdateStakePool(conf, blobber.ID, spenum.Blobber,
		blobber.StakePoolSettings, balances)
	if err != nil {
		return fmt.Errorf("creating stake pool: %v", err)
	}

	if err = sp.save(sc.ID, t.ClientID, balances); err != nil {
		return fmt.Errorf("saving stake pool: %v", err)
	}

	// update the list
	partItem := &partitions.BlobberNode{
		ID:  blobber.ID,
		Url: blobber.BaseURL,
	}
	if _, err = blobbers.Add(partItem, balances); err != nil {
		return err // todo
	}
	if err = blobbers.Save(balances); err != nil {
		return err // todo
	}

	if err := emitAddOrOverwriteBlobber(blobber, sp, balances); err != nil {
		return fmt.Errorf("emmiting blobber %v: %v", blobber, err)
	}

	// update statistic
	sc.statIncr(statAddBlobber)
	sc.statIncr(statNumberOfBlobbers)
	return
}
