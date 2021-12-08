package miner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"0chain.net/chaincore/block"
	"0chain.net/chaincore/round"
	"0chain.net/chaincore/threshold/bls"
	"0chain.net/core/common"
	"0chain.net/core/logging"
	"go.uber.org/zap"
)

// HandleVRFShare - handles the vrf share.
func (mc *Chain) HandleVRFShare(ctx context.Context, msg *BlockMessage) {

	var mr = mc.getOrStartRoundNotAhead(ctx, msg.VRFShare.Round)
	if mr == nil {
		return
	}

	// add the VRFShare
	logging.Logger.Debug("handle vrf share",
		zap.Int64("round", msg.VRFShare.Round),
		zap.Int("vrf_timeout_count", msg.VRFShare.GetRoundTimeoutCount()),
		zap.Int("sender_index", msg.Sender.SetIndex),
	)
	mc.AddVRFShare(ctx, mr, msg.VRFShare)
}

// HandleVerifyBlockMessage - handles the verify block message.
func (mc *Chain) HandleVerifyBlockMessage(ctx context.Context,
	msg *BlockMessage) {

	var (
		b         = msg.Block
		vrfShares = msg.VRFShares
	)

	if err := mc.mergeBlockVRFShares(ctx, b, vrfShares); err != nil {
		logging.Logger.Error("handle verify block - failed to merge vrf shares",
			zap.Int64("round", b.Round),
			zap.String("block", b.Hash),
			zap.Error(err))
		return
	}

	if err := mc.pushToBlockVerifyWorker(ctx, b); err != nil {
		logging.Logger.Error("handle verify block - push to block verify worker failed",
			zap.Int64("round", b.Round),
			zap.String("block", b.Hash),
			zap.Error(err))
		return
	}
}

func (mc *Chain) mergeBlockVRFShares(ctx context.Context, b *block.Block, vrfShares map[string]*round.VRFShare) error {
	// merge vrf shares requests one by one to avoid duplicate share verification
	return mc.mergeBlockVRFSharesWorker.Run(ctx, func() error {
		var (
			mb             = mc.GetMagicBlock(b.Round)
			blsThreshold   = mb.T
			mr             = mc.GetMinerRound(b.Round)
			newVRFShares   = make(map[string]*round.VRFShare)
			localVRFShares = make(map[string]*round.VRFShare)
			vrfSharesNum   int
		)

		if len(vrfShares) < blsThreshold {
			return errors.New("vrf shares of block not reached threshold")
		}

		if mr == nil {
			newVRFShares = vrfShares
			mr = mc.getOrStartRoundNotAhead(ctx, b.Round)
		} else {
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			completed, err := mc.isVRFComplete(cctx, b.Round, b.RoundRandomSeed)
			if err != nil {
				// round VRF shares already reached threshold
				return err
			}

			if completed {
				return nil
			}

			localVRFShares = mr.GetVRFShares()
			vrfSharesNum = len(localVRFShares)
			if vrfSharesNum < blsThreshold {
				for partyID, vrfs := range vrfShares {
					if _, ok := localVRFShares[partyID]; ok {
						continue
					}
					newVRFShares[partyID] = vrfs
				}
			}
		}

		dkg := mc.GetDKG(b.Round)

		for id, vrfs := range newVRFShares {
			mr.AddTimeoutVote(vrfs.GetRoundTimeoutCount(), id)
			msg, err := mc.GetBlsMessageForRound(mr.Round)
			if err != nil {
				return errors.New("failed to get bls message")
			}

			var share bls.Sign
			if err := share.SetHexString(vrfs.Share); err != nil {
				return fmt.Errorf("failed to decode share string, share: %s", vrfs.Share)
			}

			nd := mb.Miners.GetNode(id)
			if nd == nil {
				return fmt.Errorf("could not find node in magic block, mb_starting_round: %v, id: %v",
					mb.StartingRound, id)
			}

			vrfs.SetParty(nd)

			partyID := bls.ComputeIDdkg(id)
			if !dkg.VerifySignature(&share, msg, partyID) {
				return fmt.Errorf("failed to verify vrf share signature, id: %s, bls_msg: %s, share: %s",
					id, msg, vrfs.Share)
			}

			mr.AddVRFShare(newVRFShares[id], blsThreshold)
			vrfSharesNum++
			logging.Logger.Debug("handle verify block - added vrf_share",
				zap.Int64("round", b.Round),
				zap.String("block", b.Hash),
				zap.Int("vrf_share_num", vrfSharesNum))
			if vrfSharesNum >= blsThreshold {
				if mc.ThresholdNumBLSSigReceived(ctx, mr, blsThreshold) {
					mc.StartVerification(ctx, mr)
				}
				return nil
			}
		}

		return nil
	})
}

