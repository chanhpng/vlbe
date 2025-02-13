package checker_test

import (
	"context"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chanhpng/vlbe/internal/archiver"
	"github.com/chanhpng/vlbe/internal/backend"
	"github.com/chanhpng/vlbe/internal/checker"
	"github.com/chanhpng/vlbe/internal/errors"
	"github.com/chanhpng/vlbe/internal/repository"
	"github.com/chanhpng/vlbe/internal/repository/hashing"
	"github.com/chanhpng/vlbe/internal/restic"
	"github.com/chanhpng/vlbe/internal/test"
	"golang.org/x/sync/errgroup"
)

var checkerTestData = filepath.Join("testdata", "checker-test-repo.tar.gz")

func collectErrors(ctx context.Context, f func(context.Context, chan<- error)) (errs []error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errChan := make(chan error)

	go f(ctx, errChan)

	for err := range errChan {
		errs = append(errs, err)
	}

	return errs
}

func checkPacks(chkr *checker.Checker) []error {
	return collectErrors(context.TODO(), chkr.Packs)
}

func checkStruct(chkr *checker.Checker) []error {
	err := chkr.LoadSnapshots(context.TODO())
	if err != nil {
		return []error{err}
	}
	return collectErrors(context.TODO(), func(ctx context.Context, errChan chan<- error) {
		chkr.Structure(ctx, nil, errChan)
	})
}

func checkData(chkr *checker.Checker) []error {
	return collectErrors(
		context.TODO(),
		func(ctx context.Context, errCh chan<- error) {
			chkr.ReadData(ctx, errCh)
		},
	)
}

func assertOnlyMixedPackHints(t *testing.T, hints []error) {
	for _, err := range hints {
		if _, ok := err.(*checker.ErrMixedPack); !ok {
			t.Fatalf("expected mixed pack hint, got %v", err)
		}
	}
}

func TestCheckRepo(t *testing.T) {
	repo, _, cleanup := repository.TestFromFixture(t, checkerTestData)
	defer cleanup()

	chkr := checker.New(repo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}
	assertOnlyMixedPackHints(t, hints)
	if len(hints) == 0 {
		t.Fatal("expected mixed pack warnings, got none")
	}

	test.OKs(t, checkPacks(chkr))
	test.OKs(t, checkStruct(chkr))
}

func TestMissingPack(t *testing.T) {
	repo, be, cleanup := repository.TestFromFixture(t, checkerTestData)
	defer cleanup()

	packID := restic.TestParseID("657f7fb64f6a854fff6fe9279998ee09034901eded4e6db9bcee0e59745bbce6")
	test.OK(t, be.Remove(context.TODO(), backend.Handle{Type: restic.PackFile, Name: packID.String()}))

	chkr := checker.New(repo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}
	assertOnlyMixedPackHints(t, hints)

	errs = checkPacks(chkr)

	test.Assert(t, len(errs) == 1,
		"expected exactly one error, got %v", len(errs))

	if err, ok := errs[0].(*checker.PackError); ok {
		test.Equals(t, packID, err.ID)
	} else {
		t.Errorf("expected error returned by checker.Packs() to be PackError, got %v", err)
	}
}

func TestUnreferencedPack(t *testing.T) {
	repo, be, cleanup := repository.TestFromFixture(t, checkerTestData)
	defer cleanup()

	// index 3f1a only references pack 60e0
	packID := "60e0438dcb978ec6860cc1f8c43da648170ee9129af8f650f876bad19f8f788e"
	indexID := restic.TestParseID("3f1abfcb79c6f7d0a3be517d2c83c8562fba64ef2c8e9a3544b4edaf8b5e3b44")
	test.OK(t, be.Remove(context.TODO(), backend.Handle{Type: restic.IndexFile, Name: indexID.String()}))

	chkr := checker.New(repo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}
	assertOnlyMixedPackHints(t, hints)

	errs = checkPacks(chkr)

	test.Assert(t, len(errs) == 1,
		"expected exactly one error, got %v", len(errs))

	if err, ok := errs[0].(*checker.PackError); ok {
		test.Equals(t, packID, err.ID.String())
	} else {
		t.Errorf("expected error returned by checker.Packs() to be PackError, got %v", err)
	}
}

