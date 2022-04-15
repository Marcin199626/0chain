//go:build !integration_tests
// +build !integration_tests

// todo: it's a legacy ugly approach; refactor later

package storagesc

import (
	"encoding/json"
	"errors"
	"fmt"

	cstate "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	"0chain.net/core/util"
	"0chain.net/smartcontract/dbs"
	"0chain.net/smartcontract/dbs/event"
	"0chain.net/smartcontract/stakepool/spenum"
)

const (
	insertBlobberErrCode = "insert_blobber"
)

// insert new blobber, filling its stake pool
func (sc *StorageSmartContract) insertBlobber(t *transaction.Transaction,
	conf *Config, blobber *StorageNode,
	balances cstate.StateContextI,
) (err error) {

	storedBlobber := &StorageNode{}
	err = balances.GetTrieNode(blobber.GetKey(sc.ID), storedBlobber)
	switch {
	case err != nil && !errors.Is(err, util.ErrValueNotPresent):
		return fmt.Errorf("can't insert blobber: %w", err)

	case err == nil:
		if err = sc.updateBlobber(t, conf, blobber, balances); err != nil {
			return common.NewErrorf(insertBlobberErrCode, "can't update blobber: %w", err)
		}

		// if blobber was removed from partitions at the last update and need to add it now
		if storedBlobber.Capacity <= 0 && blobber.Capacity > 0 {
			if err := addBlobberToPartitions(blobber, balances); err != nil {
				return common.NewErrorf(insertBlobberErrCode, "can't update blobber on partitions: %w", err)
			}
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

	data, _ := json.Marshal(dbs.DbUpdates{
		Id: t.ClientID,
		Updates: map[string]interface{}{
			"total_stake": int64(sp.stake()),
		},
	})
	balances.EmitEvent(event.TypeStats, event.TagUpdateBlobber, t.ClientID, string(data))

	// update the list
	if err := addBlobberToPartitions(blobber, balances); err != nil {
		return fmt.Errorf("can't add blobber to partitions: %w", err)
	}

	// update statistic
	sc.statIncr(statAddBlobber)
	sc.statIncr(statNumberOfBlobbers)
	return
}
