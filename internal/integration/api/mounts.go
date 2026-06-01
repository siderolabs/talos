// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// MountsSuite verifies mount flag policy on a running node.
//
// Policy (see siderolabs/talos#11946):
//   - every rw mount must carry MOUNT_ATTR_NOSUID, MOUNT_ATTR_NOEXEC,
//     MOUNT_ATTR_NODEV unless explicitly exempt
//   - device nodes are not allowed outside /dev and /dev/pts: NODEV is
//     non-negotiable for every other mountpoint
type MountsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements suite.NamedSuite.
func (suite *MountsSuite) SuiteName() string {
	return "api.MountsSuite"
}

// SetupTest sets up the test context.
func (suite *MountsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping mounts test since provisioner is not qemu")
	}
}

// TearDownTest cancels the test context.
func (suite *MountsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// mountInfo is one parsed entry from /proc/self/mountinfo.
type mountInfo struct {
	mountPoint string
	fsType     string
	source     string
	options    map[string]struct{} // per-mount options (field 6)
}

func (m mountInfo) has(opt string) bool {
	_, ok := m.options[opt]

	return ok
}

// parseMountInfo parses /proc/self/mountinfo per Linux kernel docs:
// fields[4] = mount point, fields[5] = per-mount options, after " - ":
// fstype, source, super-options.
func parseMountInfo(r io.Reader) ([]mountInfo, error) {
	var out []mountInfo

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		pre, post, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}

		preFields := strings.Fields(pre)
		postFields := strings.Fields(post)

		if len(preFields) < 6 || len(postFields) < 2 {
			continue
		}

		opts := make(map[string]struct{})
		for o := range strings.SplitSeq(preFields[5], ",") {
			opts[o] = struct{}{}
		}

		out = append(out, mountInfo{
			mountPoint: preFields[4],
			options:    opts,
			fsType:     postFields[0],
			source:     postFields[1],
		})
	}

	return out, scanner.Err()
}

// nodevExempt returns true for mountpoints where device nodes are legitimate.
// Only devtmpfs at /dev and devpts at /dev/pts qualify.
func nodevExempt(m mountInfo) bool {
	switch {
	case m.fsType == "devtmpfs" && m.mountPoint == "/dev":
		return true
	case m.fsType == "devpts" && m.mountPoint == "/dev/pts":
		return true
	}

	return false
}

// workloadManagedPrefixes lists mount path prefixes that are created by
// kubelet, containerd, or CNI plugins — not by Talos. Their flags are out
// of scope for the Talos mount policy.
var workloadManagedPrefixes = []string{
	"/run/containerd/io.containerd.",
	"/run/netns/",
	"/var/lib/kubelet/pods/",
}

func workloadManaged(m mountInfo) bool {
	for _, p := range workloadManagedPrefixes {
		if strings.HasPrefix(m.mountPoint, p) {
			return true
		}
	}

	return false
}

// noexecExemptPrefixes lists mount path prefixes where executing binaries
// is part of the design. Read-only mounts are exempt elsewhere via the
// `ro` option. /var (EPHEMERAL) is intentionally NOT exempt: containerd
// container exec goes through overlay rootfs at /run/containerd/.../rootfs
// which is a separate mount with its own flags.
var noexecExemptPrefixes = []string{
	"/opt",                               // CNI plugins, containerd plugins
	"/usr/libexec/kubernetes",            // kubelet plugins
	"/usr/lib/udev",                      // udev helpers
	constants.ExtensionServiceRootfsPath, // /usr/local/lib/containers — extension service rootfs overlays (iscsid, etc.)
}

func noexecExempt(m mountInfo) bool {
	if m.has("ro") {
		return true
	}

	// devtmpfs and hugetlbfs cannot host regular executable files in any
	// way that a userspace exec() would care about; systemd matches this
	// stance (see mount_table in systemd/src/shared/mount-setup.c — no
	// MS_NOEXEC on /dev).
	switch m.fsType {
	case "devtmpfs", "hugetlbfs":
		return true
	}

	for _, p := range noexecExemptPrefixes {
		if m.mountPoint == p || strings.HasPrefix(m.mountPoint, p+"/") {
			return true
		}
	}

	return false
}

// TestNodevPolicy asserts every mount outside /dev and /dev/pts carries nodev.
func (suite *MountsSuite) TestNodevPolicy() {
	suite.runPolicy("nodev", nodevExempt, "device nodes only in /dev and /dev/pts")
}

// TestNosuidPolicy asserts every mount carries nosuid. Talos has no
// legitimate SUID surface — even read-only signed rootfs/extension
// squashfs mounts ship no setuid binaries, so no exemptions.
func (suite *MountsSuite) TestNosuidPolicy() {
	suite.runPolicy("nosuid", func(m mountInfo) bool {
		return false
	}, "no SUID binaries anywhere in Talos")
}

// TestNoexecPolicy asserts every rw mount carries noexec, except
// documented exemptions (EPHEMERAL, /opt/cni, kubelet plugins, udev
// helpers). Read-only mounts are exempt (signed rootfs / extension
// squashfs).
func (suite *MountsSuite) TestNoexecPolicy() {
	suite.runPolicy("noexec", noexecExempt,
		"binaries should only execute from RO or explicitly exempt mounts")
}

func (suite *MountsSuite) runPolicy(opt string, exempt func(mountInfo) bool, rationale string) {
	for _, node := range suite.DiscoverNodeInternalIPs(suite.ctx) {
		suite.Run(node, func() {
			suite.checkOptOnNode(node, opt, exempt, rationale)
		})
	}
}

func (suite *MountsSuite) checkOptOnNode(node, opt string, exempt func(mountInfo) bool, rationale string) {
	mounts := suite.readMountInfo(node)

	var violations []string

	for _, m := range mounts {
		if workloadManaged(m) || exempt(m) {
			continue
		}

		// /var honors the EPHEMERAL VolumeConfig's mount.secure setting; when
		// the cluster was deployed with secure=false skip the assertion to match
		// the configured policy rather than the secure-by-default one.
		if suite.SkipEphemeralPolicy && m.mountPoint == constants.EphemeralMountPoint {
			continue
		}

		if !m.has(opt) {
			violations = append(
				violations,
				fmt.Sprintf("%s (fstype=%s, source=%s)", m.mountPoint, m.fsType, m.source),
			)
		}
	}

	suite.Assert().Empty(
		violations,
		"mounts missing %s (policy: %s):\n  %s",
		opt, rationale, strings.Join(violations, "\n  "),
	)
}

// readMountInfo fetches and parses /proc/self/mountinfo from a node.
func (suite *MountsSuite) readMountInfo(node string) []mountInfo {
	nodeCtx := client.WithNode(suite.ctx, node)

	r, err := suite.Client.Read(nodeCtx, "/proc/self/mountinfo")
	suite.Require().NoError(err)

	defer r.Close() //nolint:errcheck

	mounts, err := parseMountInfo(r)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(mounts)

	return mounts
}

func init() {
	allSuites = append(allSuites, new(MountsSuite))
}
