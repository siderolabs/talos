// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"fmt"
	"os"

	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/filetree"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// RestoreSnapshotOptions describes the snapshot restore.
type RestoreSnapshotOptions struct {
	// SnapshotPath is the path to the snapshot, it is removed once the snapshot is restored.
	SnapshotPath string
	// DataDir is the etcd data directory to restore the snapshot to.
	DataDir string
	// Name is the name of the etcd member.
	Name string
	// PeerURLs is the list of the peer URLs of the etcd member.
	PeerURLs []string
	// SkipHashCheck skips the integrity check of the snapshot.
	SkipHashCheck bool
}

// RestoreSnapshot recovers the etcd data directory from the snapshot as a new single-member cluster.
func RestoreSnapshot(logger *zap.Logger, opts RestoreSnapshotOptions) error {
	manager := snapshot.NewV3(logger)

	status, err := manager.Status(opts.SnapshotPath)
	if err != nil {
		return fmt.Errorf("error verifying snapshot: %w", err)
	}

	logger.Info("recovering etcd from snapshot",
		zap.String("hash", fmt.Sprintf("%08x", status.Hash)),
		zap.Int64("revision", status.Revision),
		zap.Int("total_keys", status.TotalKey),
		zap.Int64("total_size", status.TotalSize),
	)

	if err = manager.Restore(snapshot.RestoreConfig{
		SnapshotPath: opts.SnapshotPath,

		Name:          opts.Name,
		OutputDataDir: opts.DataDir,

		PeerURLs: opts.PeerURLs,

		InitialCluster: FormatInitialCluster(opts.Name, opts.PeerURLs),

		SkipHashCheck: opts.SkipHashCheck,
	}); err != nil {
		return fmt.Errorf("error recovering from the snapshot: %w", err)
	}

	if err = filetree.ChownRecursive(opts.DataDir, constants.EtcdUserID, constants.EtcdUserID); err != nil {
		return fmt.Errorf("error changing ownership of the recovered data: %w", err)
	}

	// the snapshot is removed last, so that it is still there if the recovery fails;
	// the etcd data is recovered at this point, so the leftover snapshot is not a reason to fail
	if err = os.Remove(opts.SnapshotPath); err != nil {
		logger.Warn("failed to delete the snapshot", zap.Error(err))
	}

	return nil
}
