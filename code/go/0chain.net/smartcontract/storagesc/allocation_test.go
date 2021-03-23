package storagesc

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	chainState "0chain.net/chaincore/chain/state"
	sci "0chain.net/chaincore/smartcontractinterface"
	"0chain.net/chaincore/state"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// use:
//
//      go test -cover -coverprofile=cover.out && go tool cover -html=cover.out -o=cover.html
//
// to test and generate coverage html page
//

const (
	client1 = "client1"
)

func TestStorageSmartContract_getAllocation(t *testing.T) {
	const allocID, clientID, clientPk = "alloc_hex", "client_hex", "pk"
	var (
		ssc      = newTestStorageSC()
		balances = newTestBalances(t, false)
		alloc    *StorageAllocation
		err      error
	)
	if alloc, err = ssc.getAllocation(allocID, balances); err == nil {
		t.Fatal("missing error")
	}
	if err != util.ErrValueNotPresent {
		t.Fatal("unexpected error:", err)
	}
	alloc = new(StorageAllocation)
	alloc.ID = allocID
	alloc.DataShards = 1
	alloc.ParityShards = 1
	alloc.Size = 1024
	alloc.Expiration = 1050
	alloc.Owner = clientID
	alloc.OwnerPublicKey = clientPk
	_, err = balances.InsertTrieNode(alloc.GetKey(ssc.ID), alloc)
	require.NoError(t, err)
	var got *StorageAllocation
	got, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)
	assert.Equal(t, alloc.Encode(), got.Encode())
}

