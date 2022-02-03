//go:build !integration_tests
// +build !integration_tests

package chain

import (
	"context"

	"0chain.net/core/util"
)

func (c *Chain) syncRoundStateToStateDB(ctx context.Context, round int64, rootStateHash util.Key) {
	syncRoundStateToStateDB(ctx, round, rootStateHash)
}
