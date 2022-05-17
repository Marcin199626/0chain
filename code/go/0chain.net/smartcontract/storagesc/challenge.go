package storagesc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"

	"0chain.net/pkg/currency"

	"0chain.net/smartcontract/dbs"
	"0chain.net/smartcontract/dbs/event"
	"0chain.net/smartcontract/stakepool/spenum"

	"0chain.net/smartcontract/partitions"

	"0chain.net/chaincore/block"
	cstate "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	"0chain.net/core/logging"
	"0chain.net/core/util"

	"go.uber.org/zap"
)

const blobberAllocationPartitionSize = 100

// completeChallenge complete the challenge
func (sc *StorageSmartContract) completeChallenge(
	challenge *StorageChallenge,
	allocChallenges *AllocationChallenges,
	blobChallenges *BlobberChallenges,
	challengeResponse *ChallengeResponse) bool {

	// TODO: do not remove the comments in case the blobber could not work
	//found := false
	//if len(allocChallenges.OpenChallenges) > 0 {
	//	latestOpenChallengeID := allocChallenges.OpenChallenges[0].ID
	//	if latestOpenChallengeID == challenge.ID {
	//		found = true
	//	}
	//}

	if !allocChallenges.removeChallenge(challenge) {
		return false
	}

	if !blobChallenges.removeChallenge(challenge) {
		return false
	}

	if challengeResponse != nil {
		challenge.Responded = true
	}

	return true
}

func (sc *StorageSmartContract) getBlobberChallenges(blobberID string,
	balances cstate.StateContextI) (*BlobberChallenges, error) {

	bc := &BlobberChallenges{BlobberID: blobberID}
	err := bc.load(balances, sc.ID)
	if err != nil {
		return nil, err
	}

	return bc, nil
}

func (sc *StorageSmartContract) getStorageChallenge(challengeID string,
	balances cstate.StateContextI) (challenge *StorageChallenge, err error) {

	challenge = new(StorageChallenge)
	challenge.ID = challengeID
	err = balances.GetTrieNode(challenge.GetKey(sc.ID), challenge)
	if err != nil {
		return nil, err
	}

	return challenge, nil
}

func (sc *StorageSmartContract) getAllocationChallenges(allocID string,
	balances cstate.StateContextI) (ac *AllocationChallenges, err error) {

	ac = new(AllocationChallenges)
	ac.AllocationID = allocID
	err = balances.GetTrieNode(ac.GetKey(sc.ID), ac)
	if err != nil {
		return nil, err
	}

	return ac, nil
}