func TestStorageSmartContract_getAllocationsList(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		clientID string
		balances chainState.StateContextI
	}

	testSC := sci.SmartContract{
		ID:                          ADDRESS,
		RestHandlers:                map[string]sci.SmartContractRestHandler{},
		SmartContractExecutionStats: map[string]interface{}{},
	}

	var clientAlloc = ClientAllocation{
		ClientID: client1,
		Allocations: &Allocations{List: sortedList{
			"alloc1", "alloc2",
		}},
	}

	tb := &testBalances{
		balances: make(map[datastore.Key]state.Balance),
		tree:     make(map[datastore.Key]util.Serializable),
	}

	tb.InsertTrieNode(datastore.Key(ADDRESS+client1), &clientAlloc)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Allocations
		wantErr bool
	}{
		{
			name:   "empty balance",
			fields: fields{SmartContract: &testSC},
			args: args{
				clientID: client1,
				balances: newTestBalances(t, false),
			},
			want:    &Allocations{},
			wantErr: false,
		},
		{
			name:   "full balance",
			fields: fields{SmartContract: &testSC},
			args: args{
				clientID: client1,
				balances: tb,
			},
			want: &Allocations{List: sortedList{
				"alloc1", "alloc2",
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Logf("Testing <%v> function with <%v> case", "getAllocationsList", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			got, err := sc.getAllocationsList(tt.args.clientID, tt.args.balances)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllocationsList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAllocationsList() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorageSmartContract_getAllAllocationsList(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		balances chainState.StateContextI
	}
	testSC := sci.SmartContract{
		ID:                          ADDRESS,
		RestHandlers:                map[string]sci.SmartContractRestHandler{},
		SmartContractExecutionStats: map[string]interface{}{},
	}

	tb := &testBalances{
		balances: make(map[datastore.Key]state.Balance),
		tree:     make(map[datastore.Key]util.Serializable),
	}

	allocs := &Allocations{List: sortedList{
		"alloc1", "alloc2",
	}}

	tb.InsertTrieNode(datastore.Key(ALL_ALLOCATIONS_KEY), allocs)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Allocations
		wantErr bool
	}{
		{
			name:   "empty balance",
			fields: fields{SmartContract: &testSC},
			args: args{
				balances: newTestBalances(t, false),
			},
			want:    &Allocations{},
			wantErr: false,
		},
		{
			name:   "full balance",
			fields: fields{SmartContract: &testSC},
			args: args{
				balances: tb,
			},
			want: &Allocations{List: sortedList{
				"alloc1", "alloc2",
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing <%v> function with <%v> case", "getAllAllocationsList", tt.name)
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			got, err := sc.getAllAllocationsList(tt.args.balances)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllAllocationsList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAllAllocationsList() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorageSmartContract_addBlobbersOffers(t *testing.T) {
	const errMsg = "can't get blobber's stake pool: value not present"
	var (
		alloc    StorageAllocation
		b1, b2   StorageNode
		balances = newTestBalances(t, false)
		ssc      = newTestStorageSC()

		err error
	)
	// setup
	alloc.ID, b1.ID, b2.ID = "a1", "b1", "b2"
	alloc.ChallengeCompletionTime = 150 * time.Second
	alloc.Expiration = 100
	alloc.BlobberDetails = []*BlobberAllocation{
		&BlobberAllocation{Size: 20 * 1024, Terms: Terms{WritePrice: 12000}},
		&BlobberAllocation{Size: 20 * 1024, Terms: Terms{WritePrice: 4000}},
	}
	// stake pool not found
	var blobbers = []*StorageNode{&b1, &b2}
	requireErrMsg(t, ssc.addBlobbersOffers(&alloc, blobbers, balances), errMsg)
	// create stake pools
	for _, b := range blobbers {
		var sp = newStakePool()
		_, err = balances.InsertTrieNode(stakePoolKey(ssc.ID, b.ID), sp)
		require.NoError(t, err)
	}
	// add the offers
	require.NoError(t, ssc.addBlobbersOffers(&alloc, blobbers, balances))
	// check out all
	var sp1, sp2 *stakePool
	// stake pool 1
	sp1, err = ssc.getStakePool(b1.ID, balances)
	require.NoError(t, err)
	// offer 1
	var off1 = sp1.findOffer(alloc.ID)
	require.NotNil(t, off1)
	assert.Equal(t, toSeconds(alloc.ChallengeCompletionTime)+alloc.Expiration,
		off1.Expire)
	assert.Equal(t, state.Balance(sizeInGB(20*1024)*12000.0), off1.Lock)
	assert.Len(t, sp1.Offers, 1)
	// stake pool 2
	sp2, err = ssc.getStakePool(b2.ID, balances)
	require.NoError(t, err)
	// offer 2
	var off2 = sp2.findOffer(alloc.ID)
	require.NotNil(t, off1)
	assert.Equal(t, toSeconds(alloc.ChallengeCompletionTime)+alloc.Expiration,
		off2.Expire)
	assert.Equal(t, state.Balance(sizeInGB(20*1024)*4000.0), off2.Lock)
	assert.Len(t, sp2.Offers, 1)

}

func TestStorageSmartContract_newAllocationRequest(t *testing.T) {

	const (
		txHash, clientID, pubKey = "a5f4c3d2_tx_hex", "client_hex",
			"pub_key_hex"

		errMsg1 = "allocation_creation_failed: " +
			"No Blobbers registered. Failed to create a storage allocation"
		errMsg3 = "allocation_creation_failed: " +
			"Invalid client in the transaction. No client id in transaction"
		errMsg4 = "allocation_creation_failed: malformed request: " +
			"invalid character '}' looking for beginning of value"
		errMsg5 = "allocation_creation_failed: " +
			"invalid request: invalid read_price range"
		errMsg5p9 = "allocation_creation_failed: " +
			"invalid request: missing owner id"
		errMsg6 = "allocation_creation_failed: " +
			"Not enough blobbers to honor the allocation"
		errMsg7 = "allocation_creation_failed: " +
			"Not enough blobbers to honor the allocation"
		errMsg8 = "allocation_creation_failed: " +
			"not enough tokens to honor the min lock demand (0 < 270)"
		errMsg9 = "allocation_creation_failed: " +
			"no tokens to lock"
	)

	var (
		ssc      = newTestStorageSC()
		balances = newTestBalances(t, false)

		tx   transaction.Transaction
		conf *scConfig

		resp string
		err  error
	)

	tx.Hash = txHash
	tx.Value = 400
	tx.ClientID = clientID
	tx.CreationDate = toSeconds(2 * time.Hour)

	balances.setTransaction(t, &tx)

	conf = setConfig(t, balances)
	conf.MaxChallengeCompletionTime = 20 * time.Second
	conf.MinAllocDuration = 20 * time.Second
	conf.MinAllocSize = 20 * GB
	conf.TimeUnit = 2 * time.Minute

	_, err = balances.InsertTrieNode(scConfigKey(ssc.ID), conf)
	require.NoError(t, err)

	// 1.

	_, err = ssc.newAllocationRequest(&tx, nil, balances)
	requireErrMsg(t, err, errMsg1)

	// setup unhealthy blobbers
	var allBlobbers = newTestAllBlobbers()
	_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, allBlobbers)
	require.NoError(t, err)

	// 3.

	tx.ClientID = ""
	_, err = ssc.newAllocationRequest(&tx, nil, balances)
	requireErrMsg(t, err, errMsg3)

	// 4.

	tx.ClientID = clientID
	_, err = ssc.newAllocationRequest(&tx, []byte("} malformed {"), balances)
	requireErrMsg(t, err, errMsg4)

	// 5. invalid request

	var nar newAllocationRequest
	nar.ReadPriceRange = PriceRange{20, 10}

	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg5)

	// 6. missing owner id

	nar.Owner = clientID
	nar.ReadPriceRange = PriceRange{Min: 10, Max: 40}
	nar.WritePriceRange = PriceRange{Min: 100, Max: 400}
	nar.Size = 20 * GB
	nar.DataShards = 1
	nar.ParityShards = 1
	nar.Expiration = tx.CreationDate + toSeconds(48*time.Hour)
	nar.Owner = "" // not set
	nar.OwnerPublicKey = pubKey
	nar.PreferredBlobbers = nil                      // not set
	nar.MaxChallengeCompletionTime = 200 * time.Hour // max cct

	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg5p9)

	// 6 .filtered blobbers

	nar.Owner = clientID
	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg6)

	// 6. not enough blobbers (no health blobbers)

	nar.Expiration = tx.CreationDate + toSeconds(100*time.Second)

	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg6)

	// 7. missing stake pools (not enough blobbers)

	// make the blobbers health
	allBlobbers.Nodes[0].LastHealthCheck = tx.CreationDate
	allBlobbers.Nodes[1].LastHealthCheck = tx.CreationDate
	_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, allBlobbers)
	require.NoError(t, err)

	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg7)

	// 8. not enough tokens

	var (
		sp1, sp2 = newStakePool(), newStakePool()
		dp1, dp2 = new(delegatePool), new(delegatePool)
	)
	dp1.Balance, dp2.Balance = 20e10, 20e10
	sp1.Pools["hash1"], sp2.Pools["hash2"] = dp1, dp2
	require.NoError(t, sp1.save(ssc.ID, "b1", balances))
	require.NoError(t, sp2.save(ssc.ID, "b2", balances))

	tx.Value = 0
	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg8)

	// 9. no tokens to lock (client balance check)

	allBlobbers.Nodes[0].Used = 5 * GB
	allBlobbers.Nodes[1].Used = 10 * GB
	_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, allBlobbers)
	require.NoError(t, err)

	tx.Value = 400
	resp, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	requireErrMsg(t, err, errMsg9)

	// 10. ok

	allBlobbers.Nodes[0].Used = 5 * GB
	allBlobbers.Nodes[1].Used = 10 * GB
	_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, allBlobbers)
	require.NoError(t, err)

	balances.balances[clientID] = 1100

	tx.Value = 400
	resp, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	require.NoError(t, err)

	// check response
	var aresp StorageAllocation
	require.NoError(t, aresp.Decode([]byte(resp)))

	assert.Equal(t, txHash, aresp.ID)
	assert.Equal(t, 1, aresp.DataShards)
	assert.Equal(t, 1, aresp.ParityShards)
	assert.Equal(t, int64(20*GB), aresp.Size)
	assert.Equal(t, tx.CreationDate+100, aresp.Expiration)

	// expected blobbers after the allocation
	var sb = newTestAllBlobbers()
	sb.Nodes[0].LastHealthCheck = tx.CreationDate
	sb.Nodes[1].LastHealthCheck = tx.CreationDate
	sb.Nodes[0].Used += 10 * GB
	sb.Nodes[1].Used += 10 * GB

	// blobbers of the allocation
	assert.EqualValues(t, sb.Nodes, aresp.Blobbers)
	// blobbers saved in all blobbers list
	allBlobbers, err = ssc.getBlobbersList(balances)
	require.NoError(t, err)
	assert.EqualValues(t, sb.Nodes, allBlobbers.Nodes)
	// independent saved blobbers
	var b1, b2 *StorageNode
	b1, err = ssc.getBlobber("b1", balances)
	require.NoError(t, err)
	assert.EqualValues(t, sb.Nodes[0], b1)
	b2, err = ssc.getBlobber("b2", balances)
	require.NoError(t, err)
	assert.EqualValues(t, sb.Nodes[1], b2)

	assert.Equal(t, clientID, aresp.Owner)
	assert.Equal(t, pubKey, aresp.OwnerPublicKey)

	if assert.NotNil(t, aresp.Stats) {
		assert.Zero(t, *aresp.Stats)
	}

	assert.Nil(t, aresp.PreferredBlobbers)
	assert.Equal(t, PriceRange{10, 40}, aresp.ReadPriceRange)
	assert.Equal(t, PriceRange{100, 400}, aresp.WritePriceRange)
	assert.Equal(t, 15*time.Second, aresp.ChallengeCompletionTime) // max
	assert.Equal(t, tx.CreationDate, aresp.StartTime)
	assert.False(t, aresp.Finalized)

	// details
	var details = []*BlobberAllocation{
		&BlobberAllocation{
			BlobberID:     "b1",
			AllocationID:  txHash,
			Size:          10 * GB,
			Stats:         &StorageAllocationStats{},
			Terms:         sb.Nodes[0].Terms,
			MinLockDemand: 166, // (wp * (size/GB) * mld) / time_unit
			Spent:         0,
		},
		&BlobberAllocation{
			BlobberID:     "b2",
			AllocationID:  txHash,
			Size:          10 * GB,
			Stats:         &StorageAllocationStats{},
			Terms:         sb.Nodes[1].Terms,
			MinLockDemand: 104, // (wp * (size/GB) * mld) / time_unit
			Spent:         0,
		},
	}

	assert.EqualValues(t, details, aresp.BlobberDetails)

	// check out pools created and changed:
	//  - write pool, should be created and filled with value of transaction
	//  - stake pool, offer should be added
	//  - challenge pool, should be created

	// 1. write pool
	var wp *writePool
	wp, err = ssc.getWritePool(clientID, balances)
	require.NoError(t, err)
	assert.Equal(t, state.Balance(400), wp.allocUntil(aresp.ID, aresp.Until()))

	// 2. stake pool offers
	var expire = aresp.Until()

	sp1, err = ssc.getStakePool("b1", balances)
	require.NoError(t, err)
	assert.EqualValues(t, &offerPool{
		Lock:   10 * sb.Nodes[0].Terms.WritePrice,
		Expire: expire,
	}, sp1.Offers[aresp.ID])

	sp2, err = ssc.getStakePool("b2", balances)
	require.NoError(t, err)
	assert.EqualValues(t, &offerPool{
		Lock:   10 * sb.Nodes[1].Terms.WritePrice,
		Expire: expire,
	}, sp2.Offers[aresp.ID])

	// 3. challenge pool existence
	var cp *challengePool
	cp, err = ssc.getChallengePool(aresp.ID, balances)
	require.NoError(t, err)

	assert.Zero(t, cp.Balance)
}

