package main

import (
	"context"
	"os"

	"github.com/chanhpng/vlbe/internal/restic"
	"github.com/spf13/pflag"
)

// initMultiSnapshotFilter is used for commands that work on multiple snapshots
// MUST be combined with restic.FindFilteredSnapshots or FindFilteredSnapshots
func initMultiSnapshotFilter(flags *pflag.FlagSet, filt *restic.SnapshotFilter, addHostShorthand bool) {
	hostShorthand := "H"
	if !addHostShorthand {
		hostShorthand = ""
	}
	flags.StringArrayVarP(&filt.Hosts, "host", hostShorthand, nil, "only consider snapshots for this `host` (can be specified multiple times) (default: $RESTIC_HOST)")
	flags.Var(&filt.Tags, "tag", "only consider snapshots including `tag[,tag,...]` (can be specified multiple times)")
	flags.StringArrayVar(&filt.Paths, "path", nil, "only consider snapshots including this (absolute) `path` (can be specified multiple times, snapshots must include all specified paths)")

	// set default based on env if set
	if host := os.Getenv("RESTIC_HOST"); host != "" {
		filt.Hosts = []string{host}
	}
}

// initSingleSnapshotFilter is used for commands that work on a single snapshot
// MUST be combined with restic.FindFilteredSnapshot
func initSingleSnapshotFilter(flags *pflag.FlagSet, filt *restic.SnapshotFilter) {
	flags.StringArrayVarP(&filt.Hosts, "host", "H", nil, "only consider snapshots for this `host`, when snapshot ID \"latest\" is given (can be specified multiple times) (default: $RESTIC_HOST)")
	flags.Var(&filt.Tags, "tag", "only consider snapshots including `tag[,tag,...]`, when snapshot ID \"latest\" is given (can be specified multiple times)")
	flags.StringArrayVar(&filt.Paths, "path", nil, "only consider snapshots including this (absolute) `path`, when snapshot ID \"latest\" is given (can be specified multiple times, snapshots must include all specified paths)")

	// set default based on env if set
	if host := os.Getenv("RESTIC_HOST"); host != "" {
		filt.Hosts = []string{host}
	}
}

// FindFilteredSnapshots yields Snapshots, either given explicitly by `snapshotIDs` or filtered from the list of all snapshots.
func FindFilteredSnapshots(ctx context.Context, be restic.Lister, loader restic.LoaderUnpacked, f *restic.SnapshotFilter, snapshotIDs []string) <-chan *restic.Snapshot {
	out := make(chan *restic.Snapshot)
	go func() {
		defer close(out)
		be, err := restic.MemorizeList(ctx, be, restic.SnapshotFile)
		if err != nil {
			Warnf("could not load snapshots: %v\n", err)
			return
		}

		err = f.FindAll(ctx, be, loader, snapshotIDs, func(id string, sn *restic.Snapshot, err error) error {
			if err != nil {
				Warnf("Ignoring %q: %v\n", id, err)
			} else {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- sn:
				}
			}
			return nil
		})
		if err != nil {
			Warnf("could not load snapshots: %v\n", err)
		}
	}()
	return out
}
