// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// CheckMembership returns an error unless the process whose cgroup membership is listed at procCgroup
// runs in cgroupPath, or in a cgroup below it.
//
// Each line is hierarchy-ID:controllers:path: a single one on the unified hierarchy, and one per
// hierarchy with cgroup v1, where any hierarchy placing the process there will do.
func CheckMembership(procCgroup, cgroupPath string) error {
	// every path is below an empty one
	if cgroupPath == "" {
		return errors.New("no cgroup to check against")
	}

	contents, err := os.ReadFile(procCgroup)
	if err != nil {
		return err
	}

	var found []string

	for line := range strings.Lines(string(contents)) {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) != 3 {
			continue
		}

		path := fields[2]

		if path == cgroupPath || strings.HasPrefix(path, cgroupPath+"/") {
			return nil
		}

		found = append(found, path)
	}

	return fmt.Errorf("it runs in cgroup %q, not %q", strings.Join(found, ", "), cgroupPath)
}