func TestStorageSmartContract_updateAllocationRequest(t *testing.T) {

	var (
		ssc                  = newTestStorageSC()
		balances             = newTestBalances(t, false)
		client               = newClient(50*x10, balances)
		tp, exp        int64 = 100, 1000
		allocID, blobs       = addAllocation(t, ssc, client, tp, exp, 0,
			balances)

		alloc *StorageAllocation
		resp  string
		err   error
	)

	alloc, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)

	var cp = alloc.deepCopy(t)

	// change terms
	tp += 100
	for _, b := range blobs {
		var blob *StorageNode
		blob, err = ssc.getBlobber(b.id, balances)
		require.NoError(t, err)
		blob.Terms.WritePrice = state.Balance(1.8 * x10)
		blob.Terms.ReadPrice = state.Balance(0.8 * x10)
		_, err = updateBlobber(t, blob, 0, tp, ssc, balances)
		require.NoError(t, err)
	}

	//
	// extend
	//

	var uar updateAllocationRequest
	uar.ID = alloc.ID
	uar.Expiration = alloc.Expiration * 2
	uar.Size = alloc.Size * 2
	tp += 100
	resp, err = uar.callUpdateAllocReq(t, client.id, 20*x10, tp, ssc, balances)
	require.NoError(t, err)

	var deco StorageAllocation
	require.NoError(t, deco.Decode([]byte(resp)))

	alloc, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)

	require.EqualValues(t, alloc, &deco)

	assert.Equal(t, alloc.Size, cp.Size*3)
	assert.Equal(t, alloc.Expiration, cp.Expiration*3)

	var tbs, mld int64
	for _, d := range alloc.BlobberDetails {
		tbs += d.Size
		mld += int64(d.MinLockDemand)
	}
	var (
		numb  = int64(alloc.DataShards + alloc.ParityShards)
		bsize = (alloc.Size + (numb - 1)) / numb

		// expected min lock demand
		emld int64
	)
	for _, d := range alloc.BlobberDetails {
		emld += int64(
			sizeInGB(d.Size) * d.Terms.MinLockDemand *
				float64(d.Terms.WritePrice) *
				alloc.restDurationInTimeUnits(alloc.StartTime),
		)
	}

	assert.Equal(t, tbs, bsize*numb)
	assert.Equal(t, emld, mld)

	//
	// reduce
	//

	cp = alloc.deepCopy(t)

	uar.ID = alloc.ID
	uar.Expiration = -(alloc.Expiration / 2)
	uar.Size = -(alloc.Size / 2)

	tp += 100
	resp, err = uar.callUpdateAllocReq(t, client.id, 0, tp, ssc, balances)
	require.NoError(t, err)
	require.NoError(t, deco.Decode([]byte(resp)))

	alloc, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)

	require.EqualValues(t, alloc, &deco)

	assert.Equal(t, alloc.Size, cp.Size/2)
	assert.Equal(t, alloc.Expiration, cp.Expiration/2)

	tbs, mld = 0, 0
	for _, detail := range alloc.BlobberDetails {
		tbs += detail.Size
		mld += int64(detail.MinLockDemand)
	}
	numb = int64(alloc.DataShards + alloc.ParityShards)
	bsize = (alloc.Size + (numb - 1)) / numb
	assert.Equal(t, tbs, bsize*numb)
	// MLD can't be reduced
	assert.Equal(t, emld /*as it was*/, mld)

}

