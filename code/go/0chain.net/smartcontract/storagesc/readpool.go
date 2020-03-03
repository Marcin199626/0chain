package storagesc

import (
	"encoding/json"
	"time"

	chainState "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/state"
	"0chain.net/chaincore/tokenpool"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/util"
)

// lock request

type lockRequest struct {
	Duration time.Duration `json:"duration"`
}

func (lr *lockRequest) encode() (b []byte) {
	var err error
	if b, err = json.Marshal(lr); err != nil {
		panic(err) // must not happen
	}
	return
}

func (lr *lockRequest) decode(input []byte) error {
	return json.Unmarshal(input, lr)
}

// unlock request

type unlockRequest struct {
	PoolID datastore.Key `json:"pool_id"`
}

func (ur *unlockRequest) encode() (b []byte) {
	var err error
	if b, err = json.Marshal(ur); err != nil {
		panic(err) // must not happen
	}
	return
}

func (ur *unlockRequest) decode(input []byte) error {
	return json.Unmarshal(input, ur)
}

// read pool (a locked tokens for a duration)

type readPool struct {
	*tokenpool.ZcnLockingPool `json:"pool"`
}

func newReadPool() *readPool {
	return &readPool{ZcnLockingPool: &tokenpool.ZcnLockingPool{}}
}

func (rp *readPool) encode() (b []byte) {
	var err error
	if b, err = json.Marshal(rp); err != nil {
		panic(err) // must never happens
	}
	return
}

func (rp *readPool) decode(input []byte) (err error) {

	type readPoolJSON struct {
		Pool json.RawMessage `json:"pool"`
	}

	var readPoolVal readPoolJSON
	if err = json.Unmarshal(input, &readPoolVal); err != nil {
		return
	}

	if len(readPoolVal.Pool) == 0 {
		return // no data given
	}

	err = rp.ZcnLockingPool.Decode(readPoolVal.Pool, &tokenLock{})
	return
}

// readPools -- set of locked tokens for a duration

// readPools of a user
type readPools struct {
	ClientID datastore.Key               `json:"client_id"`
	Pools    map[datastore.Key]*readPool `json:"pools"`
}

func newReadPools(clientID datastore.Key) (rps *readPools) {
	rps = new(readPools)
	rps.ClientID = clientID
	rps.Pools = make(map[datastore.Key]*readPool)
	return
}

func (rps *readPools) Encode() (b []byte) {
	var err error
	if b, err = json.Marshal(rps); err != nil {
		panic(err) // must never happens
	}
	return
}

func (rps *readPools) Decode(input []byte) (err error) {
	var objMap map[string]json.RawMessage
	if err = json.Unmarshal(input, &objMap); err != nil {
		return
	}
	var cid, ok = objMap["client_id"]
	if ok {
		if err = json.Unmarshal(cid, &rsp.ClientID); err != nil {
			return err
		}
	}
	var p json.RawMessage
	if p, ok = objMap["pools"]; ok {
		var rawMessagesPools map[string]json.RawMessage
		if err = json.Unmarshal(p, &rawMessagesPools); err != nil {
			return
		}
		for _, raw := range rawMessagesPools {
			var tempPool = newReadPool()
			if err = tempPool.decode(raw); err != nil {
				return
			}
			rps.addPool(tempPool)
		}
	}
	return
}

func (rps *readPools) GetHash() string {
	return util.ToHex(rps.GetHashBytes())
}

func (rps *readPools) GetHashBytes() []byte {
	return encryption.RawHash(rps.Encode())
}

func readPoolsKey(scKey, clientID string) datastore.Key {
	return datastore.Key(scKey + ":readpool:" + clientID)
}

func (rps *readPools) getKey(scKey string) datastore.Key {
	return readPoolsKey(scKey, rps.ClientID)
}

func (rps *readPools) addPool(rp *readPool) (err error) {
	if rps.hasPool(rp.ID) {
		return errors.New("user already has read pool")
	}
	rps.Pools[rp.ID] = rp
	return
}

func (rps *readPools) delPool(id datastore.Key) {
	delete(rps.Pools, id)
}

// stat

type readPoolStats struct {
	Stats []*readPoolStat `json:"stats"`
}

func (stats *readPoolStats) encode() (b []byte) {
	var err error
	if b, err = json.Marshal(stats); err != nil {
		panic(err) // must never happens
	}
	return
}

func (stats *readPoolStats) decode(input []byte) error {
	return json.Unmarshal(input, stats)
}

func (stats *readPoolStats) addStat(stat *readPoolStat) {
	stats.Stats = append(stats.Stats, stat)
}

type readPoolStat struct {
	ID        datastore.Key    `json:"pool_id"`
	StartTime common.Timestamp `json:"start_time"`
	Duartion  time.Duration    `json:"duration"`
	TimeLeft  time.Duration    `json:"time_left"`
	Locked    bool             `json:"locked"`
	Balance   state.Balance    `json:"balance"`
}

func (stat *readPoolStat) encode() (b []byte) {
	var err error
	if b, err = json.Marshal(stat); err != nil {
		panic(err) // must never happens
	}
	return
}

func (stat *readPoolStat) decode(input []byte) error {
	return json.Unmarshal(input, stat)
}

