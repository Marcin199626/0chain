package cases

import (
	"sync"

	"github.com/mitchellh/mapstructure"

	"0chain.net/conductor/cases"
	"0chain.net/conductor/conductrpc/stats"
)

type (
	// StateNodesRequestor represents TestCaseConfigurator implementation.
	StateNodesRequestor struct {
		TestReport `json:"test_report" yaml:"test_report" mapstructure:"test_report"`

		// IgnoringRequestsBy contains nodes which must ignore Replica0.
		IgnoringRequestsBy Nodes `json:"ignoring_requests_by" yaml:"ignoring_requests_by" mapstructure:"ignoring_requests_by"`

		Prepared bool

		Configured bool

		Ignored int

		statsCollector *stats.NodesClientStats

		mu sync.Mutex
	}
)

const (
	StateNodesRequestorName = "attack StateNodesRequestor"
)

var (
	// Ensure StateNodesRequestor implements TestCaseConfigurator.
	_ TestCaseConfigurator = (*BlockStateChangeRequestor)(nil)
)

// NewStateNodesRequestor creates initialised StateNodesRequestor.
func NewStateNodesRequestor(statsCollector *stats.NodesClientStats) *StateNodesRequestor {
	return &StateNodesRequestor{
		statsCollector: statsCollector,
	}
}

// TestCase implements TestCaseConfigurator interface.
func (n *StateNodesRequestor) TestCase() cases.TestCase {
	return cases.NewStateNodesRequestor(n.statsCollector, n.getType())
}

// Name implements TestCaseConfigurator interface.
func (n *StateNodesRequestor) Name() string {
	postfix := ""
	switch n.getType() {
	case cases.SNRNoReplies:
		postfix = "neither node reply"

	default:
		postfix = "unknown"
	}
	return StateNodesRequestorName + ": " + postfix
}

// Decode implements MapDecoder interface.
func (n *StateNodesRequestor) Decode(val interface{}) error {
	return mapstructure.Decode(val, n)
}

func (n *StateNodesRequestor) getType() cases.StateNodesRequestorCaseType {
	switch {
	case n.IgnoringRequestsBy.Num() > 1:
		return cases.SNRNoReplies

	default:
		return -1
	}
}

func (n *StateNodesRequestor) Lock() {
	if n == nil {
		return
	}
	n.mu.Lock()
}

func (n *StateNodesRequestor) Unlock() {
	if n == nil {
		return
	}
	n.mu.Unlock()
}