func (mc *Chain) isVRFComplete(ctx context.Context, r int64, rrs int64) (bool, error) {
	var (
		mb           = mc.GetMagicBlock(r)
		blsThreshold = mb.T
		mr           = mc.GetMinerRound(r)
	)

	if mr == nil {
		return false, fmt.Errorf("round not started yet, round: %v", r)
	}

	vrfShares := mr.GetVRFShares()
	if len(vrfShares) >= blsThreshold {
		roundRRS := mr.GetRandomSeed()
		if roundRRS == 0 {
			ts := time.Now()
			err := func() error {
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(100 * time.Millisecond):
						// wait for the computing of RRS, the RRS could be 0 when the vrf shares
						// just meet the threshold and not start to compute the RRS yet.
						if mr.IsVRFComplete() {
							roundRRS = mr.GetRandomSeed()
							return nil
						}
					}
				}
			}()

			if err == context.DeadlineExceeded {
				return false, nil
			}

			logging.Logger.Debug("round is vrf ready after waiting for",
				zap.Duration("duration", time.Since(ts)),
				zap.Int64("round", r),
				zap.Int64("round_rrs", roundRRS))
		}

		if roundRRS == rrs {
			return true, nil
		}
		return false, fmt.Errorf("RRS does not match, round_rrs: %d, block_rrs: %d", roundRRS, rrs)
	}

	return false, nil
}