// move tokens from challenge pool to blobber's stake pool (to unlocked)
func (sc *StorageSmartContract) blobberReward(t *transaction.Transaction,
	alloc *StorageAllocation, prev common.Timestamp, allocChallenges *AllocationChallenges,
	blobAlloc *BlobberAllocation, validators []string, partial float64,
	balances cstate.StateContextI) (err error) {

	var conf *Config
	if conf, err = sc.getConfig(balances, true); err != nil {
		return fmt.Errorf("can't get SC configurations: %v", err.Error())
	}

	// time of this challenge
	var tp = allocChallenges.LatestCompletedChallenge.Created

	if tp > alloc.Expiration+toSeconds(blobAlloc.Terms.ChallengeCompletionTime) {
		return errors.New("late challenge response")
	}

	if tp > alloc.Expiration {
		tp = alloc.Expiration // last challenge
	}

	// pool
	var cp *challengePool
	if cp, err = sc.getChallengePool(alloc.ID, balances); err != nil {
		return fmt.Errorf("can't get allocation's challenge pool: %v", err)
	}

	var (
		rdtu = alloc.restDurationInTimeUnits(prev)
		dtu  = alloc.durationInTimeUnits(tp - prev)
		move = float64(blobAlloc.challenge(dtu, rdtu))
	)

	// part of this tokens goes to related validators
	var validatorsReward = conf.ValidatorReward * move
	move -= validatorsReward

	// for a case of a partial verification
	blobberReward := move * partial // blobber (partial) reward
	back := move - blobberReward    // return back to write pool

	if back > 0 {
		// move back to write pool
		var wp *writePool
		if wp, err = sc.getWritePool(alloc.Owner, balances); err != nil {
			return fmt.Errorf("can't get allocation's write pool: %v", err)
		}
		var until = alloc.Until()
		err = cp.moveToWritePool(alloc, blobAlloc.BlobberID, until, wp, currency.Coin(back))
		if err != nil {
			return fmt.Errorf("moving partial challenge to write pool: %v", err)
		}
		alloc.MovedBack += currency.Coin(back)
		blobAlloc.Returned += currency.Coin(back)
		// save write pool
		if err = wp.save(sc.ID, alloc.Owner, balances); err != nil {
			return fmt.Errorf("can't save allocation's write pool: %v", err)
		}
	}

	var sp *stakePool
	if sp, err = sc.getStakePool(blobAlloc.BlobberID, balances); err != nil {
		return fmt.Errorf("can't get stake pool: %v", err)
	}

	err = sp.DistributeRewards(currency.Coin(blobberReward), blobAlloc.BlobberID, spenum.Blobber, balances)
	if err != nil {
		return fmt.Errorf("can't move tokens to blobber: %v", err)
	}

	blobAlloc.ChallengeReward += currency.Coin(blobberReward)

	// validators' stake pools
	var vsps []*stakePool
	if vsps, err = sc.validatorsStakePools(validators, balances); err != nil {
		return
	}

	err = cp.moveToValidators(sc.ID, validatorsReward, validators, vsps, balances)
	if err != nil {
		return fmt.Errorf("rewarding validators: %v", err)
	}
	alloc.MovedToValidators += currency.Coin(validatorsReward)

	// save validators' stake pools
	if err = sc.saveStakePools(validators, vsps, balances); err != nil {
		return
	}

	// save the pools
	if err = sp.save(sc.ID, blobAlloc.BlobberID, balances); err != nil {
		return fmt.Errorf("can't save sake pool: %v", err)
	}

	if err = cp.save(sc.ID, alloc.ID, balances); err != nil {
		return fmt.Errorf("can't save allocation's challenge pool: %v", err)
	}

	return
}

// obtain stake pools of given validators
func (ssc *StorageSmartContract) validatorsStakePools(
	validators []datastore.Key, balances cstate.StateContextI) (
	sps []*stakePool, err error) {

	sps = make([]*stakePool, 0, len(validators))
	for _, id := range validators {
		var sp *stakePool
		if sp, err = ssc.getStakePool(id, balances); err != nil {
			return nil, fmt.Errorf("can't get validator %s stake pool: %v",
				id, err)
		}
		sps = append(sps, sp)
	}

	return
}

func (ssc *StorageSmartContract) saveStakePools(validators []datastore.Key,
	sps []*stakePool, balances cstate.StateContextI) (err error) {

	for i, sp := range sps {
		if err = sp.save(ssc.ID, validators[i], balances); err != nil {
			return fmt.Errorf("saving stake pool: %v", err)
		}
		data, _ := json.Marshal(dbs.DbUpdates{
			Id: validators[i],
			Updates: map[string]interface{}{
				"total_stake": int64(sp.stake()),
			},
		})
		balances.EmitEvent(event.TypeStats, event.TagUpdateBlobber, validators[i], string(data))

	}
	return
}

