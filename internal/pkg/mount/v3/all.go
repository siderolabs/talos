// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"bufio"
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

// UnmountAll attempts to unmount all the mounted filesystems via "self" mountinfo.
//
//nolint:gocyclo
func UnmountAll() error {
	// timeout in seconds
	const timeout = 10

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for iteration := range timeout {
		mounts, err := readMountInfo()
		if err != nil {
			return err
		}

		failedUnmounts := 0

		var failedMountPoints []string

		for _, mountInfo := range mounts {
			if mountInfo.MountPoint == "" {
				continue
			}

			if strings.HasPrefix(mountInfo.MountSource, "/dev/") {
				err = SafeUnmount(context.Background(), log.Printf, mountInfo.MountPoint, true, false)
				if err == nil {
					log.Printf("unmounted %s (%s)", mountInfo.MountPoint, mountInfo.MountSource)
				} else {
					log.Printf("failed unmounting %s: %s", mountInfo.MountPoint, err)

					failedUnmounts++

					failedMountPoints = append(failedMountPoints, mountInfo.MountPoint)
				}
			}
		}

		if failedUnmounts == 0 {
			break
		}

		// Log mount users on first and last failure to help diagnose busy mounts.
		if iteration == 0 || iteration == timeout-1 {
			for _, mp := range failedMountPoints {
				logMountUsers(log.Printf, mp)
			}
		}

		log.Printf("retrying %d unmount operations...", failedUnmounts)

		<-ticker.C
	}

	return nil
}

type mountInfo struct {
	MountPoint   string
	MountSource  string
	MountType    string
	MountOptions map[string]string
}

func readMountInfo() ([]mountInfo, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	var mounts []mountInfo

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.SplitN(line, " - ", 2)

		if len(parts) < 2 {
			continue
		}

		var mntInfo mountInfo

		pre := strings.Fields(parts[0])
		post := strings.Fields(parts[1])

		if len(pre) >= 5 {
			mntInfo.MountPoint = pre[4]
		}

		if len(post) >= 1 {
			mntInfo.MountType = post[0]
		}

		if len(post) >= 2 {
			mntInfo.MountSource = post[1]
		}

		if len(post) >= 3 {
			mntInfo.MountOptions = make(map[string]string)

			for option := range strings.SplitSeq(post[2], ",") {
				k, v, ok := strings.Cut(option, "=")
				if ok {
					mntInfo.MountOptions[k] = v
				} else {
					mntInfo.MountOptions[option] = ""
				}
			}
		}

		mounts = append(mounts, mntInfo)
	}

	return mounts, scanner.Err()
}

func getSubmounts(target string) ([]string, error) {
	mounts, err := readMountInfo()
	if err != nil {
		return nil, err
	}

	var submounts []string

	seen := make(map[string]struct{})

	add := func(mountPoint string) {
		if _, ok := seen[mountPoint]; ok {
			return
		}

		seen[mountPoint] = struct{}{}
		submounts = append(submounts, mountPoint)
	}

	for _, mnt := range mounts {
		if mnt.MountPoint == target {
			continue
		}

		// mounts nested under the target keep it busy.
		if strings.HasPrefix(mnt.MountPoint, target+"/") {
			add(mnt.MountPoint)

			continue
		}

		// overlays mounted elsewhere still pin the target if any of their
		// backing directories (upper/work/lower) live under it.
		if mnt.MountType == "overlay" && overlayReferences(mnt, target) {
			add(mnt.MountPoint)
		}
	}

	sort.Slice(submounts, func(i, j int) bool {
		return len(submounts[i]) > len(submounts[j])
	})

	return submounts, nil
}

// overlayReferences reports whether an overlay mount has any of its backing
// directories (upperdir, workdir or one of the lowerdirs) located under target.
func overlayReferences(mnt mountInfo, target string) bool {
	prefix := target + "/"

	if strings.HasPrefix(mnt.MountOptions["upperdir"], prefix) {
		return true
	}

	if strings.HasPrefix(mnt.MountOptions["workdir"], prefix) {
		return true
	}

	for lowerdir := range strings.SplitSeq(mnt.MountOptions["lowerdir"], ":") {
		if strings.HasPrefix(lowerdir, prefix) {
			return true
		}
	}

	return false
}