func (mc *Chain) pushToBlockVerifyWorker(ctx context.Context, b *block.Block) error {
	select {
	case mc.blockVerifyC <- b:
		return nil
	case <-time.NewTimer(500 * time.Millisecond).C:
		return errors.New("push to channel timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BlockVerifyWorkers starts the workers for processing 'verify block' messages
func (mc *Chain) BlockVerifyWorkers(ctx context.Context) {
	// TODO: make the worker number configurable
	workerNum := 4
	wg := sync.WaitGroup{}
	for i := 0; i < workerNum; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case b := <-mc.blockVerifyC:
					ts := time.Now()
					if err := mc.processVerifyBlockWithTimeout(ctx, b, 3*time.Second); err != nil {
						logging.Logger.Error("process verify block failed",
							zap.Int64("round", b.Round),
							zap.String("block", b.Hash),
							zap.Any("duration", time.Since(ts)),
							zap.Error(err))
						continue
					}
					logging.Logger.Debug("verify block processed",
						zap.Int64("round", b.Round),
						zap.String("block", b.Hash))
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	wg.Wait()
}

func (mc *Chain) processVerifyBlockWithTimeout(ctx context.Context, b *block.Block, timeout time.Duration) error {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errC := make(chan error, 1)
	doneC := make(chan struct{})
	go func() {
		err := mc.processVerifyBlock(cctx, b)
		if err != nil {
			errC <- err
			return
		}
		close(doneC)
	}()

	select {
	case err := <-errC:
		return err
	case <-doneC:
		return nil
	case <-cctx.Done():
		return cctx.Err()
	}
}

func (mc *Chain) processVerifyBlock(ctx context.Context, b *block.Block) error {
	logging.Logger.Debug("verify block",
		zap.Int64("round", b.Round),
		zap.String("block", b.Hash))

	if err := b.Validate(ctx); err != nil {
		logging.Logger.Debug("verify block - can't validate",
			zap.Int64("round", b.Round), zap.Error(err))
		return err
	}

	if b.Round < mc.GetCurrentRound()-1 {
		logging.Logger.Debug("verify block - round mismatch",
			zap.Int64("current_round", mc.GetCurrentRound()),
			zap.Int64("block_round", b.Round))
		return nil
	}

	// get previous block notarization tickets, and update local prev block if exist
	if b.Round > 1 {
		// TODO: run in gorountine for debug and test purpose
		// do not run this in goroutine
		//
		// put into a goroutine so that tickets verification would not affect the
		// new round RRS generation
		go func() {
			// TODO: check if the block's prev notarized block reached the notarization threshold
			pr := mc.GetMinerRound(b.Round - 1)
			cctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := mc.updatePreviousBlockNotarization(cctx, b, pr); err != nil {
				return
			}
		}()
	}

	mr := mc.GetMinerRound(b.Round)
	if mr == nil {
		logging.Logger.Error("verify block - got block proposal before starting round",
			zap.Int64("round", b.Round), zap.String("block", b.Hash),
			zap.String("miner", b.MinerID))

		mr = mc.getOrStartRoundNotAhead(ctx, b.Round)
		if mr == nil {
			logging.Logger.Error("verify block - can't start new round",
				zap.Int64("round", b.Round))
			return nil
		}

		//mc.startRound(ctx, mr, b.GetRoundRandomSeed())

		mc.AddToRoundVerification(ctx, mr, b)
		return nil
	}

	if !mr.IsVRFComplete() {
		logging.Logger.Info("verify block - got block proposal before VRF is complete",
			zap.Int64("round", b.Round), zap.String("block", b.Hash),
			zap.String("miner", b.MinerID))

		if mr.GetTimeoutCount() < b.RoundTimeoutCount {
			logging.Logger.Info("verify block - ignoring, round timout count < block round timeout count",
				zap.Int64("round", b.Round), zap.String("block", b.Hash),
				zap.String("miner", b.MinerID),
				zap.Int("round_toc", mr.GetTimeoutCount()),
				zap.Int("round_toc", b.RoundTimeoutCount))
			return nil
		}

		if b.GetRoundRandomSeed() != mr.GetRandomSeed() {
			logging.Logger.Info("verify block - got block with different RRS",
				zap.Int64("round", b.Round),
				zap.Int64("block RRS", b.GetRoundRandomSeed()),
				zap.Int64("round RRS", mr.GetRandomSeed()))
			//mc.startRound(ctx, mr, b.GetRoundRandomSeed())
		}
	}

	vts := mr.GetVerificationTickets(b.Hash)
	if len(vts) == 0 {
		mc.AddToRoundVerification(ctx, mr, b)
		return nil
	}

	// TODO: mc.MergeVerificationTickets does not verify block's own tickets, might be a problem!
	mc.MergeVerificationTickets(b, vts)
	if !b.IsBlockNotarized() {
		mc.AddToRoundVerification(ctx, mr, b)
		return nil
	}

	if mr.GetRandomSeed() == b.GetRoundRandomSeed() {
		b = mc.AddRoundBlock(mr, b)
		mc.checkBlockNotarization(ctx, mr, b, true)
		return nil
	}

	/* Since this is a notarized block, we are accepting it. */
	b1, r1, err := mc.AddNotarizedBlockToRound(mr, b)
	if err != nil {
		logging.Logger.Error("verify block failed",
			zap.Int64("round", b.Round),
			zap.String("block", b.Hash),
			zap.String("miner", b.MinerID),
			zap.Error(err))
		return nil
	}

	b = b1
	mr = r1.(*Round)
	logging.Logger.Info("verify block - added a notarizedBlockToRound, got notarized block with different RRS",
		zap.Int64("round", b.Round),
		zap.String("block", b.Hash),
		zap.String("miner", b.MinerID),
		zap.Int("round_toc", mr.GetTimeoutCount()),
		zap.Int("round_toc", b.RoundTimeoutCount))

	mc.checkBlockNotarization(ctx, mr, b, true)
	return nil
}

func (mc *Chain) verifyTicketsWithRetry(ctx context.Context,
	r int64, block string, bvts []*block.VerificationTicket, retryN int) error {
	for i := 0; i < retryN; i++ {
		err := func() error {
			logging.Logger.Debug("verification ticket",
				zap.Int64("round", r),
				zap.String("block", block),
				zap.Int("retry", i))
			cctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			return mc.VerifyTickets(cctx, block, bvts, r)
		}()

		switch err {
		case nil:
			return nil
		case context.DeadlineExceeded:
			if mc.GetCurrentRound() > r {
				return common.NewErrorf("verify_tickets_timeout", "chain moved on, round: %d", r)
			}
		default:
			logging.Logger.Error("verification ticket failed",
				zap.Int64("round", r),
				zap.Error(err))
			return err
		}
	}

	return common.NewErrorf("verify_tickets_timeout", "ticket timeout with retry, round: %d", r)
}

// HandleVerificationTicketMessage - handles the verification ticket message.
func (mc *Chain) HandleVerificationTicketMessage(ctx context.Context,
	msg *BlockMessage) {

	var (
		bvt = msg.BlockVerificationTicket
		rn  = bvt.Round
		mr  = mc.GetMinerRound(rn)
	)

	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := mc.VerifyTickets(cctx, bvt.BlockID, []*block.VerificationTicket{&bvt.VerificationTicket}, rn); err != nil {
		logging.Logger.Error("handle vt. msg - verification ticket failed",
			zap.Error(err),
			zap.Int64("round", bvt.Round),
			zap.String("block", bvt.BlockID))
		return
	}

	b, err := mc.GetBlock(ctx, bvt.BlockID)
	if err != nil {
		logging.Logger.Debug("handle vt. msg - block does not exist, collect tickets though",
			zap.Int64("round", bvt.Round),
			zap.String("block", bvt.BlockID))

		mr.AddVerificationTickets([]*block.BlockVerificationTicket{bvt})
		return
	}

	mc.ProcessVerifiedTicket(ctx, mr, b, &bvt.VerificationTicket)
}

func (mc *Chain) isNotarizing(hash string) (notarizing bool) {
	mc.nbpMutex.Lock()
	_, notarizing = mc.notarizationBlockProcessMap[hash]
	mc.nbpMutex.Unlock()
	return
}

func (mc *Chain) processNotarization(ctx context.Context, not *Notarization) {
	mc.nbpMutex.Lock()
	if _, ok := mc.notarizationBlockProcessMap[not.BlockID]; ok {
		mc.nbpMutex.Unlock()
		return
	}

	mc.notarizationBlockProcessMap[not.BlockID] = struct{}{}
	mc.nbpMutex.Unlock()

	select {
	case mc.notarizationBlockProcessC <- not:
	case <-time.After(500 * time.Millisecond):
		logging.Logger.Warn("process notarization slow, push to channel timeout",
			zap.Int64("round", not.Round))
		mc.nbpMutex.Lock()
		delete(mc.notarizationBlockProcessMap, not.BlockID)
		mc.nbpMutex.Unlock()
	case <-ctx.Done():
	}
}

// NotarizationProcessWorker represents a worker to process notarization messages sequentially
func (mc *Chain) NotarizationProcessWorker(ctx context.Context) {
	for {
		select {
		case not := <-mc.notarizationBlockProcessC:
			func() {
				doneC := make(chan struct{})
				errC := make(chan error, 1)
				cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				ts := time.Now()
				go func() {
					if err := mc.notarizationProcess(cctx, not); err != nil {
						errC <- err
					}
					close(doneC)
				}()

				select {
				case err := <-errC:
					logging.Logger.Error("process notarization failed",
						zap.Int64("round", not.Round),
						zap.String("block", not.BlockID),
						zap.Error(err))
				case <-doneC:
					logging.Logger.Info("process notarization success",
						zap.Int64("round", not.Round),
						zap.String("block", not.BlockID),
						zap.Any("duration", time.Since(ts)))
				case <-cctx.Done():
					logging.Logger.Error("process notarization timeout",
						zap.Int64("round", not.Round),
						zap.String("block", not.BlockID))
				}
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (mc *Chain) notarizationProcess(ctx context.Context, not *Notarization) error {
	var (
		r    = mc.GetMinerRound(not.Round)
		b, _ = mc.GetBlock(ctx, not.BlockID)
	)

	if b == nil {
		// fetch from remote
		var err error
		b, err = mc.GetNotarizedBlock(ctx, not.BlockID, not.Round)
		if err != nil {
			return fmt.Errorf("fetch notarized block failed, err: %v", err)
		}
		r = mc.GetMinerRound(not.Round)
	}

	if !b.IsBlockNotarized() {
		var vts = b.UnknownTickets(not.VerificationTickets)
		if len(vts) == 0 {
			err := mc.VerifyBlockNotarization(ctx, b)
			if err != nil {
				return errors.New("no new tickets detected")
			}
		} else {
			logging.Logger.Debug("process notarization - merge notarization block",
				zap.Int64("round", b.Round),
				zap.String("block", b.Hash))
			if err := mc.MergeNotarization(ctx, r, b, vts); err != nil {
				return fmt.Errorf("merge notarization tickets failed, err: %v", err)
			}

			if !b.IsBlockNotarized() {
				logging.Logger.Error("process notarization - not notarized after merging!",
					zap.Int64("round", b.Round),
					zap.String("block", b.Hash),
					zap.Int("unknown tickets num", len(vts)),
					zap.Int("block tickets", len(b.GetVerificationTickets())))
				return fmt.Errorf("block is not notarized after merging tickets, "+
					"block tickets num: %v, unknown tickets num: %v", len(b.GetVerificationTickets()), len(vts))
			}
		}
	}

	if mc.GetCurrentRound() <= not.Round && !mc.isAheadOfSharders(ctx, not.Round) {
		logging.Logger.Info("process notarization - start next round",
			zap.Int64("new round", not.Round+1))

		go mc.StartNextRound(ctx, r)
	}

	if !b.IsStateComputed() {
		if err := mc.GetBlockStateChange(b); err != nil {
			return fmt.Errorf("process notarization - sync state changes failed, round: %d, err: %v", b.Round, err)
		}
	}

	// update LFB if the LFB is far away behind the LFB ticket(fetch from sharder)
	lfb := mc.GetLatestFinalizedBlock()
	if lfb == nil {
		return nil
	}

	if lfbTK := mc.GetLatestLFBTicket(ctx); lfbTK != nil && lfbTK.Round-lfb.Round >= int64(mc.PruneStateBelowCount/3) {
		if b.Round >= lfbTK.Round {
			// try to get LFB ticket block from local
			lfb, err := mc.GetBlock(ctx, lfbTK.LFBHash)
			if err != nil {
				// acquire from sharder
				logging.Logger.Debug("process notarization - ensure LFB from sharder",
					zap.Int64("round", b.Round),
					zap.Int64("LFB ticket round", lfbTK.Round),
					zap.String("LFB ticket block", lfbTK.LFBHash))
				_, err := mc.ensureLatestFinalizedBlock(ctx)
				return err
			}
			logging.Logger.Debug("process notarization - update LFB, round > tk round",
				zap.Int64("round", b.Round),
				zap.Int64("lfb round", lfb.Round),
				zap.Int64("LFB ticket round", lfbTK.Round),
				zap.String("LFB ticket block", lfbTK.LFBHash))
			mc.SetLatestFinalizedBlock(ctx, lfb)
			return nil
		}

		logging.Logger.Debug("process notarization - update LFB, round <= tk round",
			zap.Int64("round", b.Round),
			zap.Int64("lfb round", lfb.Round),
			zap.Int64("LFB ticket round", lfbTK.Round),
			zap.String("LFB ticket block", lfbTK.LFBHash))
		_, err := mc.ensureLatestFinalizedBlock(ctx)
		return err
	}

	return nil
}

// HandleNotarizationMessage - handles the block notarization message.
func (mc *Chain) HandleNotarizationMessage(ctx context.Context, msg *BlockMessage) {
	mc.processNotarization(ctx, msg.Notarization)
}

// HandleNotarizedBlockMessage - handles a notarized block for a previous round.
func (mc *Chain) HandleNotarizedBlockMessage(ctx context.Context,
	msg *BlockMessage) {

	nb := msg.Block

	var mr = mc.getOrStartRoundNotAhead(ctx, nb.Round)
	if mr == nil {
		logging.Logger.Debug("notarized block handler -- is ahead or no pr",
			zap.String("block", nb.Hash), zap.Any("round", nb.Round),
			zap.Bool("has_pr", mc.GetMinerRound(nb.Round-1) != nil))
		return // can't handle yet
	}

	if mr.GetRandomSeed() == 0 {
		mc.SetRandomSeed(mr, nb.GetRoundRandomSeed())
	}

	lfb := mc.GetLatestFinalizedBlock()
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := mc.verifyBlockNotarizationWorker.Run(cctx, func() error {
		return mc.VerifyBlockNotarization(ctx, nb)
	}); err != nil {
		logging.Logger.Error("handle notarized block",
			zap.Error(err),
			zap.Int64("round", nb.Round),
			zap.Int64("lfb_round", lfb.Round))
		return
	}

	if !mr.IsVRFComplete() {
		mc.startRound(ctx, mr, nb.GetRoundRandomSeed())
	}

	var b = mc.AddRoundBlock(mr, nb)
	if !mc.AddNotarizedBlock(ctx, mr, b) {
		return
	}

	if mc.isAheadOfSharders(ctx, nb.Round+1) {
		logging.Logger.Error("handle notarized block",
			zap.Error(errors.New("next round ahead of sharders")),
			zap.Int64("round", nb.Round),
			zap.Int64("lfb_round", lfb.Round))
		return
	}

	mc.StartNextRound(ctx, mr) // start next or skip
}
