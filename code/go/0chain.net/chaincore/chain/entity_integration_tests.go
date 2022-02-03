//go:build integration_tests
// +build integration_tests

package chain

import (
	"go.uber.org/zap"

	"0chain.net/chaincore/block"
	"0chain.net/chaincore/node"
	"0chain.net/chaincore/round"
	crpc "0chain.net/conductor/conductrpc"
	"0chain.net/conductor/config"
	"0chain.net/conductor/config/cases"
	"0chain.net/core/logging"
)

var myFailingRound int64 // once set, we ignore all restarts for that round

func (c *Chain) IsRoundGenerator(r round.RoundI, nd *node.Node) bool {

	var (
		rank          = r.GetMinerRank(nd)
		state         = crpc.Client().State()
		comp          bool
		numGenerators = c.GetGeneratorsNumOfRound(r.GetRoundNumber())
		is            = rank != -1 && rank < numGenerators
	)

	if is {
		// test if we have request to skip this round
		if r.GetRoundNumber() == myFailingRound {
			logging.Logger.Info("we're still pretending to be not a generator for round", zap.Int64("round", r.GetRoundNumber()))
			return false
		}
		if config.Round(r.GetRoundNumber()) == state.GeneratorsFailureRoundNumber && r.GetTimeoutCount() == 0 {
			logging.Logger.Info("we're a failing generator for round", zap.Int64("round", r.GetRoundNumber()))
			// remember this round as failing
			myFailingRound = r.GetRoundNumber()
			return false
		}
		return true // regular round generator
	}

	var competingBlock = state.CompetingBlock
	comp = competingBlock.IsCompetingRoundGenerator(state, nd.GetKey(),
		r.GetRoundNumber())

	if comp {
		return true // competing generator
	}

	return false // is not
}

func (c *Chain) SetLatestFinalizedBlock(b *block.Block) {
	nss := isNotifyingSyncState(b)
	c.setLatestFinalizedBlock(b, nss)
}

func isNotifyingSyncState(bl *block.Block) bool {
	if bl == nil || bl.Round == 0 {
		return false
	}

	cfg := crpc.Client().State().StateNodesRequestor
	if cfg == nil || bl.Round != cfg.OnRound {
		return false
	}

	mi := createMIByBlock(bl)
	isReplica0 := !mi.IsGenerator(node.Self.ID) && mi.GetTypeRank(node.Self.ID) == 0
	return !isReplica0
}

func createMIByBlock(bl *block.Block) cases.MinerInformer {
	sChain := GetServerChain()
	miners := sChain.GetMiners(bl.Round)

	roundI := round.NewRound(bl.Round)
	roundI.SetRandomSeed(bl.RoundRandomSeed, len(miners.Nodes))

	return cases.NewMinerInformer(roundI, miners, sChain.GetGeneratorsNum())
}