func TestStorageSmartContract_getAllocationBlobbers(t *testing.T) {
	const allocTxHash, clientID, pubKey = "a5f4c3d2_tx_hex", "client_hex",
		"pub_key_hex"

	var (
		ssc      = newTestStorageSC()
		balances = newTestBalances(t, false)

		alloc *StorageAllocation
		err   error
	)

	createNewTestAllocation(t, ssc, allocTxHash, clientID, pubKey, balances)

	alloc, err = ssc.getAllocation(allocTxHash, balances)
	require.NoError(t, err)

	var blobbers []*StorageNode
	blobbers, err = ssc.getAllocationBlobbers(alloc, balances)
	require.NoError(t, err)

	assert.Len(t, blobbers, 2)
}

func TestStorageSmartContract_closeAllocation(t *testing.T) {

	const (
		allocTxHash, clientID, pubKey, closeTxHash = "a5f4c3d2_tx_hex",
			"client_hex", "pub_key_hex", "close_tx_hash"

		errMsg1 = "allocation_closing_failed: " +
			"doesn't need to close allocation is about to expire"
		errMsg2 = "allocation_closing_failed: " +
			"doesn't need to close allocation is about to expire"
	)

	var (
		ssc      = newTestStorageSC()
		balances = newTestBalances(t, false)
		tx       transaction.Transaction

		alloc *StorageAllocation
		resp  string
		err   error
	)

	createNewTestAllocation(t, ssc, allocTxHash, clientID, pubKey, balances)

	tx.Hash = closeTxHash
	tx.ClientID = clientID
	tx.CreationDate = 1050

	alloc, err = ssc.getAllocation(allocTxHash, balances)
	require.NoError(t, err)

	// 1. expiring allocation
	alloc.Expiration = 1049
	_, err = ssc.closeAllocation(&tx, alloc, balances)
	requireErrMsg(t, err, errMsg1)

	// 2. close (all related pools has created)
	alloc.Expiration = tx.CreationDate +
		toSeconds(alloc.ChallengeCompletionTime) + 20
	resp, err = ssc.closeAllocation(&tx, alloc, balances)
	require.NoError(t, err)
	assert.NotZero(t, resp)

	// checking out

	alloc, err = ssc.getAllocation(alloc.ID, balances)
	require.NoError(t, err)

	require.Equal(t, tx.CreationDate, alloc.Expiration)

	var expire = alloc.Until()

	for _, detail := range alloc.BlobberDetails {
		var sp *stakePool
		sp, err = ssc.getStakePool(detail.BlobberID, balances)
		require.NoError(t, err)
		var offer = sp.findOffer(alloc.ID)
		require.NotNil(t, offer)
		assert.Equal(t, expire, offer.Expire)
	}
}

func Test_updateBlobbersInAll(t *testing.T) {
	var (
		all        StorageNodes
		balances   = newTestBalances(t, false)
		b1, b2, b3 StorageNode
		u1, u2     StorageNode
		decode     StorageNodes

		err error
	)

	b1.ID, b2.ID, b3.ID = "b1", "b2", "b3"
	b1.Capacity, b2.Capacity, b3.Capacity = 100, 100, 100

	all.Nodes = []*StorageNode{&b1, &b2, &b3}

	u1.ID, u2.ID = "b1", "b2"
	u1.Capacity, u2.Capacity = 200, 200

	err = updateBlobbersInAll(&all, []*StorageNode{&u1, &u2}, balances)
	require.NoError(t, err)

	var allSeri, ok = balances.tree[ALL_BLOBBERS_KEY]
	require.True(t, ok)
	require.NotNil(t, allSeri)
	require.NoError(t, decode.Decode(allSeri.Encode()))

	require.Len(t, decode.Nodes, 3)
	assert.Equal(t, "b1", decode.Nodes[0].ID)
	assert.Equal(t, int64(200), decode.Nodes[0].Capacity)
	assert.Equal(t, "b2", decode.Nodes[1].ID)
	assert.Equal(t, int64(200), decode.Nodes[1].Capacity)
	assert.Equal(t, "b3", decode.Nodes[2].ID)
	assert.Equal(t, int64(100), decode.Nodes[2].Capacity)
}

func Test_toSeconds(t *testing.T) {
	if toSeconds(time.Second*60+time.Millisecond*90) != 60 {
		t.Fatal("wrong")
	}
}

func Test_sizeInGB(t *testing.T) {
	if sizeInGB(12345*1024*1024*1024) != 12345.0 {
		t.Error("wrong")
	}
}

