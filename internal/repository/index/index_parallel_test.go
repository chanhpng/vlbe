package index_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chanhpng/vlbe/internal/errors"
	"github.com/chanhpng/vlbe/internal/repository"
	"github.com/chanhpng/vlbe/internal/repository/index"
	"github.com/chanhpng/vlbe/internal/restic"
	rtest "github.com/chanhpng/vlbe/internal/test"
)

var repoFixture = filepath.Join("..", "testdata", "test-repo.tar.gz")

func TestRepositoryForAllIndexes(t *testing.T) {
	repo, _, cleanup := repository.TestFromFixture(t, repoFixture)
	defer cleanup()

	expectedIndexIDs := restic.NewIDSet()
	rtest.OK(t, repo.List(context.TODO(), restic.IndexFile, func(id restic.ID, size int64) error {
		expectedIndexIDs.Insert(id)
		return nil
	}))

	// check that all expected indexes are loaded without errors
	indexIDs := restic.NewIDSet()
	var indexErr error
	rtest.OK(t, index.ForAllIndexes(context.TODO(), repo, repo, func(id restic.ID, index *index.Index, oldFormat bool, err error) error {
		if err != nil {
			indexErr = err
		}
		indexIDs.Insert(id)
		return nil
	}))
	rtest.OK(t, indexErr)
	rtest.Equals(t, expectedIndexIDs, indexIDs)

	// must failed with the returned error
	iterErr := errors.New("error to pass upwards")

	err := index.ForAllIndexes(context.TODO(), repo, repo, func(id restic.ID, index *index.Index, oldFormat bool, err error) error {
		return iterErr
	})

	rtest.Equals(t, iterErr, err)
}
