// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"path/filepath"
	"testing"
)

func TestLookupAgainstTalosFileContexts(t *testing.T) {
	// tools/labeled-squashfs -> internal/pkg/selinux/policy/file_contexts
	fc := filepath.Join("..", "..", "internal", "pkg", "selinux", "policy", "file_contexts")

	rules, err := parseFileContexts(fc)
	if err != nil {
		t.Fatalf("parse %s: %v", fc, err)
	}

	if len(rules) == 0 {
		t.Fatalf("no rules parsed")
	}

	cases := []struct {
		path    string
		ft      fileType
		want    string
		wantHit bool
	}{
		// Exact-match literal entries.
		{"/usr/bin/init", typeReg, "system_u:object_r:init_exec_t:s0", true},
		{"/usr/bin/poweroff", typeAny, "system_u:object_r:init_exec_t:s0", true},
		{"/usr/bin/runc", typeAny, "system_u:object_r:containerd_exec_t:s0", true},
		// Regex match: /etc(/.*)? covers both /etc and /etc/foo.
		{"/etc", typeDir, "system_u:object_r:etc_t:s0", true},
		{"/etc/cni/00-foo.conf", typeReg, "system_u:object_r:cni_conf_t:s0", true},
		// More specific rule should win (last-match-wins).
		{"/usr/bin/foo", typeReg, "system_u:object_r:bin_exec_t:s0", true},
		{"/usr/lib/modules/somemod.ko", typeReg, "system_u:object_r:module_t:s0", true},
		// Root entry.
		{"/", typeDir, "system_u:object_r:rootfs_t:s0", true},
	}

	for _, tc := range cases {
		got := lookup(rules, tc.path, tc.ft)
		if tc.wantHit && got != tc.want {
			t.Errorf("lookup(%q, %v) = %q, want %q", tc.path, tc.ft, got, tc.want)
		}

		if !tc.wantHit && got != "" {
			t.Errorf("lookup(%q, %v) = %q, want no match", tc.path, tc.ft, got)
		}
	}
}

func TestParseTypeSpec(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want fileType
	}{
		{"--", typeReg},
		{"-d", typeDir},
		{"-l", typeLnk},
		{"-c", typeChr},
		{"-b", typeBlk},
		{"-p", typeFifo},
		{"-s", typeSock},
	} {
		got, err := parseTypeSpec(tc.in)
		if err != nil {
			t.Errorf("parseTypeSpec(%q): unexpected err %v", tc.in, err)

			continue
		}

		if got != tc.want {
			t.Errorf("parseTypeSpec(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	if _, err := parseTypeSpec("-x"); err == nil {
		t.Errorf("parseTypeSpec(-x): expected error")
	}
}