// move tokens from challenge pool back to write pool
func (sc *StorageSmartContract) blobberPenalty(t *transaction.Transaction,
	alloc *StorageAllocation, prev common.Timestamp, ac *AllocationChallenges,
	blobAlloc *BlobberAllocation, validators []string,
	balances cstate.StateContextI) (err error) {

	var conf *Config
	if conf, err = sc.getConfig(balances, true); err != nil {
		return fmt.Errorf("can't get SC configurations: %v", err.Error())
	}

	// time of this challenge
	var tp = ac.LatestCompletedChallenge.Created

	if tp > alloc.Expiration+toSeconds(blobAlloc.Terms.ChallengeCompletionTime) {
		return errors.New("late challenge response")
	}

	if tp > alloc.Expiration {
		tp = alloc.Expiration // last challenge
	}

	// pools
	var cp *challengePool
	if cp, err = sc.getChallengePool(alloc.ID, balances); err != nil {
		return fmt.Errorf("can't get allocation's challenge pool: %v", err)
	}

	var wp *writePool
	if wp, err = sc.getWritePool(alloc.Owner, balances); err != nil {
		return fmt.Errorf("can't get allocation's write pool: %v", err)
	}

	var (
		rdtu = alloc.restDurationInTimeUnits(prev)
		dtu  = alloc.durationInTimeUnits(tp - prev)
		move = float64(blobAlloc.challenge(dtu, rdtu))
	)

	// part of this tokens goes to related validators
	var validatorsReward = conf.ValidatorReward * move
	move -= validatorsReward

	// validators' stake pools
	var vsps []*stakePool
	if vsps, err = sc.validatorsStakePools(validators, balances); err != nil {
		return
	}

	// validators reward
	err = cp.moveToValidators(sc.ID, validatorsReward, validators, vsps, balances)
	if err != nil {
		return fmt.Errorf("rewarding validators: %v", err)
	}
	alloc.MovedToValidators += currency.Coin(validatorsReward)

	// save validators' stake pools
	if err = sc.saveStakePools(validators, vsps, balances); err != nil {
		return
	}

	// move back to write pool
	var until = alloc.Until()
	err = cp.moveToWritePool(alloc, blobAlloc.BlobberID, until, wp, currency.Coin(move))
	if err != nil {
		return fmt.Errorf("moving failed challenge to write pool: %v", err)
	}
	alloc.MovedBack += currency.Coin(move)
	blobAlloc.Returned += currency.Coin(move)

	// blobber stake penalty
	if conf.BlobberSlash > 0 && move > 0 &&
		currency.Coin(conf.BlobberSlash*move) > 0 {

		var slash = currency.Coin(conf.BlobberSlash * move)

		// load stake pool
		var sp *stakePool
		if sp, err = sc.getStakePool(blobAlloc.BlobberID, balances); err != nil {
			return fmt.Errorf("can't get blobber's stake pool: %v", err)
		}

		var move currency.Coin
		move, err = sp.slash(alloc, blobAlloc.BlobberID, until, wp, blobAlloc.Offer(), slash, balances)
		if err != nil {
			return fmt.Errorf("can't move tokens to write pool: %v", err)
		}

		sp.TotalOffers -= move    // subtract the offer stake
		blobAlloc.Penalty += move // penalty statistic

		// save stake pool
		if err = sp.save(sc.ID, blobAlloc.BlobberID, balances); err != nil {
			return fmt.Errorf("can't save blobber's stake pool: %v", err)
		}
	}

	// save pools
	if err = wp.save(sc.ID, alloc.Owner, balances); err != nil {
		return fmt.Errorf("can't save allocation's write pool: %v", err)
	}

	if err = cp.save(sc.ID, alloc.ID, balances); err != nil {
		return fmt.Errorf("can't save allocation's challenge pool: %v", err)
	}

	return
}

