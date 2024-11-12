// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// See https://www.kernel.org/doc/Documentation/ABI/testing/ima_policy
var rules = []string{
	"dont_measure fsmagic=0x9fa0",     // PROC_SUPER_MAGIC
	"dont_measure fsmagic=0x62656572", // SYSFS_MAGIC
	"dont_measure fsmagic=0x64626720", // DEBUGFS_MAGIC
	"dont_measure fsmagic=0x1021994",  // TMPFS_MAGIC
	"dont_measure fsmagic=0x1cd1",     // DEVPTS_SUPER_MAGIC
	"dont_measure fsmagic=0x42494e4d", // BINFMTFS_MAGIC
	"dont_measure fsmagic=0x73636673", // SECURITYFS_MAGIC
	"dont_measure fsmagic=0xf97cff8c", // SELINUX_MAGIC
	"dont_measure fsmagic=0x43415d53", // SMACK_MAGIC
	"dont_measure fsmagic=0x27e0eb",   // CGROUP_SUPER_MAGIC
	"dont_measure fsmagic=0x63677270", // CGROUP2_SUPER_MAGIC
	"dont_measure fsmagic=0x6e736673", // NSFS_MAGIC
	"dont_measure fsmagic=0xde5e81e4", // EFIVARFS_MAGIC
	"dont_measure fsmagic=0x58465342", // XFS_MAGIC
	"dont_measure fsmagic=0x794c7630", // OVERLAYFS_SUPER_MAGIC
	"dont_measure fsmagic=0x9123683e", // BTRFS_SUPER_MAGIC
	"dont_measure fsmagic=0x72b6",     // JFFS2_SUPER_MAGIC
	"dont_measure fsmagic=0x4d44",     // MSDOS_SUPER_MAGIC
	"dont_measure fsmagic=0x2011bab0", // EXFAT_SUPER_MAGIC
	"dont_measure fsmagic=0x6969",     // NFS_SUPER_MAGIC
	"dont_measure fsmagic=0x5346544e", // NTFS_SB_MAGIC
	"dont_measure fsmagic=0x9660",     // ISOFS_SUPER_MAGIC
	"dont_measure fsmagic=0x15013346", // UDF_SUPER_MAGIC
	"dont_measure fsmagic=0x52654973", // REISERFS_SUPER_MAGIC
	"dont_measure fsmagic=0x137d",     // EXT_SUPER_MAGIC
	"dont_measure fsmagic=0xef51",     // EXT2_OLD_SUPER_MAGIC
	"dont_measure fsmagic=0xef53",     // EXT2_SUPER_MAGIC / EXT3_SUPER_MAGIC / EXT4_SUPER_MAGIC
	"dont_measure fsmagic=0x00c36400", // CEPH_SUPER_MAGIC
	"dont_measure fsmagic=0x65735543", // FUSE_CTL_SUPER_MAGIC
	"measure func=MMAP_CHECK mask=MAY_EXEC",
	"measure func=BPRM_CHECK mask=MAY_EXEC",
	"measure func=FILE_CHECK mask=^MAY_READ euid=0",
	"measure func=FILE_CHECK mask=^MAY_READ uid=0",
	"measure func=MODULE_CHECK",
	"measure func=FIRMWARE_CHECK",
	"measure func=POLICY_CHECK",
}

// WriteIMAPolicy represents the WriteIMAPolicy task.
func WriteIMAPolicy(ctx context.Context, log *zap.Logger, rt runtime.Runtime, next NextTaskFunc) error {
	if rt.State().Platform().Mode().InContainer() {
		return next()(ctx, log, rt, next)
	}

	if _, err := os.Stat("/sys/kernel/security/ima/policy"); os.IsNotExist(err) {
		return fmt.Errorf("policy file does not exist: %w", err)
	}

	f, err := os.OpenFile("/sys/kernel/security/ima/policy", os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	for _, line := range rules {
		if _, err = f.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("rule %q is invalid", err)
		}
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to close policy file: %w", err)
	}

	return next()(ctx, log, rt, next)
}
