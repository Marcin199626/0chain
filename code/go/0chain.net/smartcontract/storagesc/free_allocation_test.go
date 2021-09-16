package storagesc_test

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	cstate "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/mocks"
	sci "0chain.net/chaincore/smartcontractinterface"
	"0chain.net/chaincore/state"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	"0chain.net/core/encryption"
	"0chain.net/core/util"
	. "0chain.net/smartcontract/storagesc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const blobberHealthTime = 60 * 60

func TestAddFreeStorageAssigner(t *testing.T) {
	const (
		mockCooperationId        = "mock cooperation id"
		mockPublicKey            = "mock public key"
		mockAnotherPublicKey     = "another mock public key"
		mockIndividualTokenLimit = 20
		mockTotalTokenLimit      = 3000
		mockNotOwner             = "mock not owner"
	)

	type args struct {
		ssc      *StorageSmartContract
		txn      *transaction.Transaction
		input    []byte
		balances cstate.StateContextI
	}
	type want struct {
		err    bool
		errMsg string
	}
	type parameters struct {
		clientId string
		info     NewFreeStorageAssignerInfo
		exists   bool
		existing FreeStorageAssigner
	}
	var conf = &ScConfig{
		MaxIndividualFreeAllocation: zcnToBalance(mockIndividualTokenLimit),
		MaxTotalFreeAllocation:      zcnToBalance(mockTotalTokenLimit),
	}

	setExpectations := func(t *testing.T, name string, p parameters, want want) args {
		var balances = &mocks.StateContextI{}
		var txn = &transaction.Transaction{
			ClientID: p.clientId,
		}
		var ssc = &StorageSmartContract{
			SmartContract: sci.NewSC(ADDRESS),
		}
		input, err := json.Marshal(p.info)
		require.NoError(t, err)

		balances.On("GetTrieNode", ScConfigKey(ssc.ID)).Return(conf, nil).Once()

		//var newRedeemed []freeStorageRedeemed
		if p.exists {
			balances.On(
				"GetTrieNode",
				FreeStorageAssignerKey(ssc.ID, p.info.Name),
			).Return(FsaToFsa(&p.existing), nil).Once()
		} else {
			balances.On(
				"GetTrieNode", FreeStorageAssignerKey(ssc.ID, p.info.Name),
			).Return(nil, util.ErrValueNotPresent).Once()
		}

		balances.On("InsertTrieNode", FreeStorageAssignerKey(ssc.ID, p.info.Name),
			FsaToFsa(&FreeStorageAssigner{
				ClientId:           p.info.Name,
				PublicKey:          p.info.PublicKey,
				IndividualLimit:    zcnToBalance(p.info.IndividualLimit),
				TotalLimit:         zcnToBalance(p.info.TotalLimit),
				CurrentRedeemed:    p.existing.CurrentRedeemed,
				RedeemedTimestamps: p.existing.RedeemedTimestamps,
			})).Return("", nil).Once()

		return args{ssc, txn, input, balances}
	}

	testCases := []struct {
		name       string
		parameters parameters
		want       want
	}{
		{
			name: "ok_new",
			parameters: parameters{
				clientId: owner,
				info: NewFreeStorageAssignerInfo{
					Name:            mockCooperationId + "ok_new",
					PublicKey:       mockPublicKey,
					IndividualLimit: mockIndividualTokenLimit,
					TotalLimit:      mockTotalTokenLimit,
				},
				exists: false,
			},
		},
		{
			name: "ok_existing",
			parameters: parameters{
				clientId: owner,
				info: NewFreeStorageAssignerInfo{
					Name:            mockCooperationId + "ok_existing",
					PublicKey:       mockPublicKey,
					IndividualLimit: mockIndividualTokenLimit,
					TotalLimit:      mockTotalTokenLimit,
				},
				exists: true,
				existing: FreeStorageAssigner{
					ClientId:           mockCooperationId + "ok_existing",
					PublicKey:          mockAnotherPublicKey,
					IndividualLimit:    mockIndividualTokenLimit / 2,
					TotalLimit:         mockTotalTokenLimit / 2,
					CurrentRedeemed:    mockTotalTokenLimit / 4,
					RedeemedTimestamps: []common.Timestamp{20, 30, 50, 70, 110, 130, 170},
				},
			},
		},
		{
			name: "not_owner",
			parameters: parameters{
				clientId: mockNotOwner,
				info: NewFreeStorageAssignerInfo{
					Name:            mockCooperationId + "ok_new",
					PublicKey:       mockPublicKey,
					IndividualLimit: mockIndividualTokenLimit,
					TotalLimit:      mockTotalTokenLimit,
				},
				exists: false,
			},
			want: want{
				true,
				"add_free_storage_assigner: unauthorized access - only the owner can update the variables",
			},
		},
	}
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			args := setExpectations(t, test.name, test.parameters, test.want)

			err := args.ssc.AddFreeStorageAssigner(args.txn, args.input, args.balances)

			require.EqualValues(t, test.want.err, err != nil)
			if err != nil {
				require.EqualValues(t, test.want.errMsg, err.Error())
				return
			}
			require.True(t, mock.AssertExpectationsForObjects(t, args.balances))
		})
	}
}