func (sc *StorageSmartContract) verifyChallenge(t *transaction.Transaction,
	input []byte, balances cstate.StateContextI) (resp string, err error) {

	var challResp ChallengeResponse

	conf, err := sc.getConfig(balances, true)
	if err != nil {
		return "", common.NewError("verify_challenge",
			"cannot get smart contract configurations: "+err.Error())
	}

	rewardRound := GetCurrentRewardRound(balances.GetBlock().Round, conf.BlockReward.TriggerPeriod)

	ongoingParts, err := getOngoingPassedBlobberRewardsPartitions(balances, conf.BlockReward.TriggerPeriod)
	if err != nil {
		return "", common.NewError("verify_challenge",
			"cannot get ongoing partition: "+err.Error())
	}

	if err = json.Unmarshal(input, &challResp); err != nil {
		return
	}

	if len(challResp.ID) == 0 ||
		len(challResp.ValidationTickets) == 0 {

		return "", common.NewError("verify_challenge",
			"Invalid parameters to challenge response")
	}

	// get challenge node
	challenge, err := sc.getStorageChallenge(challResp.ID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge", "could not find challenge, %v", err)
	}

	if challenge.BlobberID != t.ClientID {
		return "", common.NewErrorf("verify_challenge", "challenge blobber id does not match")
	}

	allocChallenges, err := sc.getAllocationChallenges(challenge.AllocationID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge", "could not find allocation challenges, %v", err)
	}

	blobChallenges, err := sc.getBlobberChallenges(t.ClientID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge",
			"could not get the blobber challenge %s: %v", t.ClientID, err)
	}

	_, ok := allocChallenges.ChallengeMap[challResp.ID]
	if !ok {
		lcc := allocChallenges.LatestCompletedChallenge
		if allocChallenges.LatestCompletedChallenge != nil &&
			challResp.ID == lcc.ID && lcc.Responded {
			return "challenge already redeemed", nil
		}

		return "", common.NewErrorf("verify_challenge",
			"could not find the challenge with ID %s", challResp.ID)
	}

	var alloc *StorageAllocation
	alloc, err = sc.getAllocation(challenge.AllocationID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge",
			"can't get related allocation: %v", err)
	}

	blobAlloc, ok := alloc.BlobberAllocsMap[t.ClientID]
	if !ok {
		return "", common.NewError("verify_challenge",
			"Blobber is not part of the allocation")
	}

	var (
		success, failure int
		validators       []string // validators for rewards
	)
	for _, vt := range challResp.ValidationTickets {
		if vt != nil {
			if ok, err := vt.VerifySign(balances); !ok || err != nil {
				continue
			}

			validators = append(validators, vt.ValidatorID)

			if !vt.Result {
				failure++
				continue
			}
			success++
		}
	}

	// time of previous complete challenge (not the current one)
	// or allocation start time if no challenges
	var prev = alloc.StartTime
	if last := allocChallenges.LatestCompletedChallenge; last != nil {
		prev = last.Created
	}

	var (
		threshold = challenge.TotalValidators / 2
		pass      = success > threshold ||
			(success > failure && success+failure < threshold)
		cct   = toSeconds(blobAlloc.Terms.ChallengeCompletionTime)
		fresh = challenge.Created+cct >= t.CreationDate
	)

	// verification, or partial verification
	if pass && fresh {
		blobber, err := sc.getBlobber(t.ClientID, balances)
		if err != nil {
			return "", common.NewError("verify_challenge",
				"can't get blobber"+err.Error())
		}

		// this expiry of blobber needs to be corrected once logic is finalized

		if blobber.RewardPartition.StartRound != rewardRound ||
			balances.GetBlock().Round == 0 {

			var dataRead float64 = 0
			if blobber.LastRewardDataReadRound >= rewardRound {
				dataRead = blobber.DataReadLastRewardRound
			}

			partIndex, err := ongoingParts.AddItem(
				balances,
				&BlobberRewardNode{
					ID:                blobber.ID,
					SuccessChallenges: 0,
					WritePrice:        blobber.Terms.WritePrice,
					ReadPrice:         blobber.Terms.ReadPrice,
					TotalData:         sizeInGB(blobber.BytesWritten),
					DataRead:          dataRead,
				})
			if err != nil {
				return "", common.NewError("verify_challenge",
					"can't add to ongoing partition list "+err.Error())
			}

			blobber.RewardPartition = RewardPartitionLocation{
				Index:      partIndex,
				StartRound: rewardRound,
				Timestamp:  t.CreationDate,
			}

			_, err = balances.InsertTrieNode(blobber.GetKey(sc.ID), blobber)
			if err != nil {
				return "", common.NewError("verify_challenge",
					"error inserting blobber to chain"+err.Error())
			}
		}

		var brStats BlobberRewardNode
		if err := ongoingParts.GetItem(balances, blobber.RewardPartition.Index, blobber.ID, &brStats); err != nil {
			return "", common.NewError("verify_challenge",
				"can't get blobber reward from partition list: "+err.Error())
		}

		brStats.SuccessChallenges++

		if !sc.completeChallenge(challenge, allocChallenges, blobChallenges, &challResp) {
			return "", common.NewError("challenge_out_of_order",
				"First challenge on the list is not same as the one"+
					" attempted to redeem")
		}
		alloc.Stats.LastestClosedChallengeTxn = challenge.ID
		alloc.Stats.SuccessChallenges++
		alloc.Stats.OpenChallenges--

		blobAlloc.Stats.LastestClosedChallengeTxn = challenge.ID
		blobAlloc.Stats.SuccessChallenges++
		blobAlloc.Stats.OpenChallenges--

		if err := challenge.Save(balances, sc.ID); err != nil {
			return "", common.NewError("verify_challenge_error", err.Error())
		}

		err = emitUpdateChallengeResponse(challenge.ID, challenge.Responded, balances)
		if err != nil {
			return "", common.NewError("verify_challenge_error", err.Error())
		}

		err = ongoingParts.UpdateItem(balances, blobber.RewardPartition.Index, &brStats)
		if err != nil {
			return "", common.NewError("verify_challenge",
				"error updating blobber reward item")
		}

		err = ongoingParts.Save(balances)
		if err != nil {
			return "", common.NewError("verify_challenge",
				"error saving ongoing blobber reward partition")
		}

		if err := allocChallenges.Save(balances, sc.ID); err != nil {
			return "", common.NewError("verify_challenge", err.Error())
		}

		if err := blobChallenges.save(balances, sc.ID); err != nil {
			return "", common.NewError("verify_challenge", err.Error())
		}

		var partial = 1.0
		if success < threshold {
			partial = float64(success) / float64(threshold)
		}

		err = sc.blobberReward(t, alloc, prev, allocChallenges, blobAlloc,
			validators, partial, balances)
		if err != nil {
			return "", common.NewError("challenge_reward_error", err.Error())
		}

		// save allocation object
		_, err = balances.InsertTrieNode(alloc.GetKey(sc.ID), alloc)
		if err != nil {
			return "", common.NewError("challenge_reward_error", err.Error())
		}
		if err := alloc.Save(balances, sc.ID); err != nil {
			return "", common.NewError("challenge_reward_error", err.Error())
		}

		err = emitAddOrOverwriteAllocation(alloc, balances)
		if err != nil {
			return "", common.NewErrorf("verify_challenge",
				"saving allocation in db: %v", err)
		}

		if success < threshold {
			return "challenge passed partially by blobber", nil
		}

		return "challenge passed by blobber", nil
	}

	var enoughFails = failure > (challenge.TotalValidators/2) ||
		(success+failure) == challenge.TotalValidators

	if enoughFails || (pass && !fresh) {

		if !sc.completeChallenge(challenge, allocChallenges, blobChallenges, &challResp) {
			return "", common.NewError("challenge_out_of_order",
				"First challenge on the list is not same as the one"+
					" attempted to redeem")
		}
		alloc.Stats.LastestClosedChallengeTxn = challenge.ID
		alloc.Stats.FailedChallenges++
		alloc.Stats.OpenChallenges--

		blobAlloc.Stats.LastestClosedChallengeTxn = challenge.ID
		blobAlloc.Stats.FailedChallenges++
		blobAlloc.Stats.OpenChallenges--

		err = emitUpdateChallengeResponse(challenge.ID, challenge.Responded, balances)
		if err != nil {
			return "", common.NewError("verify_challenge_error", err.Error())
		}

		if err := allocChallenges.Save(balances, sc.ID); err != nil {
			return "", common.NewError("challenge_penalty_error", err.Error())
		}

		if err := blobChallenges.save(balances, sc.ID); err != nil {
			return "", common.NewError("challenge_penalty_error", err.Error())
		}

		logging.Logger.Info("Challenge failed", zap.Any("challenge", challResp.ID))

		err = sc.blobberPenalty(t, alloc, prev, allocChallenges, blobAlloc,
			validators, balances)
		if err != nil {
			return "", common.NewError("challenge_penalty_error", err.Error())
		}

		// save allocation object
		_, err = balances.InsertTrieNode(alloc.GetKey(sc.ID), alloc)
		if err != nil {
			return "", common.NewError("challenge_reward_error", err.Error())
		}

		err = emitAddOrOverwriteAllocation(alloc, balances)
		if err != nil {
			return "", common.NewErrorf("challenge_reward_error",
				"saving allocation in db: %v", err)
		}

		if pass && !fresh {
			return "late challenge (failed)", nil
		}

		return "Challenge Failed by Blobber", nil
	}

	return "", common.NewError("not_enough_validations",
		"Not enough validations, no successful validations")
}

