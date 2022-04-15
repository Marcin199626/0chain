package partitions

type (
	info struct {
		partitionIndex int
		itemIndex      int
	}
)

var (
	// Ensure info implements ItemInformer interface.
	_ ItemInformer = (*info)(nil)
)

func newInfo(partitionIndex, itemIndex int) *info {
	return &info{
		partitionIndex: partitionIndex,
		itemIndex:      itemIndex,
	}
}

// PartitionIndex implements ItemInformer interface.
func (i *info) PartitionIndex() int {
	return i.partitionIndex
}

// ItemIndex implements ItemInformer interface.
func (i *info) ItemIndex() int {
	return i.itemIndex
}