func TestUnreferencedBlobs(t *testing.T) {
	repo, _, cleanup := repository.TestFromFixture(t, checkerTestData)
	defer cleanup()

	snapshotID := restic.TestParseID("51d249d28815200d59e4be7b3f21a157b864dc343353df9d8e498220c2499b02")
	test.OK(t, repo.RemoveUnpacked(context.TODO(), restic.SnapshotFile, snapshotID))

	unusedBlobsBySnapshot := restic.BlobHandles{
		restic.TestParseHandle("58c748bbe2929fdf30c73262bd8313fe828f8925b05d1d4a87fe109082acb849", restic.DataBlob),
		restic.TestParseHandle("988a272ab9768182abfd1fe7d7a7b68967825f0b861d3b36156795832c772235", restic.DataBlob),
		restic.TestParseHandle("c01952de4d91da1b1b80bc6e06eaa4ec21523f4853b69dc8231708b9b7ec62d8", restic.TreeBlob),
		restic.TestParseHandle("bec3a53d7dc737f9a9bee68b107ec9e8ad722019f649b34d474b9982c3a3fec7", restic.TreeBlob),
		restic.TestParseHandle("2a6f01e5e92d8343c4c6b78b51c5a4dc9c39d42c04e26088c7614b13d8d0559d", restic.TreeBlob),
		restic.TestParseHandle("18b51b327df9391732ba7aaf841a4885f350d8a557b2da8352c9acf8898e3f10", restic.DataBlob),
	}

	sort.Sort(unusedBlobsBySnapshot)

	chkr := checker.New(repo, true)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}
	assertOnlyMixedPackHints(t, hints)

	test.OKs(t, checkPacks(chkr))
	test.OKs(t, checkStruct(chkr))

	blobs, err := chkr.UnusedBlobs(context.TODO())
	test.OK(t, err)
	sort.Sort(blobs)

	test.Equals(t, unusedBlobsBySnapshot, blobs)
}

func TestModifiedIndex(t *testing.T) {
	repo, be, cleanup := repository.TestFromFixture(t, checkerTestData)
	defer cleanup()

	done := make(chan struct{})
	defer close(done)

	h := backend.Handle{
		Type: restic.IndexFile,
		Name: "90f838b4ac28735fda8644fe6a08dbc742e57aaf81b30977b4fefa357010eafd",
	}

	tmpfile, err := os.CreateTemp("", "restic-test-mod-index-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := tmpfile.Close()
		if err != nil {
			t.Fatal(err)
		}

		err = os.Remove(tmpfile.Name())
		if err != nil {
			t.Fatal(err)
		}
	}()
	wr := io.Writer(tmpfile)
	var hw *hashing.Writer
	if be.Hasher() != nil {
		hw = hashing.NewWriter(wr, be.Hasher())
		wr = hw
	}

	// read the file from the backend
	err = be.Load(context.TODO(), h, 0, 0, func(rd io.Reader) error {
		_, err := io.Copy(wr, rd)
		return err
	})
	test.OK(t, err)

	// save the index again with a modified name so that the hash doesn't match
	// the content any more
	h2 := backend.Handle{
		Type: restic.IndexFile,
		Name: "80f838b4ac28735fda8644fe6a08dbc742e57aaf81b30977b4fefa357010eafd",
	}

	var hash []byte
	if hw != nil {
		hash = hw.Sum(nil)
	}
	rd, err := backend.NewFileReader(tmpfile, hash)
	if err != nil {
		t.Fatal(err)
	}

	err = be.Save(context.TODO(), h2, rd)
	if err != nil {
		t.Fatal(err)
	}

	chkr := checker.New(repo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) == 0 {
		t.Fatalf("expected errors not found")
	}

	for _, err := range errs {
		t.Logf("found expected error %v", err)
	}

	assertOnlyMixedPackHints(t, hints)
}

var checkerDuplicateIndexTestData = filepath.Join("testdata", "duplicate-packs-in-index-test-repo.tar.gz")

func TestDuplicatePacksInIndex(t *testing.T) {
	repo, _, cleanup := repository.TestFromFixture(t, checkerDuplicateIndexTestData)
	defer cleanup()

	chkr := checker.New(repo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(hints) == 0 {
		t.Fatalf("did not get expected checker hints for duplicate packs in indexes")
	}

	found := false
	for _, hint := range hints {
		if _, ok := hint.(*checker.ErrDuplicatePacks); ok {
			found = true
		} else {
			t.Errorf("got unexpected hint: %v", hint)
		}
	}

	if !found {
		t.Fatalf("did not find hint ErrDuplicatePacks")
	}

	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v: %v", len(errs), errs)
	}
}

// errorBackend randomly modifies data after reading.
type errorBackend struct {
	backend.Backend
	ProduceErrors bool
}

func (b errorBackend) Load(ctx context.Context, h backend.Handle, length int, offset int64, consumer func(rd io.Reader) error) error {
	return b.Backend.Load(ctx, h, length, offset, func(rd io.Reader) error {
		if b.ProduceErrors {
			return consumer(errorReadCloser{rd})
		}
		return consumer(rd)
	})
}

type errorReadCloser struct {
	io.Reader
}