func (sc *StorageSmartContract) getAllocationForChallenge(
	t *transaction.Transaction,
	allocID string,
	balances cstate.StateContextI) (alloc *StorageAllocation, err error) {

	alloc, err = sc.getAllocation(allocID, balances)
	switch err {
	case nil:
	case util.ErrValueNotPresent:
		logging.Logger.Error("client state has invalid allocations",
			zap.Any("selected_allocation", allocID))
		return nil, common.NewErrorf("invalid_allocation",
			"client state has invalid allocations")
	default:
		return nil, common.NewErrorf("adding_challenge_error",
			"unexpected error getting allocation: %v", err)
	}

	if alloc.Expiration < t.CreationDate {
		return nil, common.NewErrorf("adding_challenge_error",
			"allocation is already expired, alloc.Expiration: %d, t.CreationDate: %d",
			alloc.Expiration, t.CreationDate)
	}
	if alloc.Stats == nil {
		return nil, common.NewError("adding_challenge_error",
			"found empty allocation stats")
	}
	if alloc.Stats.NumWrites > 0 {
		return alloc, nil // found
	}
	return nil, nil
}

type challengeOutput struct {
	alloc             *StorageAllocation
	storageChallenge  *StorageChallenge
	blobberChallenges *BlobberChallenges
	allocChallenges   *AllocationChallenges
	challInfo         *StorageChallengeResponse
}