func newTestAllBlobbers() (all *StorageNodes) {
	all = new(StorageNodes)
	all.Nodes = []*StorageNode{
		&StorageNode{
			ID:      "b1",
			BaseURL: "http://blobber1.test.ru:9100/api",
			Terms: Terms{
				ReadPrice:               20,
				WritePrice:              200,
				MinLockDemand:           0.1,
				MaxOfferDuration:        200 * time.Second,
				ChallengeCompletionTime: 15 * time.Second,
			},
			Capacity:        20 * GB, // 20 GB
			Used:            5 * GB,  //  5 GB
			LastHealthCheck: 0,
		},
		&StorageNode{
			ID:      "b2",
			BaseURL: "http://blobber2.test.ru:9100/api",
			Terms: Terms{
				ReadPrice:               25,
				WritePrice:              250,
				MinLockDemand:           0.05,
				MaxOfferDuration:        250 * time.Second,
				ChallengeCompletionTime: 10 * time.Second,
			},
			Capacity:        20 * GB, // 20 GB
			Used:            10 * GB, // 10 GB
			LastHealthCheck: 0,
		},
	}
	return
}

func isEqualStrings(a, b []string) (eq bool) {
	if len(a) != len(b) {
		return
	}
	for i, ax := range a {
		if b[i] != ax {
			return false
		}
	}
	return true
}

// create allocation with blobbers, configurations, stake pools
func createNewTestAllocation(t *testing.T, ssc *StorageSmartContract,
	txHash, clientID, pubKey string, balances chainState.StateContextI) {

	var (
		tx          transaction.Transaction
		nar         newAllocationRequest
		allBlobbers *StorageNodes
		conf        scConfig
		err         error
	)

	tx.Hash = txHash
	tx.Value = 400
	tx.ClientID = clientID
	tx.CreationDate = toSeconds(2 * time.Hour)

	balances.(*testBalances).setTransaction(t, &tx)

	conf.MaxChallengeCompletionTime = 20 * time.Second
	conf.MinAllocDuration = 20 * time.Second
	conf.MinAllocSize = 20 * GB

	_, err = balances.InsertTrieNode(scConfigKey(ssc.ID), &conf)
	require.NoError(t, err)

	allBlobbers = newTestAllBlobbers()
	allBlobbers.Nodes[0].LastHealthCheck = tx.CreationDate
	allBlobbers.Nodes[1].LastHealthCheck = tx.CreationDate
	_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, allBlobbers)
	require.NoError(t, err)

	nar.ReadPriceRange = PriceRange{Min: 10, Max: 40}
	nar.WritePriceRange = PriceRange{Min: 100, Max: 400}
	nar.Size = 20 * GB
	nar.DataShards = 1
	nar.ParityShards = 1
	nar.Expiration = tx.CreationDate + toSeconds(48*time.Hour)
	nar.Owner = clientID
	nar.OwnerPublicKey = pubKey
	nar.PreferredBlobbers = nil                      // not set
	nar.MaxChallengeCompletionTime = 200 * time.Hour //

	nar.Expiration = tx.CreationDate + toSeconds(100*time.Second)

	var (
		sp1, sp2 = newStakePool(), newStakePool()
		dp1, dp2 = new(delegatePool), new(delegatePool)
	)
	dp1.Balance, dp2.Balance = 20e10, 20e10
	sp1.Pools["hash1"], sp2.Pools["hash2"] = dp1, dp2
	require.NoError(t, sp1.save(ssc.ID, "b1", balances))
	require.NoError(t, sp2.save(ssc.ID, "b2", balances))

	tx.Value = 400

	allBlobbers.Nodes[0].Used = 5 * GB
	allBlobbers.Nodes[1].Used = 10 * GB
	_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, allBlobbers)
	require.NoError(t, err)

	balances.(*testBalances).balances[clientID] = 1100

	tx.Value = 400
	_, err = ssc.newAllocationRequest(&tx, mustEncode(t, &nar), balances)
	require.NoError(t, err)
	return
}

func (alloc *StorageAllocation) deepCopy(t *testing.T) (cp *StorageAllocation) {
	cp = new(StorageAllocation)
	require.NoError(t, cp.Decode(mustEncode(t, alloc)))
	return
}

// add empty blobber challenges
func addBloberChallenges(t *testing.T, sscID string, alloc *StorageAllocation,
	balances *testBalances) {

	var err error
	for _, d := range alloc.BlobberDetails {
		var bc = new(BlobberChallenge)
		bc.BlobberID = d.BlobberID
		_, err = balances.InsertTrieNode(bc.GetKey(sscID), bc)
		require.NoError(t, err)
	}
}

