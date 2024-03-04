// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sysblock implements gathering block device information from /sys/block filesystem.
package sysblock

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mdlayher/kobject"
)

// Walk the /sys/block filesystem and gather block device information.
//
//nolint:gocyclo
func Walk(root string) ([]*kobject.Event, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", root, err)
	}

	result := make([]*kobject.Event, 0, len(entries))

	for _, entry := range entries {
		fi, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("failed to stat %s: %w", entry.Name(), err)
		}

		if fi.Mode()&os.ModeSymlink == 0 {
			continue
		}

		path, err := filepath.EvalSymlinks(filepath.Join(root, entry.Name()))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, fmt.Errorf("failed to resolve symlink %s: %w", entry.Name(), err)
		}

		uevent, err := readUevent(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, err
		}

		result = append(result, &kobject.Event{
			Action:     kobject.Add,
			DevicePath: path,
			Subsystem:  "block",
			Values:     uevent,
		})

		partitions, err := readPartitions(path)
		if err != nil {
			return nil, err
		}

		result = append(result, partitions...)
	}

	return result, nil
}

// readUevent reads the /sys/block/<device>/uevent file and returns the content.
func readUevent(path string) (map[string]string, error) {
	path = filepath.Join(path, "uevent")

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", path, err)
	}

	result := map[string]string{}

	for _, kv := range bytes.Split(content, []byte("\n")) {
		key, value, ok := bytes.Cut(kv, []byte("="))
		if !ok {
			continue
		}

		result[string(key)] = string(value)
	}

	return result, nil
}

// readPartitions reads partitions for a given device and returns the list of events.
func readPartitions(path string) ([]*kobject.Event, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var result []*kobject.Event //nolint:prealloc

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		partitionPath := filepath.Join(path, entry.Name())

		_, err = os.Stat(filepath.Join(partitionPath, "partition"))
		if err != nil {
			continue
		}

		uevent, err := readUevent(partitionPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, err
		}

		result = append(result, &kobject.Event{
			Action:     kobject.Add,
			DevicePath: partitionPath,
			Subsystem:  "block",
			Values:     uevent,
		})
	}

	return result, nil
}