type challengeBlobberSelection int

// randomWeightSelection select n blobbers from blobberChallenge partition and then select a blobber with the highest weight
// randomSelection select a blobber randomly from partition
const (
	randomWeightSelection challengeBlobberSelection = iota
	randomSelection
)

// selectBlobberForChallenge select blobber for challenge in random manner
func selectBlobberForChallenge(selection challengeBlobberSelection, challengeBlobbersPartition *partitions.Partitions,
	r *rand.Rand, balances cstate.StateContextI) (string, error) {

	var challengeBlobbers []ChallengeReadyBlobber
	err := challengeBlobbersPartition.GetRandomItems(balances, r, &challengeBlobbers)
	if err != nil {
		return "", errors.New("error getting random slice from blobber challenge partition")
	}

	switch selection {
	case randomWeightSelection:
		const maxBlobbersSelect = 5

		var challengeBlobber ChallengeReadyBlobber
		var maxUsedCap uint64

		var blobbersSelected = make([]ChallengeReadyBlobber, 0, maxBlobbersSelect)
		if len(challengeBlobbers) <= maxBlobbersSelect {
			blobbersSelected = challengeBlobbers
		} else {
			for i := 0; i < maxBlobbersSelect; i++ {
				randomIndex := r.Intn(len(challengeBlobbers))
				blobbersSelected = append(blobbersSelected, challengeBlobbers[randomIndex])
			}
		}

		for _, bc := range blobbersSelected {
			if bc.UsedCapacity > maxUsedCap {
				maxUsedCap = bc.UsedCapacity
				challengeBlobber = bc
			}
		}

		return challengeBlobber.BlobberID, nil
	case randomSelection:
		randomIndex := r.Intn(len(challengeBlobbers))
		return challengeBlobbers[randomIndex].BlobberID, nil
	default:
		return "", errors.New("invalid blobber selection pattern")
	}
}

