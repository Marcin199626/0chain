package cases

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"0chain.net/conductor/conductrpc/stats"
)

type (
	// StateNodesRequestor represents implementation of the TestCase interface.
	StateNodesRequestor struct {
		clientStats *stats.NodesClientStats

		roundInfo *RoundInfo

		caseType StateNodesRequestorCaseType

		wg *sync.WaitGroup
	}

	// StateNodesRequestorCaseType represents type that determines test behavior.
	StateNodesRequestorCaseType int
)

const (
	// todo
	SNRNoReplies StateNodesRequestorCaseType = iota
)

var (
	// Ensure StateNodesRequestor implements TestCase interface.
	_ TestCase = (*StateNodesRequestor)(nil)
)

// NewStateNodesRequestor creates initialised StateNodesRequestor.
func NewStateNodesRequestor(clientStatsCollector *stats.NodesClientStats, caseType StateNodesRequestorCaseType) *StateNodesRequestor {
	wg := new(sync.WaitGroup)
	wg.Add(2)
	return &StateNodesRequestor{
		clientStats: clientStatsCollector,
		caseType:    caseType,
		wg:          wg,
	}
}

// Check implements TestCase interface.
func (n *StateNodesRequestor) Check(ctx context.Context) (success bool, err error) {
	prepared := make(chan struct{})
	go func() {
		n.wg.Wait()
		prepared <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return false, errors.New("cases state is not prepared, context is done")

	case <-prepared:
		return n.check()
	}
}

func (n *StateNodesRequestor) check() (success bool, err error) {
	switch n.caseType {
	case SNRNoReplies:
		return n.checkRetryRequesting(2)

	default:
		panic("unknown case type")
	}
}

func (n *StateNodesRequestor) checkRetryRequesting(minRequests int) (success bool, err error) {
	replica0 := n.roundInfo.getNodeID(false, 0)
	replica0Stats, ok := n.clientStats.BlockStateChange[replica0]
	if !ok {
		return false, errors.New("no reports from replica0")
	}

	if len(n.roundInfo.NotarisedBlocks) != 1 {
		return false, errors.New("expected 1 notarised block")
	}

	notBlock := n.roundInfo.NotarisedBlocks[0]
	numReports := replica0Stats.CountWithHash(notBlock.Hash)
	if numReports < minRequests {
		return false, fmt.Errorf("insufficient reports count: %d", numReports)
	}
	return true, nil
}

// Configure implements TestCase interface.
func (n *StateNodesRequestor) Configure(blob []byte) error {
	panic("not implemented")
}

// AddResult implements TestCase interface.
func (n *StateNodesRequestor) AddResult(blob []byte) error {
	defer n.wg.Done()
	n.roundInfo = new(RoundInfo)
	return n.roundInfo.Decode(blob)
}
