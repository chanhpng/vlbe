package index

import (
	"testing"

	"github.com/chanhpng/vlbe/internal/restic"
	"github.com/chanhpng/vlbe/internal/test"
)

func TestMergeIndex(t testing.TB, mi *MasterIndex) ([]*Index, int, restic.IDSet) {
	finalIndexes := mi.finalizeNotFinalIndexes()
	ids := restic.NewIDSet()
	for _, idx := range finalIndexes {
		id := restic.NewRandomID()
		ids.Insert(id)
		test.OK(t, idx.SetID(id))
	}

	test.OK(t, mi.MergeFinalIndexes())
	return finalIndexes, len(mi.idx), ids
}