// - finalize allocation
func Test_finalize_allocation(t *testing.T) {

	var (
		ssc            = newTestStorageSC()
		balances       = newTestBalances(t, false)
		client         = newClient(100*x10, balances)
		tp, exp  int64 = 0, int64(toSeconds(time.Hour))
		err      error
	)

	setConfig(t, balances)

	tp += 100
	var allocID, blobs = addAllocation(t, ssc, client, tp, exp, 0, balances)

	// blobbers: stake 10k, balance 40k

	var alloc *StorageAllocation
	alloc, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)

	var b1 *Client
	for _, b := range blobs {
		if b.id == alloc.BlobberDetails[0].BlobberID {
			b1 = b
			break
		}
	}
	require.NotNil(t, b1)

	// add 10 validators
	var valids []*Client
	tp += 100
	for i := 0; i < 10; i++ {
		valids = append(valids, addValidator(t, ssc, tp, balances))
	}

	// generate some challenges to fill challenge pool

	const allocRoot = "alloc-root-1"

	// write 100 MB
	tp += 100
	var cc = &BlobberCloseConnection{
		AllocationRoot:     allocRoot,
		PrevAllocationRoot: "",
		WriteMarker: &WriteMarker{
			AllocationRoot:         allocRoot,
			PreviousAllocationRoot: "",
			AllocationID:           allocID,
			Size:                   10 * 1024 * 1024, // 100 MB
			BlobberID:              b1.id,
			Timestamp:              common.Timestamp(tp),
			ClientID:               client.id,
		},
	}
	cc.WriteMarker.Signature, err = client.scheme.Sign(
		encryption.Hash(cc.WriteMarker.GetHashData()))
	require.NoError(t, err)

	// write
	tp += 100
	var tx = newTransaction(b1.id, ssc.ID, 0, tp)
	balances.setTransaction(t, tx)
	var resp string
	resp, err = ssc.commitBlobberConnection(tx, mustEncode(t, &cc),
		balances)
	require.NoError(t, err)
	require.NotZero(t, resp)

	// until the end
	alloc, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)

	// load validators
	var validators *ValidatorNodes
	validators, err = ssc.getValidatorsList(balances)
	require.NoError(t, err)

	// load blobber
	var blobber *StorageNode
	blobber, err = ssc.getBlobber(b1.id, balances)
	require.NoError(t, err)

	//
	var (
		step            = (int64(alloc.Expiration) - tp) / 10
		challID, prevID string
	)

	// expire the allocation challenging it (+ last challenge)
	for i := int64(0); i < 2; i++ {
		tp += step / 2

		challID = fmt.Sprintf("chall-%d", i)
		genChall(t, ssc, b1.id, tp, prevID, challID, i, validators.Nodes,
			alloc.ID, blobber, allocRoot, balances)

		var chall = new(ChallengeResponse)
		chall.ID = challID

		for _, val := range valids {
			chall.ValidationTickets = append(chall.ValidationTickets,
				val.validTicket(t, chall.ID, b1.id, true, tp))
		}

		tp += step / 2
		tx = newTransaction(b1.id, ssc.ID, 0, tp)
		balances.setTransaction(t, tx)
		var resp string
		resp, err = ssc.verifyChallenge(tx, mustEncode(t, chall), balances)
		require.NoError(t, err)
		require.NotZero(t, resp)

		// next stage
		prevID = challID
	}

	// balances
	var wp *writePool
	wp, err = ssc.getWritePool(client.id, balances)
	require.NoError(t, err)

	var cp *challengePool
	cp, err = ssc.getChallengePool(allocID, balances)
	require.NoError(t, err)

	var sp *stakePool
	sp, err = ssc.getStakePool(b1.id, balances)
	require.NoError(t, err)

	require.NotNil(t, sp.findOffer(allocID))

	// expire the allocation
	tp += int64(alloc.Until())

	// finalize it

	var req lockRequest
	req.AllocationID = allocID

	tx = newTransaction(client.id, ssc.ID, 0, tp)
	balances.setTransaction(t, tx)
	_, err = ssc.finalizeAllocation(tx, mustEncode(t, &req), balances)
	require.NoError(t, err)

	// check out all the balances

	// reload
	wp, err = ssc.getWritePool(client.id, balances)
	require.NoError(t, err)

	cp, err = ssc.getChallengePool(allocID, balances)
	require.NoError(t, err)

	sp, err = ssc.getStakePool(b1.id, balances)
	require.NoError(t, err)

	tp += int64(toSeconds(alloc.ChallengeCompletionTime))
	require.Nil(t, sp.findOffer(allocID), "should be removed")
	assert.Zero(t, cp.Balance, "should be drained")
	assert.Zero(t, wp.allocUntil(allocID, common.Timestamp(tp)),
		"should be drained")

	alloc, err = ssc.getAllocation(allocID, balances)
	require.NoError(t, err)

	assert.True(t, alloc.Finalized)
	assert.True(t,
		alloc.BlobberDetails[0].MinLockDemand <= alloc.BlobberDetails[0].Spent,
		"should receive min_lock_demand")
}

// user request allocation with preferred blobbers, but the blobbers
// doesn't exist in the SC (or didn't dens health check transaction
// last time becoming themselves unhealthy)
func Test_preferred_blobbers(t *testing.T) {

	var (
		ssc            = newTestStorageSC()
		balances       = newTestBalances(t, false)
		client         = newClient(100*x10, balances)
		tp, exp  int64 = 0, int64(toSeconds(time.Hour))
	)

	// add allocation we will not use, just the addAllocation creates blobbers
	// and adds them to SC; also the addAllocation sets SC configurations
	// (e.g. create allocation for side effects)
	tp += 100
	var _, blobs = addAllocation(t, ssc, client, tp, exp, 0, balances)

	// we need at least 4 blobbers to use them as preferred blobbers
	require.True(t, len(blobs) > 4)

	// allocation request to modify and create
	var getAllocRequest = func() (nar *newAllocationRequest) {
		nar = new(newAllocationRequest)
		nar.DataShards = 10
		nar.ParityShards = 10
		nar.Expiration = common.Timestamp(exp)
		nar.Owner = client.id
		nar.OwnerPublicKey = client.pk
		nar.ReadPriceRange = PriceRange{1 * x10, 10 * x10}
		nar.WritePriceRange = PriceRange{2 * x10, 20 * x10}
		nar.Size = 2 * GB // 2 GB
		nar.MaxChallengeCompletionTime = 200 * time.Hour
		return
	}

	var newAlloc = func(t *testing.T, nar *newAllocationRequest) string {
		t.Helper()
		// call SC function
		tp += 100
		var resp, err = nar.callNewAllocReq(t, client.id, 15*x10, ssc, tp,
			balances)
		require.NoError(t, err)
		// decode response to get allocation ID
		var deco StorageAllocation
		require.NoError(t, deco.Decode([]byte(resp)))
		return deco.ID
	}

	// preferred blobbers alive (just choose n-th first)
	var getPreferredBlobbers = func(blobs []*Client, n int) (pb []string) {
		require.True(t, n <= len(blobs),
			"invalid test, not enough blobbers to choose preferred")
		pb = make([]string, 0, n)
		for i := 0; i < n; i++ {
			pb = append(pb, getBlobberURL(blobs[i].id))
		}
		return
	}

	// create allocation with preferred blobbers list
	t.Run("preferred blobbers", func(t *testing.T) {
		var (
			nar = getAllocRequest()
			pbl = getPreferredBlobbers(blobs, 4)
		)
		nar.PreferredBlobbers = pbl
		var (
			allocID    = newAlloc(t, nar)
			alloc, err = ssc.getAllocation(allocID, balances)
		)
		require.NoError(t, err)
	Preferred:
		for _, url := range pbl {
			var id = blobberIDByURL(url)
			for _, d := range alloc.BlobberDetails {
				if id == d.BlobberID {
					continue Preferred // ok
				}
			}
			t.Error("missing preferred blobber in allocation blobbers")
		}
	})

	t.Run("no preferred blobbers", func(t *testing.T) {

		var getBlobbersNotExists = func(n int) (bns []string) {
			bns = make([]string, 0, n)
			for i := 0; i < n; i++ {
				bns = append(bns, newClient(0, balances).id)
			}
			return
		}

		var (
			nar = getAllocRequest()
			pbl = getBlobbersNotExists(4)
			err error
		)
		nar.PreferredBlobbers = pbl
		tp += 100
		_, err = nar.callNewAllocReq(t, client.id, 15*x10, ssc, tp, balances)
		require.Error(t, err) // expected error
	})

	t.Run("unhealthy preferred blobbers", func(t *testing.T) {

		var updateBlobber = func(t *testing.T, b *StorageNode) {
			t.Helper()
			var all, err = ssc.getBlobbersList(balances)
			require.NoError(t, err)
			all.Nodes.update(b)
			_, err = balances.InsertTrieNode(ALL_BLOBBERS_KEY, all)
			require.NoError(t, err)
			_, err = balances.InsertTrieNode(b.GetKey(ssc.ID), b)
			require.NoError(t, err)
		}

		// revoke health check from preferred blobbers
		var (
			nar = getAllocRequest()
			pbl = getPreferredBlobbers(blobs, 4)
			err error
		)

		// after hour all blobbers become unhealthy
		tp += int64(toSeconds(time.Hour))
		for _, bx := range blobs {
			var b *StorageNode
			b, err = ssc.getBlobber(bx.id, balances)
			require.NoError(t, err)
			b.LastHealthCheck = common.Timestamp(tp) // do nothing to test the test
			updateBlobber(t, b)
		}

		// make the preferred blobbers unhealthy
		for _, url := range pbl {
			var b *StorageNode
			b, err = ssc.getBlobber(blobberIDByURL(url), balances)
			require.NoError(t, err)
			b.LastHealthCheck = 0
			updateBlobber(t, b)
		}

		nar.Expiration += common.Timestamp(tp)
		nar.PreferredBlobbers = pbl
		_, err = nar.callNewAllocReq(t, client.id, 15*x10, ssc, tp, balances)
		require.Error(t, err) // expected error
	})

}

