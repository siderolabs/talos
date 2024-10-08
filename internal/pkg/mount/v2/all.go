// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func unmountWithTimeout(target string, flags int, timeout time.Duration) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- unix.Unmount(target, flags)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		return fmt.Errorf("unmounting %s timed out after %s", target, timeout)
	case err := <-errCh:
		return err
	}
}

// UnmountAll attempts to unmount all the mounted filesystems via "self" mountinfo.
func UnmountAll() error {
	// timeout in seconds
	const timeout = 10

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range timeout {
		mounts, err := readMountInfo()
		if err != nil {
			return err
		}

		failedUnmounts := 0

		for _, mountInfo := range mounts {
			if mountInfo.MountPoint == "" {
				continue
			}

			if strings.HasPrefix(mountInfo.MountSource, "/dev/") {
				err = unmountWithTimeout(mountInfo.MountPoint, 0, time.Second)

				if err == nil {
					log.Printf("unmounted %s (%s)", mountInfo.MountPoint, mountInfo.MountSource)
				} else {
					log.Printf("failed unmounting %s: %s", mountInfo.MountPoint, err)

					failedUnmounts++
				}
			}
		}

		if failedUnmounts == 0 {
			break
		}

		log.Printf("retrying %d unmount operations...", failedUnmounts)

		<-ticker.C
	}

	return nil
}

type mountInfo struct {
	MountPoint  string
	MountSource string
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
			mntInfo.MountSource = post[1]
		}

		mounts = append(mounts, mntInfo)
	}

	return mounts, scanner.Err()
}
