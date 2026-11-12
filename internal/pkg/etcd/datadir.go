// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// LearnerMarkerFilename is the name of the file in the etcd data directory which marks the etcd data as belonging to a learner.
//
// The marker holds the member ID assigned to the learner. It is written before etcd is started
// as a learner, and it is removed once the learner is promoted to a voting member.
//
// The data of a learner is disposable (a learner is not part of the quorum), while the data of a voting member is not,
// so the marker is the way to tell them apart after a restart.
const LearnerMarkerFilename = ".talos-learner"

// memberDirectory is the directory etcd keeps its data in under the data directory.
const memberDirectory = "member"

// DataDir provides the operations on the etcd data directory.
type DataDir struct {
	path string
}

// NewDataDir creates the DataDir for the given path.
func NewDataDir(path string) *DataDir {
	return &DataDir{path: path}
}

// HasMemberData checks if etcd has ever been started with this data directory.
func (d *DataDir) HasMemberData() (bool, error) {
	_, err := os.Stat(filepath.Join(d.path, memberDirectory))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// LearnerMarker returns the member ID stored in the learner marker.
//
// If the marker doesn't exist, the second return value is false.
func (d *DataDir) LearnerMarker() (uint64, bool, error) {
	contents, err := os.ReadFile(filepath.Join(d.path, LearnerMarkerFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, false, nil
		}

		return 0, false, err
	}

	// a marker which can't be parsed still marks the data as disposable, the member ID is just not known
	memberID, _ := etcdresource.ParseMemberID(strings.TrimSpace(string(contents))) //nolint:errcheck

	return memberID, true, nil
}

// WriteLearnerMarker marks the etcd data as belonging to a learner with the given member ID.
func (d *DataDir) WriteLearnerMarker(memberID uint64) error {
	path := filepath.Join(d.path, LearnerMarkerFilename)

	f, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	if _, err = f.WriteString(etcdresource.FormatMemberID(memberID) + "\n"); err != nil {
		f.Close() //nolint:errcheck

		return err
	}

	if err = f.Sync(); err != nil {
		f.Close() //nolint:errcheck

		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	if err = os.Rename(path+".tmp", path); err != nil {
		return err
	}

	return d.sync()
}

// RemoveLearnerMarker removes the learner marker.
func (d *DataDir) RemoveLearnerMarker() error {
	if err := os.Remove(filepath.Join(d.path, LearnerMarkerFilename)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return d.sync()
}

// Wipe removes the contents of the etcd data directory.
//
// The directory itself is kept: it may be a dedicated volume mount point, and unlinking a mount point fails with EBUSY.
//
// The learner marker (if any) is removed last, so that an interrupted wipe is picked up after a restart.
func (d *DataDir) Wipe() error {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("failed to read %s: %w", d.path, err)
	}

	for _, entry := range entries {
		if entry.Name() == LearnerMarkerFilename {
			continue
		}

		p := filepath.Join(d.path, entry.Name())

		if err = os.RemoveAll(p); err != nil {
			return fmt.Errorf("failed to remove %s: %w", p, err)
		}
	}

	return d.RemoveLearnerMarker()
}

func (d *DataDir) sync() error {
	dir, err := os.Open(d.path)
	if err != nil {
		return err
	}

	defer dir.Close() //nolint:errcheck

	return dir.Sync()
}

// Path returns the path to the etcd data directory.
func (d *DataDir) Path() string {
	return d.path
}