func TestStorageSmartContract_addAllocation(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		alloc    *StorageAllocation
		balances chainState.StateContextI
		scId     string
	}
	type test struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
		before  func(*test)
	}
	tests := []test{
		{
			name:   "error: Failed to get allocation list by owner",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				alloc:    &StorageAllocation{},
				balances: newTestBalances(t, true),
			},
			want:    "",
			wantErr: true,
			before: func(t *test) {
				key := datastore.Key(t.args.scId + t.args.alloc.Owner)
				t.args.balances.InsertTrieNode(key, &fakeSerializable{})
			},
		},
		{
			name:   "error: Failed to get all allocation list by id",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				alloc:    &StorageAllocation{},
				balances: newTestBalances(t, true),
			},
			want:    "",
			wantErr: true,
			before: func(t *test) {
				t.args.balances.InsertTrieNode(ALL_ALLOCATIONS_KEY, &fakeSerializable{})
			},
		},
		{
			name:   "error: allocation id already used in trie",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				alloc:    &StorageAllocation{},
				balances: newTestBalances(t, true),
			},
			want:    "",
			wantErr: true,
			before: func(t *test) {
				key := t.args.alloc.GetKey(t.args.scId)
				t.args.balances.InsertTrieNode(key, &StorageAllocation{})
			},
		},
		{
			name:   "ok",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				alloc:    &StorageAllocation{},
				balances: newTestBalances(t, true),
			},
			want:    "",
			wantErr: false,
			before: func(t *test) {
				t.want = string(t.args.alloc.Encode())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			tt.args.scId = sc.ID
			if tt.before != nil {
				tt.before(&tt)
			}
			got, err := sc.addAllocation(tt.args.alloc, tt.args.balances)
			if (err != nil) != tt.wantErr {
				t.Errorf("addAllocation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("addAllocation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorageSmartContract_extendAllocation(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		t        *transaction.Transaction
		all      *StorageNodes
		alloc    *StorageAllocation
		blobbers []*StorageNode
		uar      *updateAllocationRequest
		balances chainState.StateContextI
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantResp string
		wantErr  bool
	}{
		{
			name:   "Error: blobber no longer provides its service",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Capacity: 0,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       0,
					Expiration: 0,
				},
				balances: newTestBalances(t, false),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: blobber doesn't have enough free space",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Capacity: 10,
						Used:     10,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: newTestBalances(t, false),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: blobber  doesn't allow so long offers",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 0},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: common.Now(),
				},
				balances: newTestBalances(t, false),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  invalid character",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID: "blobber_1",
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:stakepool:blobber_1", &fakeSerializable{})
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  value not present",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID: "blobber_1",
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: newTestBalances(t, false),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed   missing offer pool",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID: "blobber_1",
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"blobber_1": &offerPool{},
						},
					})
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  can't get write pool:value not present",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					ID:           "alloc_1",
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID: "blobber_1",
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,

					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  no tokens to lock",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					ID:           "alloc_1",
					Owner:        "owner_1",
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID: "blobber_1",
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:writepool:owner_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  lock amount is greater than balance",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 1000, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					ID:           "alloc_1",
					Owner:        "owner_1",
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID: "blobber_1",
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:writepool:owner_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.setBalance("client1", 100)
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  invalid transfer of state",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					ID:           "alloc_1",
					Owner:        "owner_1",
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID:     "blobber_1",
							MinLockDemand: 2,
							Spent:         1,
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:writepool:owner_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.setBalance("client1", 10000)
					b.txn = newTransaction("client10", "client20", 10, 0)
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  value not present",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					ID:           "alloc_1",
					Owner:        "owner_1",
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID:     "blobber_1",
							MinLockDemand: 2,
							Spent:         1,
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:writepool:owner_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.setBalance("client1", 10000)
					b.txn = newTransaction("client1", "client2", 10, 0)
					return b
				}(),
			},
			wantResp: "",
			wantErr:  true,
		},
		{
			name:   "Error: allocation_extending_failed  value not present",
			fields: fields{SmartContract: sci.NewSC("sci")},
			args: args{
				t:   newTransaction("client1", "client2", 10, 0),
				all: &StorageNodes{},
				alloc: &StorageAllocation{
					ID:           "alloc_1",
					Owner:        "owner_1",
					DataShards:   5,
					ParityShards: 5,
					BlobberDetails: []*BlobberAllocation{
						&BlobberAllocation{
							BlobberID:     "blobber_1",
							MinLockDemand: 2,
							Spent:         1,
							Size: 0,
						},
					},
				},
				blobbers: []*StorageNode{
					&StorageNode{
						Terms:    Terms{MaxOfferDuration: 100 * time.Second},
						Capacity: 10,
						Used:     1,
					},
				},
				uar: &updateAllocationRequest{
					ID:         "",
					OwnerID:    "",
					Size:       10,
					Expiration: 0,
				},
				balances: func() *testBalances {
					b := newTestBalances(t, false)
					b.InsertTrieNode("sci:writepool:owner_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.InsertTrieNode("sci:stakepool:blobber_1", &stakePool{
						Offers: map[string]*offerPool{
							"alloc_1": &offerPool{},
						},
					})
					b.setBalance("client1", 10000)
					b.txn = newTransaction("client1", "client2", 10, 0)

					b.InsertTrieNode("sci:challengepool:alloc_1", &challengePool{})
					return b
				}(),
			},
			wantResp: string((&StorageAllocation{
				ID:                      "alloc_1",
				Owner:                   "owner_1",
				DataShards:              5,
				ParityShards:            5,
				Size:                    10,
				ChallengeCompletionTime: 0,
				BlobberDetails: []*BlobberAllocation{
					&BlobberAllocation{
						BlobberID:     "blobber_1",
						MinLockDemand: 2,
						Spent:         1,
						Size: 1,
						Terms: weightedAverage(
							&Terms{MaxOfferDuration: 100 * time.Second},
							&Terms{MaxOfferDuration: 100 * time.Second},
							common.Timestamp(0), common.Timestamp(0), common.Timestamp(0),0,1),
					},
				},
			}).Encode()),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			gotResp, err := sc.extendAllocation(tt.args.t, tt.args.all, tt.args.alloc, tt.args.blobbers, tt.args.uar, tt.args.balances)
			if (err != nil) != tt.wantErr {
				t.Errorf("extendAllocation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotResp != tt.wantResp {
				t.Errorf("extendAllocation() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

// @TODO implement test cases
func TestStorageSmartContract_adjustChallengePool(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		alloc    *StorageAllocation
		wp       *writePool
		odr      common.Timestamp
		ndr      common.Timestamp
		oterms   []Terms
		now      common.Timestamp
		balances chainState.StateContextI
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{

		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			if err := sc.adjustChallengePool(tt.args.alloc, tt.args.wp, tt.args.odr, tt.args.ndr, tt.args.oterms, tt.args.now, tt.args.balances); (err != nil) != tt.wantErr {
				t.Errorf("adjustChallengePool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// @TODO implement test cases
func TestStorageSmartContract_saveUpdatedAllocation(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		all      *StorageNodes
		alloc    *StorageAllocation
		blobbers []*StorageNode
		balances chainState.StateContextI
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			if err := sc.saveUpdatedAllocation(tt.args.all, tt.args.alloc, tt.args.blobbers, tt.args.balances); (err != nil) != tt.wantErr {
				t.Errorf("saveUpdatedAllocation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// @TODO implement test cases
func TestStorageSmartContract_filterBlobbersByFreeSpace(t *testing.T) {
	type fields struct {
		SmartContract *sci.SmartContract
	}
	type args struct {
		now      common.Timestamp
		size     int64
		balances chainState.StateContextI
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantFilter filterBlobberFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &StorageSmartContract{
				SmartContract: tt.fields.SmartContract,
			}
			if gotFilter := sc.filterBlobbersByFreeSpace(tt.args.now, tt.args.size, tt.args.balances); !reflect.DeepEqual(gotFilter, tt.wantFilter) {
				t.Errorf("filterBlobbersByFreeSpace() = %v, want %v", gotFilter, tt.wantFilter)
			}
		})
	}
}

//func TestStorageSmartContract_reduceAllocation(t *testing.T) {
//	type fields struct {
//		SmartContract *sci.SmartContract
//	}
//	type args struct {
//		t        *transaction.Transaction
//		all      *StorageNodes
//		alloc    *StorageAllocation
//		blobbers []*StorageNode
//		uar      *updateAllocationRequest
//		balances chainState.StateContextI
//	}
//	type test struct {
//		name     string
//		fields   fields
//		args     args
//		wantResp string
//		wantErr  bool
//	}
//	tests := []test{
//		{
//			name:   "",
//			fields: fields{SmartContract: sci.NewSC("sci")},
//			args: args{
//				t:     newTransaction(bId1, bId2, 10, 0),
//				all:   &StorageNodes{},
//				alloc: &StorageAllocation{},
//				blobbers: []*StorageNode{
//
//				},
//				uar: &updateAllocationRequest{
//					ID:         "",
//					OwnerID:    "",
//					Size:       0,
//					Expiration: 0,
//				},
//				balances: newTestBalances(t, false),
//			},
//			wantResp: "",
//			wantErr:  false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			sc := &StorageSmartContract{
//				SmartContract: tt.fields.SmartContract,
//			}
//			gotResp, err := sc.reduceAllocation(tt.args.t, tt.args.all, tt.args.alloc, tt.args.blobbers, tt.args.uar, tt.args.balances)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("reduceAllocation() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if gotResp != tt.wantResp {
//				t.Errorf("reduceAllocation() gotResp = %v, want %v", gotResp, tt.wantResp)
//			}
//		})
//	}
//}