type tokenLock struct {
	StartTime common.Timestamp `json:"start_time"`
	Duration  time.Duration    `json:"duration"`
	Owner     datastore.Key    `json:"owner"`
}

func (tl tokenLock) IsLocked(entity interface{}) bool {
	if tm, ok := entity.(time.Time); ok {
		return tm.Sub(common.ToTime(tl.StartTime)) < tl.Duration
	}
	return true
}

func (tl tokenLock) LockStats(entity interface{}) []byte {
	if tm, ok := entity.(time.Time); ok {
		var stat readPoolStat
		stat.StartTime = tl.StartTime
		stat.Duartion = tl.Duration
		stat.TimeLeft = (tl.Duration - tm.Sub(common.ToTime(tl.StartTime)))
		stat.Locked = tl.IsLocked(tm)
		return stat.encode()
	}
	return nil
}

//
// smart contract methods
//

// getReadPoolsBytes of a client
func (ssc *StorageSmartContract) getReadPoolsBytes(t *transaction.Transaction,
	balances chainState.StateContextI) ([]byte, error) {

	return balances.GetTrieNode(readPoolsKey(ssc.ID, t.ClientID))
}

// getReadPools of current client
func (ssc *StorageSmartContract) getReadPools(t *transaction.Transaction,
	balances chainState.StateContextI) (rps *readPools, err error) {

	var poolb []byte
	if poolb, err = ssc.getReadPoolsBytes(t, balances); err != nil {
		return
	}
	rps = new(readPools)
	err = rps.Decode(poolb)
	return
}

// newReadPool SC function creates new read pool for a client.
func (ssc *StorageSmartContract) newReadPool(t *transaction.Transaction,
	input []byte, balances chainState.StateContextI) (resp string, err error) {

	_, err = ssc.getReadPoolsBytes(t, balances)

	if err != nil && err != util.ErrValueNotPresent {
		return "", common.NewError("new_read_pool_failed", err.Error())
	}

	if err == nil {
		return "", common.NewError("new_read_pool_failed", "already exist")
	}

	var rps = newReadPools(t.ClientID)
	if _, err = balances.InsertTrieNode(rps.getKey(scKey), rps); err != nil {
		return "", common.NewError("new_read_pool_failed", err.Error())
	}

	return string(rps.Encode()), nil
}

// lock tokens for read pool of transaction's client
func (ssc *StorageSmartContract) readPoolLock(t *transaction.Transaction,
	input []byte, balances chainState.StateContextI) (resp string, err error) {

	// user read pools

	var rps *readPools
	if rps, err = ssc.getReadPools(t, balances); err != nil {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}

	// lock request & user balance

	var lr lockRequest
	if err = lr.decode(inpu); err != nil {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}
	var balance state.Balance
	balance, err = balances.GetClientBalance(t.ClientID)

	if err != nil && err != util.ErrValueNotPresent {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}

	if err == util.ErrValueNotPresent {
		return "", common.NewError("read_pool_lock_failed", "no tokens to lock")
	}

	if state.Balance(t.Value) > balance {
		return "", common.NewError("read_pool_lock_failed",
			"lock amount is greater than balance")
	}

	if lp.Duration <= 0 {
		return "", common.NewError("read_pool_lock_failed",
			"invalid locking period")
	}

	// lock

	var rp = newReadPool()
	rp.TokenLockInterface = &tokenLock{
		StartTime: t.CreationDate,
		Duration:  lr.Duration,
		Owner:     t.ClientID,
	}

	var transfer *state.Transfer
	if transfer, resp, err = rp.DigPool(t.Hash, t); err != nil {
		return "", common.NewError("read_pool_lock_failed",
			err.Error())
	}

	if err = balances.AddTransfer(transfer); err != nil {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}

	if err = rps.addPool(rp); err != nil {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}

	_, err = balances.InsertTrieNode(rps.getKey(ssc.ID), rps)
	if err != nil {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}

	return
}

// unlock tokens if expired
func (ssc *StorageSmartContract) readPoolUnlock(t *transaction.Transaction,
	input []byte, balances chainState.StateContextI) (resp string, err error) {

	// user read pools

	var rps *readPools
	if rps, err = ssc.getReadPools(t, balances); err != nil {
		return "", common.NewError("read_pool_lock_failed", err.Error())
	}

	// the request

	var (
		transfer *state.Transfer
		req      unlockRequest
	)

	if err = req.decode(input); err != nil {
		return "", common.NewError("read_pool_unlock_failed", err.Error())
	}

	var pool, ok = rps.Pools[req.PoolID]
	if !ok {
		return "", common.NewError("read_pool_unlock_failed", "pool not found")
	}

	transfer, resp, err = pool.EmptyPool(pool.ID, t.ClientID,
		common.ToTime(t.CreationDate))
	if err != nil {
		return "", common.NewError("read_pool_unlock_failed", err.Error())
	}
	rps.delPool(pool.ID)
	if err = balances.AddTransfer(transfer); err != nil {
		return "", common.NewError("read_pool_unlock_failed", err.Error())
	}

	// save pools
	if _, err = balances.InsertTrieNode(rps.getKey(ssc.ID), rps); err != nil {
		return "", common.NewError("read_pool_unlock_failed", err.Error())
	}

	return
}
