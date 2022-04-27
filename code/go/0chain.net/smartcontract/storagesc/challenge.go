package storagesc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"

	"0chain.net/smartcontract/dbs"
	"0chain.net/smartcontract/dbs/event"
	"0chain.net/smartcontract/stakepool/spenum"

	"0chain.net/smartcontract/partitions"

	"0chain.net/chaincore/block"
	c_state "0chain.net/chaincore/chain/state"
	"0chain.net/chaincore/state"
	"0chain.net/chaincore/transaction"
	"0chain.net/core/common"
	"0chain.net/core/datastore"
	"0chain.net/core/encryption"
	. "0chain.net/core/logging"
	"0chain.net/core/util"

	"go.uber.org/zap"
)

const blobberChallengeAllocationPartitionSize = 100

func (sc *StorageSmartContract) completeChallengeForBlobber(
	allocation *StorageAllocation, challengeOnChain *StorageChallenge,
	challengeResponse *ChallengeResponse, blobber *StorageNode) bool {

	id := -1
	for i := range allocation.Challenges {
		if allocation.Challenges[i].BlobberID == challengeOnChain.BlobberID {
			if allocation.Challenges[i].ID == challengeResponse.ID {
				if challengeResponse != nil {
					challengeOnChain.Responded = true
				}
				blobber.LatestCompletedChallenge = challengeOnChain
			}
			id = i
			// break as we got the 1st open challenge for blobber
			break
		}
	}
	if id != -1 {
		if id == len(allocation.Challenges) {
			allocation.Challenges = allocation.Challenges[:id]
		} else {
			allocation.Challenges = append(allocation.Challenges[:id], allocation.Challenges[id+1:]...)
		}
		delete(allocation.ChallengeIDMap, challengeOnChain.ID)
	}

	return id > -1
}

func (sc *StorageSmartContract) getStorageChallenge(challengeID string,
	balances c_state.StateContextI) (challenge *StorageChallenge, err error) {

	challenge = new(StorageChallenge)
	challenge.ID = challengeID
	err = balances.GetTrieNode(challenge.GetKey(sc.ID), challenge)
	if err != nil {
		return nil, err
	}

	return challenge, nil
}

// move tokens from challenge pool to blobber's stake pool (to unlocked)
func (sc *StorageSmartContract) blobberReward(t *transaction.Transaction,
	alloc *StorageAllocation, prev common.Timestamp, blobber *StorageNode,
	details *BlobberAllocation, validators []string, partial float64,
	balances c_state.StateContextI) (err error) {

	var conf *Config
	if conf, err = sc.getConfig(balances, true); err != nil {
		return fmt.Errorf("can't get SC configurations: %v", err.Error())
	}

	// time of this challenge
	var tp = blobber.LatestCompletedChallenge.Created

	if tp > alloc.Expiration+toSeconds(details.Terms.ChallengeCompletionTime) {
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
		move = float64(details.challenge(dtu, rdtu))
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
		err = cp.moveToWritePool(alloc, details.BlobberID, until, wp, state.Balance(back))
		if err != nil {
			return fmt.Errorf("moving partial challenge to write pool: %v", err)
		}
		alloc.MovedBack += state.Balance(back)
		details.Returned += state.Balance(back)
		// save the write pool
		if err = wp.save(sc.ID, alloc.Owner, balances); err != nil {
			return fmt.Errorf("can't save allocation's write pool: %v", err)
		}
	}

	var sp *stakePool
	if sp, err = sc.getStakePool(blobber.ID, balances); err != nil {
		return fmt.Errorf("can't get stake pool: %v", err)
	}

	err = sp.DistributeRewards(blobberReward, blobber.ID, spenum.Blobber, balances)
	if err != nil {
		return fmt.Errorf("can't move tokens to blobber: %v", err)
	}

	details.ChallengeReward += state.Balance(blobberReward)

	// validators' stake pools
	var vsps []*stakePool
	if vsps, err = sc.validatorsStakePools(validators, balances); err != nil {
		return
	}

	err = cp.moveToValidators(sc.ID, validatorsReward, validators, vsps, balances)
	if err != nil {
		return fmt.Errorf("rewarding validators: %v", err)
	}
	alloc.MovedToValidators += state.Balance(validatorsReward)

	// save validators' stake pools
	if err = sc.saveStakePools(validators, vsps, balances); err != nil {
		return
	}

	// save the pools
	if err = sp.save(sc.ID, blobber.ID, balances); err != nil {
		return fmt.Errorf("can't save sake pool: %v", err)
	}

	if err = cp.save(sc.ID, alloc.ID, balances); err != nil {
		return fmt.Errorf("can't save allocation's challenge pool: %v", err)
	}

	return
}