func (sc *StorageSmartContract) populateGenerateChallenge(
	challengeBlobbersPartition *partitions.Partitions,
	seed int64,
	validators *partitions.Partitions,
	txn *transaction.Transaction,
	challengeID string,
	balances cstate.StateContextI,
) (*challengeOutput, error) {
	r := rand.New(rand.NewSource(seed))
	blobberSelection := challengeBlobberSelection(r.Intn(2))
	blobberID, err := selectBlobberForChallenge(blobberSelection, challengeBlobbersPartition, r, balances)
	if err != nil {
		return nil, common.NewError("add_challenge", err.Error())
	}

	if blobberID == "" {
		return nil, common.NewError("add_challenges", "empty blobber id")
	}

	logging.Logger.Debug("generate_challenges", zap.String("blobber id", blobberID))

	// get blobber allocations partitions
	blobberAllocParts, err := partitionsBlobberAllocations(blobberID, balances)
	if err != nil {
		return nil, common.NewErrorf("generate_challenges",
			"error getting blobber_challenge_allocation list: %v", err)
	}

	// get random allocations from the partitions
	var randBlobberAllocs []BlobberAllocationNode
	if err := blobberAllocParts.GetRandomItems(balances, r, &randBlobberAllocs); err != nil {
		return nil, common.NewErrorf("generate_challenges",
			"error getting random slice from blobber challenge allocation partition: %v", err)
	}

	// get a random allocation
	randomIndex := r.Intn(len(randBlobberAllocs))
	allocID := randBlobberAllocs[randomIndex].ID

	// get the storage allocation from MPT
	alloc, err := sc.getAllocationForChallenge(txn, allocID, balances)
	if err != nil {
		return nil, err
	}

	if alloc == nil {
		return nil, errors.New("empty allocation")
	}

	allocBlobber, ok := alloc.BlobberAllocsMap[blobberID]
	if !ok {
		return nil, errors.New("invalid blobber for allocation")
	}

	var randValidators []ValidationPartitionNode
	if err := validators.GetRandomItems(balances, r, &randValidators); err != nil {
		return nil, common.NewError("add_challenge",
			"error getting validators random slice: "+err.Error())
	}

	var (
		needValidNum       = minInt(len(randValidators), alloc.DataShards+1)
		selectedValidators = make([]*ValidationNode, 0, needValidNum)
		perm               = r.Perm(len(randValidators))
	)

	for i := 0; i < needValidNum; i++ {
		randValidator := randValidators[perm[i]]
		if randValidator.Id != blobberID {
			selectedValidators = append(selectedValidators,
				&ValidationNode{
					ID:      randValidator.Id,
					BaseURL: randValidator.Url,
				})
		}
		if len(selectedValidators) >= alloc.DataShards {
			break
		}
	}

	var storageChallenge = new(StorageChallenge)
	storageChallenge.ID = challengeID
	storageChallenge.TotalValidators = len(selectedValidators)
	storageChallenge.BlobberID = blobberID
	storageChallenge.AllocationID = alloc.ID
	storageChallenge.Created = txn.CreationDate

	challInfo := &StorageChallengeResponse{
		StorageChallenge: storageChallenge,
		Validators:       selectedValidators,
		Seed:             seed,
		AllocationRoot:   allocBlobber.AllocationRoot,
	}

	allocChallenges, err := sc.getAllocationChallenges(alloc.ID, balances)
	if err != nil {
		if err == util.ErrValueNotPresent {
			allocChallenges = &AllocationChallenges{}
			allocChallenges.AllocationID = alloc.ID
		} else {
			return nil, common.NewError("add_challenge",
				"error fetching allocation challenge: "+err.Error())
		}
	}

	blobbChallenges, err := sc.getBlobberChallenges(blobberID, balances)
	if err != nil {
		if err == util.ErrValueNotPresent {
			blobbChallenges = &BlobberChallenges{}
			blobbChallenges.BlobberID = blobberID
		} else {
			return nil, common.NewErrorf("add_challenge",
				"error fetching blobber challenge: %v", err)
		}
	}

	return &challengeOutput{
		alloc:             alloc,
		storageChallenge:  storageChallenge,
		allocChallenges:   allocChallenges,
		blobberChallenges: blobbChallenges,
		challInfo:         challInfo,
	}, nil
}

