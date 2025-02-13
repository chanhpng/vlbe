package index

import (
	"math/rand"
	"testing"
	"time"

	"github.com/chanhpng/vlbe/internal/restic"
	rtest "github.com/chanhpng/vlbe/internal/test"
)

func TestIndexMapBasic(t *testing.T) {
	t.Parallel()

	var (
		id restic.ID
		m  indexMap
		r  = rand.New(rand.NewSource(98765))
	)

	for i := 1; i <= 400; i++ {
		r.Read(id[:])
		rtest.Assert(t, m.get(id) == nil, "%v retrieved but not added", id)

		m.add(id, 0, 0, 0, 0)
		rtest.Assert(t, m.get(id) != nil, "%v added but not retrieved", id)
		rtest.Equals(t, uint(i), m.len())
	}
}

func TestIndexMapForeach(t *testing.T) {
	t.Parallel()

	const N = 10

	var m indexMap

	// Don't crash on empty map.
	m.foreach(func(*indexEntry) bool { return true })

	for i := 0; i < N; i++ {
		var id restic.ID
		id[0] = byte(i)
		m.add(id, i, uint32(i), uint32(i), uint32(i/2))
	}

	seen := make(map[int]struct{})
	m.foreach(func(e *indexEntry) bool {
		i := int(e.id[0])
		rtest.Assert(t, i < N, "unknown id %v in indexMap", e.id)
		rtest.Equals(t, i, e.packIndex)
		rtest.Equals(t, i, int(e.length))
		rtest.Equals(t, i, int(e.offset))
		rtest.Equals(t, i/2, int(e.uncompressedLength))

		seen[i] = struct{}{}
		return true
	})

	rtest.Equals(t, N, len(seen))

	ncalls := 0
	m.foreach(func(*indexEntry) bool {
		ncalls++
		return false
	})
	rtest.Equals(t, 1, ncalls)
}

func TestIndexMapForeachWithID(t *testing.T) {
	t.Parallel()

	const ndups = 3

	var (
		id restic.ID
		m  indexMap
		r  = rand.New(rand.NewSource(1234321))
	)
	r.Read(id[:])

	// No result (and no crash) for empty map.
	n := 0
	m.foreachWithID(id, func(*indexEntry) { n++ })
	rtest.Equals(t, 0, n)

	// Test insertion and retrieval of duplicates.
	for i := 0; i < ndups; i++ {
		m.add(id, i, 0, 0, 0)
	}

	for i := 0; i < 100; i++ {
		var otherid restic.ID
		r.Read(otherid[:])
		m.add(otherid, -1, 0, 0, 0)
	}

	n = 0
	var packs [ndups]bool
	m.foreachWithID(id, func(e *indexEntry) {
		packs[e.packIndex] = true
		n++
	})
	rtest.Equals(t, ndups, n)

	for i := range packs {
		rtest.Assert(t, packs[i], "duplicate from pack %d not retrieved", i)
	}
}

func TestHashedArrayTree(t *testing.T) {
	hat := newHAT()
	const testSize = 1024
	for i := uint(0); i < testSize; i++ {
		rtest.Assert(t, hat.Size() == i, "expected hat size %v got %v", i, hat.Size())
		e, idx := hat.Alloc()
		rtest.Assert(t, idx == i, "expected entry at idx %v got %v", i, idx)
		e.length = uint32(i)
	}
	for i := uint(0); i < testSize; i++ {
		e := hat.Ref(i)
		rtest.Assert(t, e.length == uint32(i), "expected entry to contain %v got %v", uint32(i), e.length)
	}
}

func BenchmarkIndexMapHash(b *testing.B) {
	var m indexMap
	m.add(restic.ID{}, 0, 0, 0, 0) // Trigger lazy initialization.

	ids := make([]restic.ID, 128) // 4 KiB.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range ids {
		r.Read(ids[i][:])
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(restic.ID{}) * len(ids)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, id := range ids {
			m.hash(id)
		}
	}
}

func TestIndexMapFirstIndex(t *testing.T) {
	t.Parallel()

	var (
		id restic.ID
		m  indexMap
		r  = rand.New(rand.NewSource(98765))
		fi = make(map[restic.ID]int)
	)

	for i := 1; i <= 400; i++ {
		r.Read(id[:])
		rtest.Equals(t, -1, m.firstIndex(id), "wrong firstIndex for nonexistent id")

		m.add(id, 0, 0, 0, 0)
		idx := m.firstIndex(id)
		rtest.Equals(t, i, idx, "unexpected index for id")
		fi[id] = idx
	}
	// iterate over blobs, as this is a hashmap the order is effectively random
	for id, idx := range fi {
		rtest.Equals(t, idx, m.firstIndex(id), "wrong index returned")
	}
}

func TestIndexMapFirstIndexDuplicates(t *testing.T) {
	t.Parallel()

	var (
		id restic.ID
		m  indexMap
		r  = rand.New(rand.NewSource(98765))
	)

	r.Read(id[:])
	for i := 1; i <= 10; i++ {
		m.add(id, 0, 0, 0, 0)
	}
	idx := m.firstIndex(id)
	rtest.Equals(t, 1, idx, "unexpected index for id")
}