// obtain stake pools of given validators
func (ssc *StorageSmartContract) validatorsStakePools(
	validators []datastore.Key, balances c_state.StateContextI) (
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
	sps []*stakePool, balances c_state.StateContextI) (err error) {

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
	alloc *StorageAllocation, prev common.Timestamp, blobber *StorageNode,
	details *BlobberAllocation, validators []string,
	balances c_state.StateContextI) (err error) {

	var conf *Config
	if conf, err = sc.getConfig(balances, true); err != nil {
		return fmt.Errorf("can't get SC configurations: %v", err.Error())
	}

	// time of this challenge
	var tp = blobber.LatestCompletedChallenge.Created

	if tp > alloc.Expiration+toSeconds(details.Terms.ChallengeCompletionTime) {
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
		move = float64(details.challenge(dtu, rdtu))
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
	alloc.MovedToValidators += state.Balance(validatorsReward)

	// save validators' stake pools
	if err = sc.saveStakePools(validators, vsps, balances); err != nil {
		return
	}

	// move back to write pool
	var until = alloc.Until()
	err = cp.moveToWritePool(alloc, details.BlobberID, until, wp, state.Balance(move))
	if err != nil {
		return fmt.Errorf("moving failed challenge to write pool: %v", err)
	}
	alloc.MovedBack += state.Balance(move)
	details.Returned += state.Balance(move)

	// blobber stake penalty
	if conf.BlobberSlash > 0 && move > 0 &&
		state.Balance(conf.BlobberSlash*move) > 0 {

		var slash = state.Balance(conf.BlobberSlash * move)

		// load stake pool
		var sp *stakePool
		if sp, err = sc.getStakePool(blobber.ID, balances); err != nil {
			return fmt.Errorf("can't get blobber's stake pool: %v", err)
		}

		var move state.Balance
		move, err = sp.slash(alloc, details.BlobberID, until, wp, details.Offer(), slash, balances)
		if err != nil {
			return fmt.Errorf("can't move tokens to write pool: %v", err)
		}

		sp.TotalOffers -= move  // subtract the offer stake
		details.Penalty += move // penalty statistic

		// save stake pool
		if err = sp.save(sc.ID, blobber.ID, balances); err != nil {
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
	input []byte, balances c_state.StateContextI) (resp string, err error) {

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

	challOnChain, err := sc.getStorageChallenge(challResp.ID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge",
			"Cannot fetch the challenge with ID %s", challResp.ID)
	}

	var alloc *StorageAllocation
	alloc, err = sc.getAllocation(challOnChain.AllocationID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge",
			"can't get related allocation: %v", err)
	}
	if err = alloc.removeExpiredChallenges(t.CreationDate); err != nil {
		return "", common.NewErrorf("verify_challenge",
			"unable to remove expired challenges: %v", err)
	}

	var _, ok = alloc.ChallengeIDMap[challResp.ID]
	if !ok {
		return "", common.NewErrorf("verify_challenge",
			"Cannot find the challenge with ID %s", challResp.ID)
	}

	if challOnChain.BlobberID != t.ClientID {
		return "", common.NewError("verify_challenge",
			"Challenge response should be submitted by the same blobber"+
				" as the challenge request")
	}

	details, ok := alloc.BlobberMap[t.ClientID]
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

	blobber, err := sc.getBlobber(t.ClientID, balances)
	if err != nil {
		return "", common.NewErrorf("verify_challenge",
			"Cannot fetch blobber")
	}

	// time of previous complete challenge (not the current one)
	// or allocation start time if no challenges
	var prev = alloc.StartTime
	if blobber.LatestCompletedChallenge != nil {
		prev = blobber.LatestCompletedChallenge.Created
	}

	var (
		threshold = challOnChain.TotalValidators / 2
		pass      = success > threshold ||
			(success > failure && success+failure < threshold)
		cct         = toSeconds(details.Terms.ChallengeCompletionTime)
		fresh       = challOnChain.Created+cct >= t.CreationDate
		enoughFails = failure > threshold ||
			(success+failure) == challOnChain.TotalValidators
		response string
	)

	// verification, or partial verification
	if pass && fresh {

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
		}

		var brStats BlobberRewardNode
		if err := ongoingParts.GetItem(balances, blobber.RewardPartition.Index, blobber.ID, &brStats); err != nil {
			return "", common.NewError("verify_challenge",
				"can't get blobber reward from partition list: "+err.Error())
		}

		brStats.SuccessChallenges++

		completed := sc.completeChallengeForBlobber(alloc, challOnChain, &challResp, blobber)
		if !completed {
			return "", common.NewError("challenge_out_of_order",
				"First challenge on the list is not same as the one"+
					" attempted to redeem")
		}
		alloc.Stats.ChallengeSuccessful(challOnChain.ID)
		details.Stats.ChallengeSuccessful(challOnChain.ID)

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

		var partial = 1.0
		if success < threshold {
			partial = float64(success) / float64(threshold)
		}

		err = sc.blobberReward(t, alloc, prev, blobber, details,
			validators, partial, balances)
		if err != nil {
			return "", common.NewError("challenge_reward_error", err.Error())
		}

		if success < threshold {
			response = "challenge passed partially by blobber"
		} else {
			response = "challenge passed by blobber"
		}
	} else if enoughFails || (pass && !fresh) {

		completed := sc.completeChallengeForBlobber(alloc, challOnChain, &challResp, blobber)
		if !completed {
			return "", common.NewError("challenge_out_of_order",
				"First challenge on the list is not same as the one"+
					" attempted to redeem")
		}
		alloc.Stats.ChallengeFailed(challOnChain.ID)
		details.Stats.ChallengeFailed(challOnChain.ID)

		Logger.Info("Challenge failed", zap.Any("challenge", challResp.ID))

		err = sc.blobberPenalty(t, alloc, prev, blobber, details,
			validators, balances)
		if err != nil {
			return "", common.NewError("challenge_penalty_error", err.Error())
		}

		if pass && !fresh {
			response = "late challenge (failed)"
		} else {
			response = "Challenge Failed by Blobber"
		}
	}

	_, err = balances.InsertTrieNode(challOnChain.GetKey(sc.ID), challOnChain)
	if err != nil {
		return "", common.NewError("verify_challenge_error", err.Error())
	}

	err = emitUpdateChallengeResponse(challOnChain.ID, challOnChain.Responded, balances)
	if err != nil {
		return "", common.NewError("verify_challenge_error", err.Error())
	}

	// save allocation object
	_, err = balances.InsertTrieNode(alloc.GetKey(sc.ID), alloc)
	if err != nil {
		return "", common.NewError("challenge_reward_error", err.Error())
	}

	_, err = balances.InsertTrieNode(blobber.GetKey(sc.ID), blobber)
	if err != nil {
		return "", common.NewError("verify_challenge",
			"error inserting blobber to chain"+err.Error())
	}

	err = emitUpdateBlobber(blobber, balances)
	if err != nil {
		return "", common.NewErrorf("challenge_response_error",
			"updating blobber in db: %v", err)
	}

	err = emitAddOrOverwriteAllocation(alloc, balances)
	if err != nil {
		return "", common.NewErrorf("challenge_reward_error",
			"saving allocation in db: %v", err)
	}
	if len(response) > 0 {
		return response, nil
	}
	return "", common.NewError("not_enough_validations",
		"Not enough validations, no successful validations")
}