func TestFreeAllocationRequest(t *testing.T) {
	const (
		mockCooperationId        = "mock cooperation id"
		mockNumBlobbers          = 10
		mockRecipient            = "mock recipient"
		mockTimestamp            = 7000
		mockUserPublicKey        = "mock user public key"
		mockTransactionHash      = "12345678"
		mockReadPoolFraction     = 0.2
		mockMinLock              = 10
		mockMinLockPeriod        = 2 * time.Minute
		mockMaxLockPeriod        = 8760 * time.Hour
		mockFreeTokens           = 5 * mockMinLock
		mockIndividualTokenLimit = mockFreeTokens + 1
		mockTotalTokenLimit      = mockIndividualTokenLimit * 300
		newSaSaved               = "new storage allocation saved"
	)
	var (
		mockMaxAnnualFreeAllocation = zcnToBalance(100354)
		mockFreeAllocationSettings  = FreeAllocationSettings{
			DataShards:                 5,
			ParityShards:               5,
			Size:                       123456,
			ReadPriceRange:             PriceRange{0, 5000},
			WritePriceRange:            PriceRange{0, 5000},
			MaxChallengeCompletionTime: 1 * time.Hour,
			Duration:                   24 * 365 * time.Hour,
			ReadPoolFraction:           mockReadPoolFraction,
		}
		mockAllBlobbers = &StorageNodes{}
		conf            = &ScConfig{
			MinAllocSize:               1027,
			MinAllocDuration:           5 * time.Minute,
			MaxChallengeCompletionTime: 1 * time.Hour,
			MaxTotalFreeAllocation:     mockMaxAnnualFreeAllocation,
			FreeAllocationSettings:     mockFreeAllocationSettings,
			ReadPool: &ReadPoolConfig{
				MinLock:       zcnToInt64(mockMinLock),
				MinLockPeriod: mockMinLockPeriod,
				MaxLockPeriod: mockMaxLockPeriod,
			},
		}
		now                         = common.Timestamp(23000000)
		mockChallengeCompletionTime = conf.MaxChallengeCompletionTime
	)

	for i := 0; i < mockNumBlobbers; i++ {
		mockBlobber := &StorageNode{
			ID:       strconv.Itoa(i),
			Capacity: 536870912,
			Used:     73,
			Terms: Terms{
				MaxOfferDuration:        mockFreeAllocationSettings.Duration * 2,
				ReadPrice:               mockFreeAllocationSettings.ReadPriceRange.Max,
				ChallengeCompletionTime: mockChallengeCompletionTime,
			},
			LastHealthCheck: now - blobberHealthTime + 1,
		}
		mockAllBlobbers.Nodes.Add(mockBlobber)
	}

	type args struct {
		ssc      *StorageSmartContract
		txn      *transaction.Transaction
		input    []byte
		balances cstate.StateContextI
	}
	type want struct {
		err    bool
		errMsg string
	}
	type parameters struct {
		assigner FreeStorageAssigner
		marker   FreeStorageMarker
		exists   bool
	}

	setExpectations := func(t *testing.T, name string, p parameters, want want) args {
		var err error
		var balances = &mocks.StateContextI{}
		balances.TestData()[newSaSaved] = false
		var readPoolLocked = zcnToInt64(p.marker.FreeTokens * mockReadPoolFraction)
		var writePoolLocked = zcnToInt64(p.marker.FreeTokens) - readPoolLocked

		var txn = &transaction.Transaction{
			ClientID:     p.marker.Recipient,
			ToClientID:   ADDRESS,
			PublicKey:    mockUserPublicKey,
			CreationDate: now,
			Value:        zcnToInt64(p.marker.FreeTokens),
		}
		txn.Hash = mockTransactionHash
		var ssc = &StorageSmartContract{
			SmartContract: sci.NewSC(ADDRESS),
		}

		p.marker.Signature, p.assigner.PublicKey = signFreeAllocationMarker(t, p.marker)

		inputBytes, err := json.Marshal(&p.marker)
		require.NoError(t, err)
		inputObj := FreeStorageAllocationInput{
			RecipientPublicKey: mockUserPublicKey,
			Marker:             string(inputBytes),
		}
		input, err := json.Marshal(&inputObj)
		require.NoError(t, err)

		require.NoError(t, err)
		balances.On(
			"GetTrieNode",
			FreeStorageAssignerKey(ssc.ID, p.marker.Assigner),
		).Return(FsaToFsa(&p.assigner), nil).Once()

		balances.On("GetTrieNode", ScConfigKey(ssc.ID)).Return(conf, nil)

		balances.On("GetTrieNode", ALL_BLOBBERS_KEY).Return(
			mockAllBlobbers, nil,
		).Once()

		for _, blobber := range mockAllBlobbers.Nodes {
			balances.On(
				"GetTrieNode", StakePoolKey(ssc.ID, blobber.ID),
			).Return(NewStakePool(), nil).Twice()
			balances.On(
				"InsertTrieNode", blobber.GetKey(ssc.ID), mock.Anything,
			).Return("", nil).Once()
			balances.On(
				"InsertTrieNode", StakePoolKey(ssc.ID, blobber.ID), mock.Anything,
			).Return("", nil).Once()
		}

		balances.On(
			"InsertTrieNode", ALL_BLOBBERS_KEY, mock.Anything,
		).Return("", nil).Once()
		balances.On(
			"GetTrieNode", WritePoolKey(ssc.ID, p.marker.Recipient),
		).Return(nil, util.ErrValueNotPresent).Once()

		balances.On(
			"GetTrieNode", ChallengePoolKey(ssc.ID, txn.Hash),
		).Return(nil, util.ErrValueNotPresent).Once()
		balances.On(
			"InsertTrieNode", ChallengePoolKey(ssc.ID, txn.Hash), mock.Anything,
		).Return("", nil).Once()

		var clientAlloc = ClientAllocation{ClientID: p.marker.Recipient}
		balances.On(
			"GetTrieNode", clientAlloc.GetKey(ssc.ID),
		).Return(nil, util.ErrValueNotPresent).Once()
		balances.On(
			"GetTrieNode", ALL_ALLOCATIONS_KEY,
		).Return(&Allocations{}, nil).Once()

		allocation := StorageAllocation{ID: txn.Hash}
		balances.On(
			"GetTrieNode",
			mock.MatchedBy(func(key string) bool {
				if balances.TestData()[newSaSaved].(bool) {
					return false
				}
				balances.TestData()[newSaSaved] = true
				return key == allocation.GetKey(ssc.ID)
			}),
		).Return(nil, util.ErrValueNotPresent).Once()
		balances.On(
			"InsertTrieNode", ALL_ALLOCATIONS_KEY, mock.Anything,
		).Return("", nil).Once()
		balances.On(
			"InsertTrieNode", clientAlloc.GetKey(ssc.ID), mock.Anything,
		).Return("", nil).Once()
		balances.On(
			"InsertTrieNode", allocation.GetKey(ssc.ID), mock.Anything,
		).Return("", nil).Once()

		balances.On(
			"InsertTrieNode",
			FreeStorageAssignerKey(ssc.ID, p.marker.Assigner),
			FsaToFsa(&FreeStorageAssigner{
				ClientId:           p.assigner.ClientId,
				PublicKey:          p.assigner.PublicKey,
				IndividualLimit:    p.assigner.IndividualLimit,
				TotalLimit:         p.assigner.TotalLimit,
				CurrentRedeemed:    p.assigner.CurrentRedeemed + state.Balance(txn.Value),
				RedeemedTimestamps: append(p.assigner.RedeemedTimestamps, p.marker.Timestamp),
			}),
		).Return("", nil).Once()

		balances.On("AddMint", &state.Mint{
			Minter:     ADDRESS,
			ToClientID: ADDRESS,
			Amount:     state.Balance(writePoolLocked),
		}).Return(nil).Once()

		balances.On("InsertTrieNode",
			WritePoolKey(ssc.ID, p.marker.Recipient),
			mock.MatchedBy(func(iwp interface{}) bool {
				wp, ok := IToWritePool(iwp)
				if !ok {
					return false
				}
				pool, found := wp.Pools.Get(mockTransactionHash)
				require.True(t, found)
				return pool.Balance == state.Balance(writePoolLocked) &&
					pool.ID == mockTransactionHash &&
					pool.AllocationID == mockTransactionHash &&
					len(pool.Blobbers) == mockNumBlobbers &&
					pool.ExpireAt == common.Timestamp(common.ToTime(txn.CreationDate).Add(
						conf.FreeAllocationSettings.Duration).Unix())+toSeconds(mockChallengeCompletionTime)
			})).Return("", nil).Once()

		// readPoolLock blockchain access
		balances.On(
			"GetTrieNode", FundedPoolsKey(ssc.ID, p.marker.Recipient),
		).Return(nil, util.ErrValueNotPresent).Once()
		balances.On(
			"InsertTrieNode", FundedPoolsKey(ssc.ID, p.marker.Recipient), mock.Anything,
		).Return("", nil).Once()

		balances.On(
			"GetTrieNode",
			mock.MatchedBy(func(key string) bool {
				return balances.TestData()[newSaSaved].(bool) &&
					key == allocation.GetKey(ssc.ID)
			}),
		).Return(&allocation, nil).Once()

		balances.On(
			"GetSignatureScheme",
		).Return(encryption.NewBLS0ChainScheme()).Once()

		balances.On(
			"AddMint", &state.Mint{
				Minter:     ADDRESS,
				ToClientID: ADDRESS,
				Amount:     state.Balance(readPoolLocked),
			},
		).Return(nil).Once()

		balances.On(
			"GetTrieNode", ReadPoolKey(ssc.ID, p.marker.Recipient),
		).Return(nil, util.ErrValueNotPresent).Once()
		balances.On("InsertTrieNode",
			ReadPoolKey(ssc.ID, p.marker.Recipient),
			mock.MatchedBy(func(irp interface{}) bool {
				rp, ok := IToReadPool(irp)
				if !ok {
					return false
				}
				pool, found := rp.Pools.Get(mockTransactionHash)
				require.True(t, found)
				return pool.Balance == state.Balance(readPoolLocked) &&
					pool.ID == mockTransactionHash &&
					pool.AllocationID == mockTransactionHash &&
					pool.ExpireAt == txn.CreationDate+toSeconds(conf.FreeAllocationSettings.Duration)
			})).Return("", nil).Once()

		return args{ssc, txn, input, balances}
	}

	testCases := []struct {
		name       string
		parameters parameters
		want       want
	}{
		{
			name: "ok_no_previous",
			parameters: parameters{
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "ok_no_previous",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "ok_no_previous",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
				},
			},
		},
		{
			name: "Total_limit_exceeded",
			parameters: parameters{
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "Total_limit_exceeded",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "Total_limit_exceeded",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
					CurrentRedeemed: zcnToBalance(mockTotalTokenLimit),
				},
			},
			want: want{
				true,
				"free_allocation_failed: marker verification failed: 153500000000000 exceeded total permitted free storage limit 153000000000000",
			},
		},
		{
			name: "individual_limit_exceeded",
			parameters: parameters{
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "individual_limit_exceeded",
					Recipient:  mockRecipient,
					FreeTokens: mockIndividualTokenLimit + 1,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "individual_limit_exceeded",
					PublicKey:       "",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
				},
			},
			want: want{
				true,
				"free_allocation_failed: marker verification failed: 520000000000 exceeded permitted free storage  510000000000",
			},
		},
		{
			name: "future_timestamp",
			parameters: parameters{
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "future_timestamp",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  now + 1,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "future_timestamp",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
				},
			},
			want: want{
				true,
				"free_allocation_failed: marker verification failed: marker timestamped in the future: 23000001",
			},
		},
		{
			name: "repeated_old_timestamp",
			parameters: parameters{
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "repeated_old_timestamp",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:           mockCooperationId + "repeated_old_timestamp",
					IndividualLimit:    zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:         zcnToBalance(mockTotalTokenLimit),
					RedeemedTimestamps: []common.Timestamp{190, mockTimestamp},
				},
			},
			want: want{
				true,
				"free_allocation_failed: marker verification failed: marker already redeemed, timestamp: 7000",
			},
		},
	}
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			args := setExpectations(t, test.name, test.parameters, test.want)

			_, err := args.ssc.FreeAllocationRequest(args.txn, args.input, args.balances)

			require.EqualValues(t, test.want.err, err != nil)
			if err != nil {
				require.EqualValues(t, test.want.errMsg, err.Error())
				return
			}
			require.True(t, mock.AssertExpectationsForObjects(t, args.balances))
		})
	}
}

