//go:build integration_tests
// +build integration_tests

package chain

import (
	"context"
	"log"

	"0chain.net/chaincore/block"
	"0chain.net/chaincore/node"
	crpc "0chain.net/conductor/conductrpc"
	"0chain.net/core/util"
)

func (c *Chain) syncRoundStateToStateDB(ctx context.Context, round int64, rootStateHash util.Key) {
	invalidated := invalidateMPTNodesIfNeeded(round, rootStateHash)

	syncRoundStateToStateDB(ctx, round, rootStateHash, invalidated)
}

//func invalidateMPTNodesIfNeeded(roundNum int64, rootStateHash util.Key) {
//	cfg := crpc.Client().State().StateNodesRequestor
//
//	cfg.Lock()
//	defer cfg.Unlock()
//
//	sChain := GetServerChain()
//	mi := cases.NewMinerInformer(
//		sChain.GetRound(roundNum),
//		sChain.GetMiners(roundNum),
//		sChain.GetGeneratorsNum(),
//	)
//	isReplica0 := !mi.IsGenerator(node.Self.ID) && mi.GetTypeRank(node.Self.ID) == 0
//	if roundNum == 30 {
//		log.Printf("invalidating, mi %#v", mi) //
//	}
//	if cfg == nil || roundNum != cfg.OnRound || cfg.Prepared || !isReplica0 {
//		return
//	}
//
//	mpt := util.NewMerklePatriciaTrie(sChain.stateDB, util.Sequence(roundNum), rootStateHash)
//	handler := func(ctx context.Context, path util.Path, key util.Key, node util.Node) error {
//		v := reflect.ValueOf(node)
//		v.Elem().Set(reflect.Zero(v.Elem().Type()))
//		return nil
//	}
//	if err := mpt.Iterate(context.Background(), handler, util.NodeTypeLeafNode|util.NodeTypeFullNode|util.NodeTypeExtensionNode); err != nil {
//		log.Panicf("Condcutor: error wile iterating mpt: %v", err)
//	}
//
//	cfg.Prepared = true
//	log.Printf("prepared") // todo
//}

func invalidateMPTNodesIfNeeded(roundNum int64, rootStateHash util.Key) (invalidated bool) {
	cfg := crpc.Client().State().StateNodesRequestor

	cfg.Lock()
	defer cfg.Unlock()

	sChain := GetServerChain()
	mi := createMIByBlock(sChain.getBlockByRound(roundNum - 1))
	isReplica0 := !mi.IsGenerator(node.Self.ID) && mi.GetTypeRank(node.Self.ID) == 0
	if cfg == nil || roundNum-1 != cfg.OnRound || cfg.Prepared || !isReplica0 {
		return
	}

	if err := sChain.stateDB.PruneBelowVersion(context.Background(), util.Sequence(roundNum-1)); err != nil {
		log.Panicf("%v", err) // todo
	}

	//mpt := util.NewMerklePatriciaTrie(sChain.stateDB, util.Sequence(roundNum), rootStateHash)
	//invalidated := false
	//handler := func(ctx context.Context, key util.Key, node util.Node) error {
	//	if !invalidated && int64(node.GetOrigin()) == cfg.OnRound {
	//		v := reflect.ValueOf(node)
	//		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	//
	//		log.Printf("invalidated") // todo
	//	}
	//	return nil
	//}
	//if err := sChain.stateDB.Iterate(context.Background(), handler); err != nil {
	//	log.Panicf("Condcutor: error wile iterating mpt: %v", err)
	//}
	//if err := mpt.Iterate(context.Background(), handler, util.NodeTypeLeafNode|util.NodeTypeFullNode|util.NodeTypeExtensionNode); err != nil {
	//	log.Panicf("Condcutor: error wile iterating mpt: %v", err)
	//}

	cfg.Prepared = true
	log.Printf("prepared") // todo
	return true
}

func (c *Chain) getBlockByRound(round int64) *block.Block {
	for _, b := range c.blocks {
		if b.Round == round {
			return b
		}
	}
	return nil
}