func (sc *StorageSmartContract) getAllocationForChallenge(
	t *transaction.Transaction,
	allocID string,
	balances c_state.StateContextI) (alloc *StorageAllocation, err error) {

	alloc, err = sc.getAllocation(allocID, balances)
	switch err {
	case nil:
	case util.ErrValueNotPresent:
		Logger.Error("client state has invalid allocations",
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

type challengeInput struct {
	cr          *rand.Rand
	challengeID string
}

type challengeOutput struct {
	alloc            *StorageAllocation
	storageChallenge *StorageChallenge
	blobberAlloc     *BlobberAllocation
	challengeInfo    *StorageChallengeInfo
	error            error
}

func (sc *StorageSmartContract) populateGenerateChallenge(
	blobberChallengeList *partitions.Partitions,
	challengeSeed int64,
	validators *partitions.Partitions,
	t *transaction.Transaction,
	challengeID string,
	balances c_state.StateContextI,
) (*challengeOutput, error) {

	challRand := rand.New(rand.NewSource(challengeSeed))

	var blobberChallenges []BlobberChallengeNode
	err := blobberChallengeList.GetRandomItems(balances, challRand, &blobberChallenges)
	if err != nil {
		return nil, common.NewError("generate_challenges",
			"error getting random slice from blobber challenge partition")
	}

	randomIndex := challRand.Intn(len(blobberChallenges))
	bcItem := blobberChallenges[randomIndex]
	Logger.Debug("generate_challenges", zap.Int("random index", randomIndex),
		zap.String("blobber id", bcItem.BlobberID), zap.Int("blobber challenges", len(blobberChallenges)))

	blobberID := bcItem.BlobberID
	if blobberID == "" {
		return nil, common.NewError("add_challenges",
			"empty blobber id")
	}

	bcAllocList, err := getBlobbersChallengeAllocationList(blobberID, balances)
	if err != nil {
		return nil, common.NewError("generate_challenges",
			"error getting blobber_challenge_allocation list: "+err.Error())
	}

	// maybe we should use another random seed
	var bcAllocPartition []BlobberChallengeAllocationNode
	if err := bcAllocList.GetRandomItems(balances, challRand, &bcAllocPartition); err != nil {
		return nil, common.NewErrorf("generate_challenges",
			"error getting random slice from blobber challenge allocation partition, %v", err)
	}

	randomIndex = challRand.Intn(len(bcAllocPartition))
	bcAllocItem := bcAllocPartition[randomIndex]

	allocID := bcAllocItem.ID

	alloc, err := sc.getAllocationForChallenge(t, allocID, balances)
	if err != nil {
		return nil, err
	}

	if alloc == nil {
		return nil, errors.New("empty allocation")
	}

	blobberAllocation, ok := alloc.BlobberMap[blobberID]
	if !ok {
		return nil, common.NewError("add_challenges",
			"blobber allocation doesn't exists in allocation")
	}

	if blobberAllocation.Stats == nil {
		blobberAllocation.Stats = new(StorageAllocationStats)
	}

	selectedValidators := make([]*ValidationNode, 0)
	var randValidators []ValidationPartitionNode
	if err := validators.GetRandomItems(balances, challRand, &randValidators); err != nil {
		return nil, common.NewError("add_challenge",
			"error getting validators random slice: "+err.Error())
	}

	perm := challRand.Perm(len(randValidators))
	for i := 0; i < minInt(len(randValidators), alloc.DataShards+1); i++ {
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
	storageChallenge.Created = t.CreationDate

	challInfo := &StorageChallengeInfo{
		ID:             challengeID,
		Created:        t.CreationDate,
		Validators:     selectedValidators,
		RandomNumber:   challengeSeed,
		AllocationID:   allocID,
		AllocationRoot: blobberAllocation.AllocationRoot,
		BlobberID:      blobberID,
	}

	return &challengeOutput{
		alloc:            alloc,
		storageChallenge: storageChallenge,
		blobberAlloc:     blobberAllocation,
		challengeInfo:    challInfo,
	}, nil
}

func (sc *StorageSmartContract) generateChallenge(t *transaction.Transaction,
	b *block.Block, _ []byte, balances c_state.StateContextI) (err error) {

	hashString := encryption.Hash(t.Hash + b.PrevHash)

	validators, err := getValidatorsList(balances)
	if err != nil {
		return common.NewErrorf("generate_challenge",
			"error getting the validators list: %v", err)
	}

	blobberChallengeList, err := getBlobbersChallengeList(balances)
	if err != nil {
		return common.NewErrorf("generate_challenge",
			"error getting the blobber challenge list: %v", err)
	}
	if listSize, err := blobberChallengeList.Size(balances); err == nil && listSize == 0 {
		Logger.Info("skipping generate challenge: empty blobber challenge partition")
		return nil
	}

	challengeID := encryption.Hash(hashString + strconv.FormatInt(1, 10))
	var challengeSeed uint64
	challengeSeed, err = strconv.ParseUint(challengeID[0:16], 16, 64)
	if err != nil {
		return common.NewErrorf("generate_challenge",
			"Error in creating challenge seed: %v", err)
	}

	result, err := sc.populateGenerateChallenge(
		blobberChallengeList,
		int64(challengeSeed),
		validators,
		t,
		challengeID,
		balances)
	if err != nil {
		return common.NewErrorf("adding_challenge_error", err.Error())
	}

	var alloc = result.alloc
	_, err = sc.addChallenge(
		alloc,
		result.storageChallenge,
		result.challengeInfo,
		t.CreationDate,
		balances,
	)
	if err != nil {
		return common.NewErrorf("adding_challenge_error",
			"Error in adding challenge: %v", err)
	}

	return nil
}

func (sc *StorageSmartContract) addChallenge(
	alloc *StorageAllocation,
	storageChallenge *StorageChallenge,
	challInfo *StorageChallengeInfo,
	now common.Timestamp,
	balances c_state.StateContextI) (resp string, err error) {

	if storageChallenge.BlobberID == "" {
		return "", common.NewError("add_challenge",
			"no blobber to add challenge to")
	}

	if _, ok := alloc.BlobberMap[storageChallenge.BlobberID]; !ok {
		return "", common.NewError("add_challenge",
			"no blobber Allocation to add challenge to")
	}
	blobberAllocation := alloc.BlobberMap[storageChallenge.BlobberID]
	if err = alloc.removeExpiredChallenges(now); err != nil {
		return "", common.NewError("add_challenge",
			"error removing expired challenges: "+err.Error())
	}

	blobber, err := sc.getBlobber(storageChallenge.BlobberID, balances)
	if err != nil {
		return "", common.NewError("add_challenge",
			"cannot fetch blobber")
	}

	if ok := alloc.addChallenge(storageChallenge, blobber); !ok {
		Logger.Warn("add_challenge",
			zap.Error(errors.New("no challenge added, challenge might already exist in allocation")))
		challengeBytes, err := json.Marshal(storageChallenge)
		return string(challengeBytes), err
	}
	challInfo.PrevID = storageChallenge.PrevID

	_, err = balances.InsertTrieNode(storageChallenge.GetKey(sc.ID), storageChallenge)
	if err != nil {
		return "", common.NewError("add_challenge",
			"error storing challenge: "+err.Error())
	}

	err = emitAddOrOverwriteChallenge(challInfo, balances)
	if err != nil {
		return "", common.NewError("add_challenge",
			"error adding challenge to db: "+err.Error())
	}

	alloc.Stats.OpenChallenges++
	alloc.Stats.TotalChallenges++
	blobberAllocation.Stats.OpenChallenges++
	blobberAllocation.Stats.TotalChallenges++

	_, err = balances.InsertTrieNode(alloc.GetKey(sc.ID), alloc)
	if err != nil {
		return "", common.NewError("add_challenge",
			"error storing allocation: "+err.Error())
	}

	err = emitAddOrOverwriteAllocation(alloc, balances)
	if err != nil {
		return "", common.NewErrorf("add_challenge",
			"saving allocation in db: %v", err)
	}

	_, err = balances.InsertTrieNode(blobber.GetKey(sc.ID), blobber)
	if err != nil {
		return "", common.NewError("add_challenge",
			"error storing blobber: "+err.Error())
	}

	err = emitUpdateBlobber(blobber, balances)
	if err != nil {
		return "", common.NewErrorf("add_challenge",
			"updating blobber in db: %v", err)
	}

	challengeBytes, err := json.Marshal(storageChallenge)
	return string(challengeBytes), err
}
