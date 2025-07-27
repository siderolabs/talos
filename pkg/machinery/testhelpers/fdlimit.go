// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package testhelpers provides utilities to assist with testing.
package testhelpers

import (
	"syscall"
	"testing"
)

// SetTestFDLimit temporarily increases the file descriptor limit for tests.
// Returns the original limit to be restored later.
func SetTestFDLimit(t *testing.T) syscall.Rlimit {
	var rLimit syscall.Rlimit
	
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("Warning: Failed to get file descriptor limit: %v", err)
		return rLimit
	}
	
	// Save original limit
	origLimit := rLimit
	
	// Set higher limits for the test
	rLimit.Cur = 65536
	if rLimit.Max < rLimit.Cur {
		rLimit.Max = rLimit.Cur
	}
	
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("Warning: Failed to increase file descriptor limit: %v", err)
	}
	
	return origLimit
}

// ResetFDLimit restores the original file descriptor limit.
func ResetFDLimit(t *testing.T, origLimit syscall.Rlimit) {
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &origLimit)
	if err != nil {
		t.Logf("Warning: Failed to restore file descriptor limit: %v", err)
	}
}