func TestUpdateFreeStorageRequest(t *testing.T) {
	const (
		mockCooperationId        = "mock cooperation id"
		mockAllocationId         = "mock allocation id"
		mockIndividualTokenLimit = 20
		mockTotalTokenLimit      = 3000
		mockNumBlobbers          = 10
		mockRecipient            = "mock recipient"
		mockFreeTokens           = 5
		mockTimestamp            = 7000
		mockUserPublicKey        = "mock user public key"
		mockTransactionHash      = "12345678"
	)
	var mockTimeUnit = 1 * time.Hour
	var mockMaxAnnualFreeAllocation = zcnToBalance(100354)
	var mockFreeAllocationSettings = FreeAllocationSettings{
		DataShards:                 5,
		ParityShards:               5,
		Size:                       123456,
		ReadPriceRange:             PriceRange{0, 5000},
		WritePriceRange:            PriceRange{0, 5000},
		MaxChallengeCompletionTime: 1 * time.Hour,
		Duration:                   24 * 365 * time.Hour,
	}
	var mockAllBlobbers = &StorageNodes{}
	var conf = &ScConfig{
		MinAllocSize:               1027,
		MinAllocDuration:           5 * time.Minute,
		MaxChallengeCompletionTime: 1 * time.Hour,
		MaxTotalFreeAllocation:     mockMaxAnnualFreeAllocation,
		FreeAllocationSettings:     mockFreeAllocationSettings,
	}
	var now = common.Timestamp(29000000)
	var mockChallengeCompletionTime = conf.MaxChallengeCompletionTime
	for i := 0; i < mockNumBlobbers; i++ {
		mockBlobber := &StorageNode{
			ID:       strconv.Itoa(i),
			Capacity: 536870912,
			Used:     73,
			Terms: Terms{
				MaxOfferDuration:        mockFreeAllocationSettings.Duration * 2,
				ReadPrice:               mockFreeAllocationSettings.ReadPriceRange.Max,
				ChallengeCompletionTime: mockChallengeCompletionTime,
			},
			LastHealthCheck: now - blobberHealthTime + 1,
		}
		mockAllBlobbers.Nodes.Add(mockBlobber)
	}

	type args struct {
		ssc      *StorageSmartContract
		txn      *transaction.Transaction
		input    []byte
		balances cstate.StateContextI
	}
	type want struct {
		err    bool
		errMsg string
	}
	type parameters struct {
		assigner     FreeStorageAssigner
		allocationId string
		marker       FreeStorageMarker
		doesNotExist bool
	}

	setExpectations := func(t *testing.T, name string, p parameters, want want) args {
		var err error
		var balances = &mocks.StateContextI{}
		var txn = &transaction.Transaction{
			ClientID:     p.marker.Recipient,
			PublicKey:    mockUserPublicKey,
			CreationDate: now,
			Value:        zcnToInt64(p.marker.FreeTokens),
		}
		txn.Hash = mockTransactionHash
		var ssc = &StorageSmartContract{
			SmartContract: sci.NewSC(ADDRESS),
		}

		p.marker.Signature, p.assigner.PublicKey = signFreeAllocationMarker(t, FreeStorageMarker{
			Assigner:   p.marker.Assigner,
			Recipient:  p.marker.Recipient,
			FreeTokens: p.marker.FreeTokens,
			Timestamp:  p.marker.Timestamp,
		})

		markerBytes, err := json.Marshal(&p.marker)
		require.NoError(t, err)
		var inputObj = &FreeStorageUpgradeInput{
			AllocationId: p.allocationId,
			Marker:       string(markerBytes),
		}
		input, err := json.Marshal(inputObj)
		require.NoError(t, err)

		if p.doesNotExist {
			balances.On(
				"GetTrieNode",
				FreeStorageAssignerKey(ssc.ID, p.marker.Assigner),
			).Return(nil, util.ErrValueNotPresent).Once()
		} else {
			balances.On(
				"GetTrieNode",
				FreeStorageAssignerKey(ssc.ID, p.marker.Assigner),
			).Return(FsaToFsa(&p.assigner), nil).Once()
		}

		balances.On("GetTrieNode", ScConfigKey(ssc.ID)).Return(conf, nil).Once()

		balances.On("GetTrieNode", ALL_BLOBBERS_KEY).Return(
			mockAllBlobbers, nil,
		).Once()

		ca := ClientAllocation{
			ClientID:    p.marker.Recipient,
			Allocations: &Allocations{},
		}
		ca.Allocations.List.Add(p.allocationId)
		balances.On("GetTrieNode", ca.GetKey(ssc.ID)).Return(
			&ca, nil,
		).Once()

		var sa = StorageAllocation{
			ID:           p.allocationId,
			Owner:        p.marker.Recipient,
			Expiration:   now + 1,
			DataShards:   conf.FreeAllocationSettings.DataShards,
			ParityShards: conf.FreeAllocationSettings.ParityShards,
			TimeUnit:     mockTimeUnit,
		}
		for _, blobber := range mockAllBlobbers.Nodes {
			balances.On(
				"GetTrieNode", blobber.GetKey(ssc.ID),
			).Return(blobber, nil).Once()
			var sp = NewStakePool()
			sp.SetOffer(p.allocationId, &OfferPool{})
			balances.On(
				"GetTrieNode", StakePoolKey(ssc.ID, blobber.ID),
			).Return(sp, nil).Once()
			balances.On(
				"InsertTrieNode", blobber.GetKey(ssc.ID), mock.Anything,
			).Return("", nil).Once()
			balances.On(
				"InsertTrieNode", StakePoolKey(ssc.ID, blobber.ID), mock.Anything,
			).Return("", nil).Once()
			sa.BlobberDetails = append(sa.BlobberDetails, &BlobberAllocation{
				BlobberID:    blobber.ID,
				AllocationID: p.allocationId,
			})
		}
		balances.On("GetTrieNode", sa.GetKey(ssc.ID)).Return(
			&sa, nil,
		).Once()
		balances.On(
			"InsertTrieNode", sa.GetKey(ssc.ID), mock.Anything,
		).Return("", nil).Once()

		balances.On(
			"InsertTrieNode", ALL_BLOBBERS_KEY, mock.Anything,
		).Return("", nil).Once()
		balances.On(
			"GetTrieNode", WritePoolKey(ssc.ID, p.marker.Recipient),
		).Return(WpToWp(&WritePool{}), nil).Once()

		balances.On(
			"GetTrieNode", ChallengePoolKey(ssc.ID, p.allocationId),
		).Return(&ChallengePool{}, nil).Once()

		balances.On(
			"GetSignatureScheme",
		).Return(encryption.NewBLS0ChainScheme()).Once()

		balances.On(
			"InsertTrieNode",
			FreeStorageAssignerKey(ssc.ID, p.marker.Assigner),
			FsaToFsa(&FreeStorageAssigner{
				ClientId:           p.assigner.ClientId,
				PublicKey:          p.assigner.PublicKey,
				IndividualLimit:    p.assigner.IndividualLimit,
				TotalLimit:         p.assigner.TotalLimit,
				CurrentRedeemed:    p.assigner.CurrentRedeemed + state.Balance(txn.Value),
				RedeemedTimestamps: append(p.assigner.RedeemedTimestamps, p.marker.Timestamp),
			}),
		).Return("", nil).Once()

		balances.On("AddMint", &state.Mint{
			Minter:     ADDRESS,
			ToClientID: ADDRESS,
			Amount:     zcnToBalance(p.marker.FreeTokens),
		}).Return(nil).Once()

		balances.On(
			"InsertTrieNode",
			WritePoolKey(ssc.ID, p.marker.Recipient),
			mock.MatchedBy(func(iwp interface{}) bool {
				wp, ok := IToWritePool(iwp)
				if !ok {
					return false
				}
				pool, found := wp.Pools.Get(p.allocationId)
				require.True(t, found)
				return pool.Balance == zcnToBalance(p.marker.FreeTokens) &&
					pool.ID == mockTransactionHash &&
					pool.AllocationID == p.allocationId &&
					len(pool.Blobbers) == mockNumBlobbers
			}),
		).Return("", nil).Once()

		return args{ssc, txn, input, balances}
	}

	testCases := []struct {
		name       string
		parameters parameters
		want       want
	}{
		{
			name: "ok_no_previous",
			parameters: parameters{
				allocationId: mockAllocationId,
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "ok_no_previous",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "ok_no_previous",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
				},
			},
		},
		{
			name: "Total_limit_exceeded",
			parameters: parameters{
				allocationId: mockAllocationId,
				marker: FreeStorageMarker{

					Assigner:   mockCooperationId + "Total_limit_exceeded",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "Total_limit_exceeded",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
					CurrentRedeemed: zcnToBalance(mockTotalTokenLimit),
				},
			},
			want: want{
				true,
				"update_free_storage_request: marker verification failed: 30050000000000 exceeded total permitted free storage limit 30000000000000",
			},
		},
		{
			name: "individual_limit_exceeded",
			parameters: parameters{
				allocationId: mockAllocationId,
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "individual_limit_exceeded",
					Recipient:  mockRecipient,
					FreeTokens: mockIndividualTokenLimit + 1,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:        mockCooperationId + "individual_limit_exceeded",
					IndividualLimit: zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:      zcnToBalance(mockTotalTokenLimit),
				},
			},
			want: want{
				true,
				"update_free_storage_request: marker verification failed: 210000000000 exceeded permitted free storage  200000000000",
			},
		},
		{
			name: "assigner_not_on_blockchain",
			parameters: parameters{
				allocationId: mockAllocationId,
				marker: FreeStorageMarker{
					Assigner:   mockCooperationId + "assigner_not_on_blockchain",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				doesNotExist: true,
			},
			want: want{
				true,
				"update_free_storage_request: error getting assigner details: value not present",
			},
		},
		{
			name: "repeated_old_timestamp",
			parameters: parameters{
				allocationId: mockAllocationId,
				marker: FreeStorageMarker{

					Assigner:   mockCooperationId + "repeated_old_timestamp",
					Recipient:  mockRecipient,
					FreeTokens: mockFreeTokens,
					Timestamp:  mockTimestamp,
				},
				assigner: FreeStorageAssigner{
					ClientId:           mockCooperationId + "repeated_old_timestamp",
					IndividualLimit:    zcnToBalance(mockIndividualTokenLimit),
					TotalLimit:         zcnToBalance(mockTotalTokenLimit),
					RedeemedTimestamps: []common.Timestamp{mockTimestamp},
				},
			},
			want: want{
				true,
				"update_free_storage_request: marker verification failed: marker already redeemed, timestamp: 7000",
			},
		},
	}
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			args := setExpectations(t, test.name, test.parameters, test.want)

			_, err := args.ssc.UpdateFreeStorageRequest(args.txn, args.input, args.balances)

			require.EqualValues(t, test.want.err, err != nil)
			if err != nil {
				require.EqualValues(t, test.want.errMsg, err.Error())
				return
			}
			require.True(t, mock.AssertExpectationsForObjects(t, args.balances))
		})
	}
}

func signFreeAllocationMarker(t *testing.T, frm FreeStorageMarker) (string, string) {
	var request = struct {
		Recipient  string           `json:"recipient"`
		FreeTokens float64          `json:"free_tokens"`
		Timestamp  common.Timestamp `json:"timestamp"`
	}{
		frm.Recipient, frm.FreeTokens, frm.Timestamp,
	}
	responseBytes, err := json.Marshal(&request)
	require.NoError(t, err)
	signatureScheme := encryption.NewBLS0ChainScheme()
	err = signatureScheme.GenerateKeys()
	require.NoError(t, err)
	signature, err := signatureScheme.Sign(hex.EncodeToString(responseBytes))
	require.NoError(t, err)
	return signature, signatureScheme.GetPublicKey()
}

func zcnToBalance(token float64) state.Balance {
	return state.Balance(token * float64(1e10))
}

func zcnToInt64(token float64) int64 {
	return int64(token * float64(1e10))
}

func toSeconds(dur time.Duration) common.Timestamp {
	return common.Timestamp(dur / time.Second)
}