func (erd errorReadCloser) Read(p []byte) (int, error) {
	n, err := erd.Reader.Read(p)
	if n > 0 {
		induceError(p[:n])
	}
	return n, err
}

// induceError flips a bit in the slice.
func induceError(data []byte) {
	pos := rand.Intn(len(data))
	data[pos] ^= 1
}

// errorOnceBackend randomly modifies data when reading a file for the first time.
type errorOnceBackend struct {
	backend.Backend
	m sync.Map
}

func (b *errorOnceBackend) Load(ctx context.Context, h backend.Handle, length int, offset int64, consumer func(rd io.Reader) error) error {
	_, isRetry := b.m.LoadOrStore(h, struct{}{})
	return b.Backend.Load(ctx, h, length, offset, func(rd io.Reader) error {
		if !isRetry && h.Type != restic.ConfigFile {
			return consumer(errorReadCloser{rd})
		}
		return consumer(rd)
	})
}

func TestCheckerModifiedData(t *testing.T) {
	repo, be := repository.TestRepositoryWithVersion(t, 0)
	sn := archiver.TestSnapshot(t, repo, ".", nil)
	t.Logf("archived as %v", sn.ID().Str())

	errBe := &errorBackend{Backend: be}

	for _, test := range []struct {
		name   string
		be     backend.Backend
		damage func()
		check  func(t *testing.T, err error)
	}{
		{
			"errorBackend",
			errBe,
			func() {
				errBe.ProduceErrors = true
			},
			func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("no error found, checker is broken")
				}
			},
		},
		{
			"errorOnceBackend",
			&errorOnceBackend{Backend: be},
			func() {},
			func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "check successful on second attempt, original error pack") {
					t.Fatalf("wrong error found, got %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			checkRepo := repository.TestOpenBackend(t, test.be)

			chkr := checker.New(checkRepo, false)

			hints, errs := chkr.LoadIndex(context.TODO(), nil)
			if len(errs) > 0 {
				t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
			}

			if len(hints) > 0 {
				t.Errorf("expected no hints, got %v: %v", len(hints), hints)
			}

			test.damage()
			var err error
			for _, err := range checkPacks(chkr) {
				t.Logf("pack error: %v", err)
			}

			for _, err := range checkStruct(chkr) {
				t.Logf("struct error: %v", err)
			}

			for _, cerr := range checkData(chkr) {
				t.Logf("data error: %v", cerr)
				if err == nil {
					err = cerr
				}
			}

			test.check(t, err)
		})
	}
}

// loadTreesOnceRepository allows each tree to be loaded only once
type loadTreesOnceRepository struct {
	restic.Repository
	loadedTrees   restic.IDSet
	mutex         sync.Mutex
	DuplicateTree bool
}

func (r *loadTreesOnceRepository) LoadTree(ctx context.Context, id restic.ID) (*restic.Tree, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.loadedTrees.Has(id) {
		// additionally store error to ensure that it cannot be swallowed
		r.DuplicateTree = true
		return nil, errors.Errorf("trying to load tree with id %v twice", id)
	}
	r.loadedTrees.Insert(id)
	return restic.LoadTree(ctx, r.Repository, id)
}

func TestCheckerNoDuplicateTreeDecodes(t *testing.T) {
	repo, _, cleanup := repository.TestFromFixture(t, checkerTestData)
	defer cleanup()
	checkRepo := &loadTreesOnceRepository{
		Repository:  repo,
		loadedTrees: restic.NewIDSet(),
	}

	chkr := checker.New(checkRepo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}
	assertOnlyMixedPackHints(t, hints)

	test.OKs(t, checkPacks(chkr))
	test.OKs(t, checkStruct(chkr))
	test.Assert(t, !checkRepo.DuplicateTree, "detected duplicate tree loading")
}

// delayRepository delays read of a specific handle.
type delayRepository struct {
	restic.Repository
	DelayTree      restic.ID
	UnblockChannel chan struct{}
	Unblocker      sync.Once
}

func (r *delayRepository) LoadTree(ctx context.Context, id restic.ID) (*restic.Tree, error) {
	if id == r.DelayTree {
		<-r.UnblockChannel
	}
	return restic.LoadTree(ctx, r.Repository, id)
}

func (r *delayRepository) LookupBlobSize(t restic.BlobType, id restic.ID) (uint, bool) {
	if id == r.DelayTree && t == restic.DataBlob {
		r.Unblock()
	}
	return r.Repository.LookupBlobSize(t, id)
}

func (r *delayRepository) Unblock() {
	r.Unblocker.Do(func() {
		close(r.UnblockChannel)
	})
}

