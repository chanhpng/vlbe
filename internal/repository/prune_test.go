package repository_test

import (
	"context"
	"math"
	"testing"

	"github.com/chanhpng/vlbe/internal/checker"
	"github.com/chanhpng/vlbe/internal/repository"
	"github.com/chanhpng/vlbe/internal/restic"
	rtest "github.com/chanhpng/vlbe/internal/test"
	"github.com/chanhpng/vlbe/internal/ui/progress"
	"golang.org/x/sync/errgroup"
)

func testPrune(t *testing.T, opts repository.PruneOptions, errOnUnused bool) {
	repo, be := repository.TestRepositoryWithVersion(t, 0)
	createRandomBlobs(t, repo, 4, 0.5, true)
	createRandomBlobs(t, repo, 5, 0.5, true)
	keep, _ := selectBlobs(t, repo, 0.5)

	var wg errgroup.Group
	repo.StartPackUploader(context.TODO(), &wg)
	// duplicate a few blobs to exercise those code paths
	for blob := range keep {
		buf, err := repo.LoadBlob(context.TODO(), blob.Type, blob.ID, nil)
		rtest.OK(t, err)
		_, _, _, err = repo.SaveBlob(context.TODO(), blob.Type, buf, blob.ID, true)
		rtest.OK(t, err)
	}
	rtest.OK(t, repo.Flush(context.TODO()))

	plan, err := repository.PlanPrune(context.TODO(), opts, repo, func(ctx context.Context, repo restic.Repository, usedBlobs restic.FindBlobSet) error {
		for blob := range keep {
			usedBlobs.Insert(blob)
		}
		return nil
	}, &progress.NoopPrinter{})
	rtest.OK(t, err)

	rtest.OK(t, plan.Execute(context.TODO(), &progress.NoopPrinter{}))

	repo = repository.TestOpenBackend(t, be)
	checker.TestCheckRepo(t, repo, true)

	if errOnUnused {
		existing := listBlobs(repo)
		rtest.Assert(t, existing.Equals(keep), "unexpected blobs, wanted %v got %v", keep, existing)
	}
}

func TestPrune(t *testing.T) {
	for _, test := range []struct {
		name        string
		opts        repository.PruneOptions
		errOnUnused bool
	}{
		{
			name: "0",
			opts: repository.PruneOptions{
				MaxRepackBytes: math.MaxUint64,
				MaxUnusedBytes: func(used uint64) (unused uint64) { return 0 },
			},
			errOnUnused: true,
		},
		{
			name: "50",
			opts: repository.PruneOptions{
				MaxRepackBytes: math.MaxUint64,
				MaxUnusedBytes: func(used uint64) (unused uint64) { return used / 2 },
			},
		},
		{
			name: "unlimited",
			opts: repository.PruneOptions{
				MaxRepackBytes: math.MaxUint64,
				MaxUnusedBytes: func(used uint64) (unused uint64) { return math.MaxUint64 },
			},
		},
		{
			name: "cachableonly",
			opts: repository.PruneOptions{
				MaxRepackBytes:      math.MaxUint64,
				MaxUnusedBytes:      func(used uint64) (unused uint64) { return used / 20 },
				RepackCacheableOnly: true,
			},
		},
		{
			name: "small",
			opts: repository.PruneOptions{
				MaxRepackBytes: math.MaxUint64,
				MaxUnusedBytes: func(used uint64) (unused uint64) { return math.MaxUint64 },
				RepackSmall:    true,
			},
			errOnUnused: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			testPrune(t, test.opts, test.errOnUnused)
		})
		t.Run(test.name+"-recovery", func(t *testing.T) {
			opts := test.opts
			opts.UnsafeRecovery = true
			// unsafeNoSpaceRecovery does not repack partially used pack files
			testPrune(t, opts, false)
		})
	}
}
