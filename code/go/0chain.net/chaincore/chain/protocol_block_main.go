//go:build !integration_tests
// +build !integration_tests

package chain

import (
	"0chain.net/chaincore/block"
)

func (c *Chain) UpdateBlockNotarization(b *block.Block) bool {
	return c.updateBlockNotarization(b)
}