func TestCheckerBlobTypeConfusion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	repo := repository.TestRepository(t)

	damagedNode := &restic.Node{
		Name:    "damaged",
		Type:    "file",
		Mode:    0644,
		Size:    42,
		Content: restic.IDs{restic.TestParseID("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")},
	}
	damagedTree := &restic.Tree{
		Nodes: []*restic.Node{damagedNode},
	}

	wg, wgCtx := errgroup.WithContext(ctx)
	repo.StartPackUploader(wgCtx, wg)
	id, err := restic.SaveTree(ctx, repo, damagedTree)
	test.OK(t, repo.Flush(ctx))
	test.OK(t, err)

	buf, err := repo.LoadBlob(ctx, restic.TreeBlob, id, nil)
	test.OK(t, err)

	wg, wgCtx = errgroup.WithContext(ctx)
	repo.StartPackUploader(wgCtx, wg)
	_, _, _, err = repo.SaveBlob(ctx, restic.DataBlob, buf, id, false)
	test.OK(t, err)

	malNode := &restic.Node{
		Name:    "aaaaa",
		Type:    "file",
		Mode:    0644,
		Size:    uint64(len(buf)),
		Content: restic.IDs{id},
	}
	dirNode := &restic.Node{
		Name:    "bbbbb",
		Type:    "dir",
		Mode:    0755,
		Subtree: &id,
	}

	rootTree := &restic.Tree{
		Nodes: []*restic.Node{malNode, dirNode},
	}

	rootID, err := restic.SaveTree(ctx, repo, rootTree)
	test.OK(t, err)

	test.OK(t, repo.Flush(ctx))

	snapshot, err := restic.NewSnapshot([]string{"/damaged"}, []string{"test"}, "foo", time.Now())
	test.OK(t, err)

	snapshot.Tree = &rootID

	snapID, err := restic.SaveSnapshot(ctx, repo, snapshot)
	test.OK(t, err)

	t.Logf("saved snapshot %v", snapID.Str())

	delayRepo := &delayRepository{
		Repository:     repo,
		DelayTree:      id,
		UnblockChannel: make(chan struct{}),
	}

	chkr := checker.New(delayRepo, false)

	go func() {
		<-ctx.Done()
		delayRepo.Unblock()
	}()

	hints, errs := chkr.LoadIndex(ctx, nil)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}

	if len(hints) > 0 {
		t.Errorf("expected no hints, got %v: %v", len(hints), hints)
	}

	errFound := false

	for _, err := range checkStruct(chkr) {
		t.Logf("struct error: %v", err)
		errFound = true
	}

	test.OK(t, ctx.Err())

	if !errFound {
		t.Fatal("no error found, checker is broken")
	}
}

func loadBenchRepository(t *testing.B) (*checker.Checker, restic.Repository, func()) {
	repo, _, cleanup := repository.TestFromFixture(t, checkerTestData)

	chkr := checker.New(repo, false)
	hints, errs := chkr.LoadIndex(context.TODO(), nil)
	if len(errs) > 0 {
		defer cleanup()
		t.Fatalf("expected no errors, got %v: %v", len(errs), errs)
	}

	for _, err := range hints {
		if _, ok := err.(*checker.ErrMixedPack); !ok {
			t.Fatalf("expected mixed pack hint, got %v", err)
		}
	}
	return chkr, repo, cleanup
}

func BenchmarkChecker(t *testing.B) {
	chkr, _, cleanup := loadBenchRepository(t)
	defer cleanup()

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		test.OKs(t, checkPacks(chkr))
		test.OKs(t, checkStruct(chkr))
		test.OKs(t, checkData(chkr))
	}
}

func benchmarkSnapshotScaling(t *testing.B, newSnapshots int) {
	chkr, repo, cleanup := loadBenchRepository(t)
	defer cleanup()

	snID := restic.TestParseID("51d249d28815200d59e4be7b3f21a157b864dc343353df9d8e498220c2499b02")
	sn2, err := restic.LoadSnapshot(context.TODO(), repo, snID)
	if err != nil {
		t.Fatal(err)
	}

	treeID := sn2.Tree

	for i := 0; i < newSnapshots; i++ {
		sn, err := restic.NewSnapshot([]string{"test" + strconv.Itoa(i)}, nil, "", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		sn.Tree = treeID

		_, err = restic.SaveSnapshot(context.TODO(), repo, sn)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		test.OKs(t, checkPacks(chkr))
		test.OKs(t, checkStruct(chkr))
		test.OKs(t, checkData(chkr))
	}
}

func BenchmarkCheckerSnapshotScaling(b *testing.B) {
	counts := []int{50, 100, 200}
	for _, count := range counts {
		count := count
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			benchmarkSnapshotScaling(b, count)
		})
	}
}