func (sc *StorageSmartContract) generateChallenge(t *transaction.Transaction,
	b *block.Block, _ []byte, balances cstate.StateContextI) (err error) {

	validators, err := getValidatorsList(balances)
	if err != nil {
		return common.NewErrorf("generate_challenge",
			"error getting the validators list: %v", err)
	}

	challengeReadyParts, err := partitionsChallengeReadyBlobbers(balances)
	if err != nil {
		return common.NewErrorf("generate_challenge",
			"error getting the blobber challenge list: %v", err)
	}

	bcNum, err := challengeReadyParts.Size(balances)
	if err != nil {
		return common.NewErrorf("generate_challenge", "error getting blobber challenge size: %v", err)
	}

	if bcNum == 0 {
		logging.Logger.Info("skipping generate challenge: empty blobber challenge partition")
		return nil
	}

	hashSeed := encryption.Hash(t.Hash + b.PrevHash)
	// the "1" was the index when generating multiple challenges.
	// keep it in case we need to generate more than 1 challenge at once.
	challengeID := encryption.Hash(hashSeed + "1")

	seedSource, err := strconv.ParseUint(challengeID[0:16], 16, 64)
	if err != nil {
		return common.NewErrorf("generate_challenge",
			"Error in creating challenge seed: %v", err)
	}

	result, err := sc.populateGenerateChallenge(
		challengeReadyParts,
		int64(seedSource),
		validators,
		t,
		challengeID,
		balances)
	if err != nil {
		return common.NewErrorf("adding_challenge_error", err.Error())
	}

	err = sc.addChallenge(result.alloc,
		result.storageChallenge,
		result.allocChallenges,
		result.blobberChallenges,
		result.challInfo,
		balances)
	if err != nil {
		return common.NewErrorf("adding_challenge_error",
			"Error in adding challenge: %v", err)
	}

	return nil
}

func (sc *StorageSmartContract) addChallenge(alloc *StorageAllocation,
	challenge *StorageChallenge,
	allocChallenges *AllocationChallenges,
	blobChallenges *BlobberChallenges,
	challInfo *StorageChallengeResponse,
	balances cstate.StateContextI) error {

	if challenge.BlobberID == "" {
		return common.NewError("add_challenge",
			"no blobber to add challenge to")
	}

	blobAlloc, ok := alloc.BlobberAllocsMap[challenge.BlobberID]
	if !ok {
		return common.NewError("add_challenge",
			"no blobber Allocation to add challenge to")
	}

	// remove expired challenges
	expiredIDs, err := alloc.removeExpiredChallenges(allocChallenges, blobChallenges, challenge.Created)
	if err != nil {
		return common.NewErrorf("add_challenge", "remove expired challenges: %v", err)
	}

	// TODO: maybe delete them periodically later instead of remove immediately
	for _, id := range expiredIDs {
		_, err := balances.DeleteTrieNode(storageChallengeKey(sc.ID, id))
		if err != nil {
			return common.NewErrorf("add_challenge", "could not delete challenge node: %v", err)
		}
	}

	// add the generated challenge to the open challenges list in the allocation
	if !allocChallenges.addChallenge(challenge) {
		return common.NewError("add_challenge", "challenge already exist in allocation")
	}

	// save the allocation challenges to MPT
	if err := allocChallenges.Save(balances, sc.ID); err != nil {
		return common.NewErrorf("add_challenge",
			"error storing alloc challenge: %v", err)
	}

	if !blobChallenges.addChallenge(challenge.ID, challenge.Created) {
		return common.NewError("add_challenge", "challenge already exist in blobber")
	}

	if err := blobChallenges.save(balances, sc.ID); err != nil {
		return common.NewErrorf("add_challenge",
			"error storing blobber challenge: %v", err)
	}

	// save challenge to MPT
	if err := challenge.Save(balances, sc.ID); err != nil {
		return common.NewErrorf("add_challenge",
			"error storing challenge: %v", err)
	}

	alloc.Stats.OpenChallenges++
	alloc.Stats.TotalChallenges++
	blobAlloc.Stats.OpenChallenges++
	blobAlloc.Stats.TotalChallenges++

	if err := alloc.Save(balances, sc.ID); err != nil {
		return common.NewErrorf("add_challenge",
			"error storing allocation: %v", err)
	}

	if err := emitAddOrOverwriteAllocation(alloc, balances); err != nil {
		return common.NewErrorf("add_challenge",
			"saving allocation in db: %v", err)
	}

	err = emitAddChallenge(challInfo, balances)
	if err != nil {
		return common.NewError("add_challenge",
			"error adding challenge to db: "+err.Error())
	}

	return nil
}
