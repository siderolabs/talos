// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proc

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// List returns a list of all PIDs via /proc.
func List() ([]int, error) {
	proc, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}

	defer proc.Close() //nolint: errcheck

	names, err := proc.Readdirnames(0)

	// ignore error if some entries were returned
	if err != nil && len(names) == 0 {
		return nil, err
	}

	pids := make([]int, 0, len(names))

	for _, name := range names {
		pid, err := strconv.Atoi(name)
		if err != nil {
			// /proc contains other entries which are not process IDs
			continue
		}

		ppid, err := getPPID(name)

		// skip processes we can't read stat for
		// also skip processes with:
		//   * PPID == 0 (init and kthreadd)
		//   * PPID == 2 (owned by kthreadd, kernel thread proc)
		if err != nil || ppid == 0 || ppid == 2 {
			continue
		}

		pids = append(pids, pid)
	}

	return pids, nil
}

func getPPID(pid string) (int, error) {
	stat, err := os.Open(filepath.Join("/proc", pid, "stat"))
	if err != nil {
		return -1, err
	}

	defer stat.Close() //nolint: errcheck

	scanner := bufio.NewScanner(stat)
	if !scanner.Scan() {
		return -1, scanner.Err()
	}

	line := scanner.Text()

	inParens := false
	field := 0
	ppid := ""

	for i, ch := range line {
		switch ch {
		case '(':
			inParens = true
		case ')':
			inParens = false
		case ' ':
			if inParens {
				continue
			}

			field++

			if field == 3 {
				ppid = strings.SplitN(line[i+1:], " ", 2)[0]
			}
		}
	}

	if ppid != "" {
		return strconv.Atoi(ppid)
	}

	return -1, nil
}
