// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runtime-spec/specs-go"
	pprocfs "github.com/prometheus/procfs"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-blockdevice/blockdevice/partition/gpt"
	"github.com/siderolabs/go-blockdevice/blockdevice/util"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/siderolabs/go-cmd/pkg/cmd/proc"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sys/unix"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	installer "github.com/siderolabs/talos/cmd/installer/pkg/install"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/disk"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/emergency"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/internal/pkg/cri"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/internal/pkg/install"
	"github.com/siderolabs/talos/internal/pkg/logind"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/kernel/kspp"
	"github.com/siderolabs/talos/pkg/kubernetes"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	metamachinery "github.com/siderolabs/talos/pkg/machinery/meta"
	resourcefiles "github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	resourceruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	resourcev1alpha1 "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/minimal"
)

// WaitForUSB represents the WaitForUSB task.
func WaitForUSB(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		// Wait for USB storage in the case that the install disk is supplied over
		// USB. If we don't wait, there is the chance that we will fail to detect the
		// install disk.
		file := "/sys/module/usb_storage/parameters/delay_use"

		_, err := os.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}

		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		val := strings.TrimSuffix(string(b), "\n")

		var i int

		i, err = strconv.Atoi(val)
		if err != nil {
			return err
		}

		logger.Printf("waiting %d second(s) for USB storage", i)

		time.Sleep(time.Duration(i) * time.Second)

		return nil
	}, "waitForUSB"
}

// EnforceKSPPRequirements represents the EnforceKSPPRequirements task.
func EnforceKSPPRequirements(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = resourceruntime.NewKernelParamsSetCondition(r.State().V1Alpha2().Resources(), kspp.GetKernelParams()...).Wait(ctx); err != nil {
			return err
		}

		return kspp.EnforceKSPPKernelParameters()
	}, "enforceKSPPRequirements"
}

// SetupSystemDirectory represents the SetupSystemDirectory task.
func SetupSystemDirectory(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, p := range []string{constants.SystemEtcPath, constants.SystemVarPath, constants.StateMountPoint} {
			if err = os.MkdirAll(p, 0o700); err != nil {
				return err
			}
		}

		for _, p := range []string{constants.SystemRunPath} {
			if err = os.MkdirAll(p, 0o751); err != nil {
				return err
			}
		}

		return nil
	}, "setupSystemDirectory"
}

// CreateSystemCgroups represents the CreateSystemCgroups task.
//
//nolint:gocyclo
func CreateSystemCgroups(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// in container mode cgroups mode depends on cgroups provided by the container runtime
		if r.State().Platform().Mode() != runtime.ModeContainer {
			// assert that cgroupsv2 is being used when running not in container mode,
			// as Talos sets up cgroupsv2 on its own
			if cgroups.Mode() != cgroups.Unified && !mount.ForceGGroupsV1() {
				return errors.New("cgroupsv2 should be used")
			}
		}

		// Initialize cgroups root path.
		if err = cgroup.InitRoot(); err != nil {
			return fmt.Errorf("error initializing cgroups root path: %w", err)
		}

		logger.Printf("using cgroups root: %s", cgroup.Root())

		groups := []struct {
			name      string
			resources *cgroup2.Resources
		}{
			{
				name: constants.CgroupInit,
				resources: &cgroup2.Resources{
					Memory: &cgroup2.Memory{
						Min: pointer.To[int64](constants.CgroupInitReservedMemory),
						Low: pointer.To[int64](constants.CgroupInitReservedMemory * 2),
					},
				},
			},
			{
				name: constants.CgroupSystem,
				resources: &cgroup2.Resources{
					Memory: &cgroup2.Memory{
						Min: pointer.To[int64](constants.CgroupSystemReservedMemory),
						Low: pointer.To[int64](constants.CgroupSystemReservedMemory * 2),
					},
				},
			},
			{
				name:      constants.CgroupSystemRuntime,
				resources: &cgroup2.Resources{},
			},
			{
				name:      constants.CgroupUdevd,
				resources: &cgroup2.Resources{},
			},
			{
				name: constants.CgroupPodRuntime,
				resources: &cgroup2.Resources{
					Memory: &cgroup2.Memory{
						Min: pointer.To[int64](constants.CgroupPodRuntimeReservedMemory),
						Low: pointer.To[int64](constants.CgroupPodRuntimeReservedMemory * 2),
					},
				},
			},
			{
				name: constants.CgroupKubelet,
				resources: &cgroup2.Resources{
					Memory: &cgroup2.Memory{
						Min: pointer.To[int64](constants.CgroupKubeletReservedMemory),
						Low: pointer.To[int64](constants.CgroupKubeletReservedMemory * 2),
					},
				},
			},
			{
				name: constants.CgroupDashboard,
				resources: &cgroup2.Resources{
					Memory: &cgroup2.Memory{
						Min: pointer.To[int64](constants.CgroupDashboardReservedMemory),
						Low: pointer.To[int64](constants.CgroupDashboardLowMemory),
					},
				},
			},
		}

		for _, c := range groups {
			if cgroups.Mode() == cgroups.Unified {
				resources := c.resources

				if r.State().Platform().Mode() == runtime.ModeContainer {
					// don't attempt to set resources in container mode, as they might conflict with the parent cgroup tree
					resources = &cgroup2.Resources{}
				}

				cg, err := cgroup2.NewManager(constants.CgroupMountPath, cgroup.Path(c.name), resources)
				if err != nil {
					return fmt.Errorf("failed to create cgroup: %w", err)
				}

				if c.name == constants.CgroupInit {
					if err := cg.AddProc(uint64(os.Getpid())); err != nil {
						return fmt.Errorf("failed to move init process to cgroup: %w", err)
					}
				}
			} else {
				cg, err := cgroup1.New(cgroup1.StaticPath(c.name), &specs.LinuxResources{})
				if err != nil {
					return fmt.Errorf("failed to create cgroup: %w", err)
				}

				if c.name == constants.CgroupInit {
					if err := cg.Add(cgroup1.Process{
						Pid: os.Getpid(),
					}); err != nil {
						return fmt.Errorf("failed to move init process to cgroup: %w", err)
					}
				}
			}
		}

		return nil
	}, "CreateSystemCgroups"
}

// MountBPFFS represents the MountBPFFS task.
func MountBPFFS(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.BPFMountPoints()
		if err != nil {
			return err
		}

		return mount.Mount(mountpoints)
	}, "mountBPFFS"
}

// MountCgroups represents the MountCgroups task.
func MountCgroups(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.CGroupMountPoints()
		if err != nil {
			return err
		}

		return mount.Mount(mountpoints)
	}, "mountCgroups"
}

// MountPseudoFilesystems represents the MountPseudoFilesystems task.
func MountPseudoFilesystems(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.PseudoSubMountPoints()
		if err != nil {
			return err
		}

		return mount.Mount(mountpoints)
	}, "mountPseudoFilesystems"
}

// SetRLimit represents the SetRLimit task.
func SetRLimit(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// TODO(andrewrynhard): Should we read limit from /proc/sys/fs/nr_open?
		return unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 1048576, Max: 1048576})
	}, "setRLimit"
}

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
func WriteIMAPolicy(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if _, err = os.Stat("/sys/kernel/security/ima/policy"); os.IsNotExist(err) {
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

		return nil
	}, "writeIMAPolicy"
}

// OSRelease renders a valid /etc/os-release file and writes it to disk. The
// node's OS Image field is reported by the node from /etc/os-release.
func OSRelease() (err error) {
	if err = createBindMount(filepath.Join(constants.SystemEtcPath, "os-release"), "/etc/os-release"); err != nil {
		return err
	}

	contents, err := version.OSRelease()
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(constants.SystemEtcPath, "os-release"), contents, 0o644)
}

// createBindMount creates a common way to create a writable source file with a
// bind mounted destination. This is most commonly used for well known files
// under /etc that need to be adjusted during startup.
func createBindMount(src, dst string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(src, os.O_WRONLY|os.O_CREATE, 0o644); err != nil {
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	if err = unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for %s: %w", dst, err)
	}

	return nil
}

// CreateOSReleaseFile represents the CreateOSReleaseFile task.
func CreateOSReleaseFile(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// Create /etc/os-release.
		return OSRelease()
	}, "createOSReleaseFile"
}

// LoadConfig represents the LoadConfig task.
func LoadConfig(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		// create a request to initialize the process acquisition process
		request := resourcev1alpha1.NewAcquireConfigSpec()
		if err := r.State().V1Alpha2().Resources().Create(ctx, request); err != nil {
			return fmt.Errorf("failed to create config request: %w", err)
		}

		// wait for the config to be acquired
		status := resourcev1alpha1.NewAcquireConfigStatus()
		if _, err := r.State().V1Alpha2().Resources().WatchFor(ctx, status.Metadata(), state.WithEventTypes(state.Created)); err != nil {
			return err
		}

		// clean up request to make sure controller doesn't work after this point
		return r.State().V1Alpha2().Resources().Destroy(ctx, request.Metadata())
	}, "loadConfig"
}

// SaveConfig represents the SaveConfig task.
func SaveConfig(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var b []byte

		b, err = r.ConfigContainer().Bytes()
		if err != nil {
			return err
		}

		return os.WriteFile(constants.ConfigPath, b, 0o600)
	}, "saveConfig"
}

// MemorySizeCheck represents the MemorySizeCheck task.
func MemorySizeCheck(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if r.State().Platform().Mode() == runtime.ModeContainer {
			log.Println("skipping memory size check in the container")

			return nil
		}

		pc, err := pprocfs.NewDefaultFS()
		if err != nil {
			return fmt.Errorf("failed to open procfs: %w", err)
		}

		info, err := pc.Meminfo()
		if err != nil {
			return fmt.Errorf("failed to read meminfo: %w", err)
		}

		minimum, recommended, err := minimal.Memory(r.Config().Machine().Type())
		if err != nil {
			return err
		}

		switch memTotal := pointer.SafeDeref(info.MemTotal) * humanize.KiByte; {
		case memTotal < minimum:
			log.Println("WARNING: memory size is less than recommended")
			log.Println("WARNING: Talos may not work properly")
			log.Println("WARNING: minimum memory size is", minimum/humanize.MiByte, "MiB")
			log.Println("WARNING: recommended memory size is", recommended/humanize.MiByte, "MiB")
			log.Println("WARNING: current total memory size is", memTotal/humanize.MiByte, "MiB")
		case memTotal < recommended:
			log.Println("NOTE: recommended memory size is", recommended/humanize.MiByte, "MiB")
			log.Println("NOTE: current total memory size is", memTotal/humanize.MiByte, "MiB")
		default:
			log.Println("memory size is OK")
			log.Println("memory size is", memTotal/humanize.MiByte, "MiB")
		}

		return nil
	}, "memorySizeCheck"
}

// DiskSizeCheck represents the DiskSizeCheck task.
func DiskSizeCheck(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if r.State().Platform().Mode() == runtime.ModeContainer {
			log.Println("skipping disk size check in the container")

			return nil
		}

		disk := r.State().Machine().Disk() // get ephemeral disk state
		if disk == nil {
			return errors.New("failed to get ephemeral disk state")
		}

		diskSize, err := disk.Size()
		if err != nil {
			return fmt.Errorf("failed to get ephemeral disk size: %w", err)
		}

		if minimum := minimal.DiskSize(); diskSize < minimum {
			log.Println("WARNING: disk size is less than recommended")
			log.Println("WARNING: Talos may not work properly")
			log.Println("WARNING: minimum recommended disk size is", minimum/humanize.MiByte, "MiB")
			log.Println("WARNING: current total disk size is", diskSize/humanize.MiByte, "MiB")
		} else {
			log.Println("disk size is OK")
			log.Println("disk size is", diskSize/humanize.MiByte, "MiB")
		}

		return nil
	}, "diskSizeCheck"
}

// SetUserEnvVars represents the SetUserEnvVars task.
func SetUserEnvVars(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, env := range environment.Get(r.Config()) {
			key, val, _ := strings.Cut(env, "=")

			if err = os.Setenv(key, val); err != nil {
				return fmt.Errorf("failed to set enivronment variable: %w", err)
			}
		}

		return nil
	}, "setUserEnvVars"
}

// StartContainerd represents the task to start containerd.
func StartContainerd(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svc := &services.Containerd{}

		system.Services(r).LoadAndStart(svc)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx)
	}, "startContainerd"
}

// WriteUdevRules is the task that writes udev rules to a udev rules file.
// TODO: frezbo: move this to controller based since writing udev rules doesn't need a restart.
func WriteUdevRules(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		rules := r.Config().Machine().Udev().Rules()

		var content strings.Builder

		for _, rule := range rules {
			content.WriteString(strings.ReplaceAll(rule, "\n", "\\\n"))
			content.WriteByte('\n')
		}

		if err = os.WriteFile(constants.UdevRulesPath, []byte(content.String()), 0o644); err != nil {
			return fmt.Errorf("failed writing custom udev rules: %w", err)
		}

		if len(rules) > 0 {
			if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "control", "--reload"); err != nil {
				return err
			}

			if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "trigger", "--type=devices", "--action=add"); err != nil {
				return err
			}

			if _, err := cmd.RunContext(ctx, "/sbin/udevadm", "trigger", "--type=subsystems", "--action=add"); err != nil {
				return err
			}

			// This ensures that `udevd` finishes processing kernel events, triggered by
			// `udevd trigger`, to prevent a race condition when a user specifies a path
			// under `/dev/disk/*` in any disk definitions.
			_, err := cmd.RunContext(ctx, "/sbin/udevadm", "settle", "--timeout=50")

			return err
		}

		return nil
	}, "writeUdevRules"
}

// StartMachined represents the task to start machined.
func StartMachined(_ runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if err := tpm2.PCRExtent(secureboot.UKIPCR, []byte(secureboot.EnterMachined)); err != nil {
			return err
		}

		svc := &services.Machined{}

		id := svc.ID(r)

		err := system.Services(r).Start(id)
		if err != nil {
			return fmt.Errorf("failed to start machined service: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventUp, id).Wait(ctx)
	}, "startMachined"
}

// StartSyslogd represents the task to start syslogd.
func StartSyslogd(r runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
	return func(_ context.Context, _ *log.Logger, r runtime.Runtime) error {
		system.Services(r).LoadAndStart(&services.Syslogd{})

		return nil
	}, "startSyslogd"
}

// StartDashboard represents the task to start dashboard.
func StartDashboard(_ runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
	return func(_ context.Context, _ *log.Logger, r runtime.Runtime) error {
		system.Services(r).LoadAndStart(&services.Dashboard{})

		return nil
	}, "startDashboard"
}

// StartUdevd represents the task to start udevd.
func StartUdevd(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		mp := mount.NewMountPoints()
		mp.Set("udev-data", mount.NewMountPoint("", constants.UdevDir, "", unix.MS_I_VERSION, "", mount.WithFlags(mount.Overlay|mount.SystemOverlay|mount.Shared)))

		if err = mount.Mount(mp); err != nil {
			return err
		}

		svc := &services.Udevd{}

		system.Services(r).LoadAndStart(svc)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx)
	}, "startUdevd"
}

// ExtendPCRStartAll represents the task to extend the PCR with the StartTheWorld PCR phase.
func ExtendPCRStartAll(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return tpm2.PCRExtent(secureboot.UKIPCR, []byte(secureboot.StartTheWorld))
	}, "extendPCRStartAll"
}

// StartAllServices represents the task to start the system services.
func StartAllServices(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// nb: Treating the beginning of "service starts" as the activate event for a normal
		// non-maintenance mode boot. At this point, we'd expect the user to
		// start interacting with the system for troubleshooting at least.
		platform.FireEvent(
			ctx,
			r.State().Platform(),
			platform.Event{
				Type:    platform.EventTypeActivate,
				Message: "Talos is ready for user interaction.",
			},
		)

		svcs := system.Services(r)

		// load the kubelet service, but don't start it;
		// KubeletServiceController will start it once it's ready.
		svcs.Load(
			&services.Kubelet{},
		)

		serviceList := []system.Service{
			&services.CRI{},
		}

		switch t := r.Config().Machine().Type(); t {
		case machine.TypeInit:
			serviceList = append(serviceList,
				&services.Trustd{},
				&services.Etcd{Bootstrap: true},
			)
		case machine.TypeControlPlane:
			serviceList = append(serviceList,
				&services.Trustd{},
				&services.Etcd{},
			)
		case machine.TypeWorker:
			// nothing
		case machine.TypeUnknown:
			fallthrough
		default:
			panic(fmt.Sprintf("unexpected machine type %v", t))
		}

		svcs.LoadAndStart(serviceList...)

		var all []conditions.Condition

		logger.Printf("waiting for %d services", len(svcs.List()))

		for _, svc := range svcs.List() {
			cond := system.WaitForService(system.StateEventUp, svc.AsProto().GetId())
			all = append(all, cond)
		}

		ctx, cancel := context.WithTimeout(ctx, constants.BootTimeout)
		defer cancel()

		aggregateCondition := conditions.WaitForAll(all...)

		errChan := make(chan error)

		go func() {
			errChan <- aggregateCondition.Wait(ctx)
		}()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			logger.Printf("%s", aggregateCondition.String())

			select {
			case err := <-errChan:
				return err
			case <-ticker.C:
			}
		}
	}, "startAllServices"
}

// StopServicesEphemeral represents the StopServicesEphemeral task.
func StopServicesEphemeral(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// stopping 'cri' service stops everything which depends on it (kubelet, etcd, ...)
		return system.Services(nil).StopWithRevDepenencies(ctx, "cri", "udevd", "trustd")
	}, "stopServicesForUpgrade"
}

// StopAllServices represents the StopAllServices task.
func StopAllServices(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		system.Services(nil).Shutdown(ctx)

		return nil
	}, "stopAllServices"
}

// MountOverlayFilesystems represents the MountOverlayFilesystems task.
func MountOverlayFilesystems(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.OverlayMountPoints()
		if err != nil {
			return err
		}

		return mount.Mount(mountpoints)
	}, "mountOverlayFilesystems"
}

// SetupSharedFilesystems represents the SetupSharedFilesystems task.
func SetupSharedFilesystems(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		targets := []string{"/", "/var", "/etc/cni", "/run"}
		for _, t := range targets {
			if err = unix.Mount("", t, "", unix.MS_SHARED|unix.MS_REC, ""); err != nil {
				return err
			}
		}

		return nil
	}, "setupSharedFilesystems"
}

// SetupVarDirectory represents the SetupVarDirectory task.
func SetupVarDirectory(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, p := range []string{"/var/log/audit", "/var/log/containers", "/var/log/pods", "/var/lib/kubelet", "/var/run/lock", constants.SeccompProfilesDirectory} {
			if err = os.MkdirAll(p, 0o700); err != nil {
				return err
			}
		}

		// Handle Kubernetes directories which need different ownership
		for _, p := range []string{constants.KubernetesAuditLogDir} {
			if err = os.MkdirAll(p, 0o700); err != nil {
				return err
			}

			if err = os.Chown(p, constants.KubernetesAPIServerRunUser, constants.KubernetesAPIServerRunGroup); err != nil {
				return fmt.Errorf("failed to chown %s: %w", p, err)
			}
		}

		return nil
	}, "setupVarDirectory"
}

// MountUserDisks represents the MountUserDisks task.
func MountUserDisks(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = partitionAndFormatDisks(logger, r); err != nil {
			return err
		}

		return mountDisks(logger, r)
	}, "mountUserDisks"
}

// TODO(andrewrynhard): We shouldn't pull in the installer command package
// here.
func partitionAndFormatDisks(logger *log.Logger, r runtime.Runtime) error {
	m := &installer.Manifest{
		Devices: map[string]installer.Device{},
		Targets: map[string][]*installer.Target{},
		Printf:  logger.Printf,
	}

	for _, disk := range r.Config().Machine().Disks() {
		if err := func() error {
			bd, err := blockdevice.Open(disk.Device(), blockdevice.WithMode(blockdevice.ReadonlyMode), blockdevice.WithExclusiveLock(true))
			if err != nil {
				return err
			}

			deviceName := bd.Device().Name()

			if disk.Device() != deviceName {
				logger.Printf("using device name %q instead of %q", deviceName, disk.Device())
			}

			//nolint:errcheck
			defer bd.Close()

			var pt *gpt.GPT

			pt, err = bd.PartitionTable()
			if err != nil {
				if !errors.Is(err, blockdevice.ErrMissingPartitionTable) {
					return err
				}
			}

			// Partitions will be created/recreated if either of the following
			//  conditions are true:
			// - a partition table exists AND there are no partitions
			// - a partition table does not exist

			if pt != nil {
				if len(pt.Partitions().Items()) > 0 {
					logger.Printf(("skipping setup of %q, found existing partitions"), deviceName)

					return nil
				}
			}

			m.Devices[deviceName] = installer.Device{
				Device:                 deviceName,
				ResetPartitionTable:    true,
				SkipOverlayMountsCheck: true,
			}

			for _, part := range disk.Partitions() {
				extraTarget := &installer.Target{
					Device: deviceName,
					FormatOptions: &partition.FormatOptions{
						Force:          true,
						FileSystemType: partition.FilesystemTypeXFS,
					},
					Options: &partition.Options{
						Size:          part.Size(),
						PartitionType: partition.LinuxFilesystemData,
					},
				}

				m.Targets[deviceName] = append(m.Targets[deviceName], extraTarget)
			}

			return nil
		}(); err != nil {
			return err
		}
	}

	return m.Execute()
}

func mountDisks(logger *log.Logger, r runtime.Runtime) (err error) {
	mountpoints := mount.NewMountPoints()

	for _, disk := range r.Config().Machine().Disks() {
		bd, err := blockdevice.Open(disk.Device(), blockdevice.WithMode(blockdevice.ReadonlyMode), blockdevice.WithExclusiveLock(true))
		if err != nil {
			return err
		}

		deviceName := bd.Device().Name()

		if disk.Device() != deviceName {
			logger.Printf("using device name %q instead of %q", deviceName, disk.Device())
		}

		if err = bd.Close(); err != nil {
			return err
		}

		for i, part := range disk.Partitions() {
			var partname string

			partname, err = util.PartPath(deviceName, i+1)
			if err != nil {
				return err
			}

			if _, err = os.Stat(part.MountPoint()); errors.Is(err, os.ErrNotExist) {
				if err = os.MkdirAll(part.MountPoint(), 0o700); err != nil {
					return err
				}
			}

			mountpoints.Set(partname,
				mount.NewMountPoint(partname, part.MountPoint(), "xfs", unix.MS_NOATIME, "",
					mount.WithProjectQuota(r.Config().Machine().Features().DiskQuotaSupportEnabled()),
				),
			)
		}
	}

	return mount.Mount(mountpoints)
}

// WriteUserFiles represents the WriteUserFiles task.
//
//nolint:gocyclo,cyclop
func WriteUserFiles(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var result *multierror.Error

		files, err := r.Config().Machine().Files()
		if err != nil {
			return fmt.Errorf("error generating extra files: %w", err)
		}

		for _, f := range files {
			content := f.Content()

			switch f.Op() {
			case "create":
				// Allow create at all times.
			case "overwrite":
				if err = existsAndIsFile(f.Path()); err != nil {
					result = multierror.Append(result, err)

					continue
				}
			case "append":
				if err = existsAndIsFile(f.Path()); err != nil {
					result = multierror.Append(result, err)

					continue
				}

				var existingFileContents []byte

				existingFileContents, err = os.ReadFile(f.Path())
				if err != nil {
					result = multierror.Append(result, err)

					continue
				}

				content = string(existingFileContents) + "\n" + f.Content()
			default:
				result = multierror.Append(result, fmt.Errorf("unknown operation for file %q: %q", f.Path(), f.Op()))

				continue
			}

			if filepath.Dir(f.Path()) == constants.ManifestsDirectory {
				if err = os.WriteFile(f.Path(), []byte(content), f.Permissions()); err != nil {
					result = multierror.Append(result, err)

					continue
				}

				if err = os.Chmod(f.Path(), f.Permissions()); err != nil {
					result = multierror.Append(result, err)

					continue
				}

				continue
			}

			// CRI configuration customization
			if f.Path() == filepath.Join("/etc", constants.CRICustomizationConfigPart) {
				if err = injectCRIConfigPatch(ctx, r.State().V1Alpha2().Resources(), []byte(f.Content())); err != nil {
					result = multierror.Append(result, err)
				}

				continue
			}

			// Determine if supplied path is in /var or not.
			// If not, we'll write it to /var anyways and bind mount below
			p := f.Path()
			inVar := true
			parts := strings.Split(
				strings.TrimLeft(f.Path(), "/"),
				string(os.PathSeparator),
			)

			if parts[0] != "var" {
				p = filepath.Join("/var", f.Path())
				inVar = false
			}

			// We do not want to support creating new files anywhere outside of
			// /var. If a valid use case comes up, we can reconsider then.
			if !inVar && f.Op() == "create" {
				return fmt.Errorf("create operation not allowed outside of /var: %q", f.Path())
			}

			if err = os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				result = multierror.Append(result, err)

				continue
			}

			if err = os.WriteFile(p, []byte(content), f.Permissions()); err != nil {
				result = multierror.Append(result, err)

				continue
			}

			if err = os.Chmod(p, f.Permissions()); err != nil {
				result = multierror.Append(result, err)

				continue
			}

			if !inVar {
				if err = unix.Mount(p, f.Path(), "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
					result = multierror.Append(result, fmt.Errorf("failed to create bind mount for %s: %w", p, err))
				}
			}
		}

		return result.ErrorOrNil()
	}, "writeUserFiles"
}

func injectCRIConfigPatch(ctx context.Context, st state.State, content []byte) error {
	// limit overall waiting time
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ch := make(chan state.Event)

	// wait for the CRI config to be created
	if err := st.Watch(ctx, resourcefiles.NewEtcFileSpec(resourcefiles.NamespaceName, constants.CRIConfig).Metadata(), ch); err != nil {
		return err
	}

	// first update should be received about the existing resource
	select {
	case <-ch:
	case <-ctx.Done():
		return ctx.Err()
	}

	etcFileSpec := resourcefiles.NewEtcFileSpec(resourcefiles.NamespaceName, constants.CRICustomizationConfigPart)
	etcFileSpec.TypedSpec().Mode = 0o600
	etcFileSpec.TypedSpec().Contents = content

	if err := st.Create(ctx, etcFileSpec); err != nil {
		return err
	}

	// wait for the CRI config parts controller to generate the merged file
	var version resource.Version

	select {
	case ev := <-ch:
		version = ev.Resource.Metadata().Version()
	case <-ctx.Done():
		return ctx.Err()
	}

	// wait for the file to be rendered
	_, err := st.WatchFor(ctx, resourcefiles.NewEtcFileStatus(resourcefiles.NamespaceName, constants.CRIConfig).Metadata(), state.WithCondition(func(r resource.Resource) (bool, error) {
		fileStatus, ok := r.(*resourcefiles.EtcFileStatus)
		if !ok {
			return false, nil
		}

		return fileStatus.TypedSpec().SpecVersion == version.String(), nil
	}))

	return err
}

func existsAndIsFile(p string) (err error) {
	var info os.FileInfo

	info, err = os.Stat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return fmt.Errorf("file must exist: %q", p)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("invalid mode: %q", info.Mode().String())
	}

	return nil
}

// UnmountOverlayFilesystems represents the UnmountOverlayFilesystems task.
func UnmountOverlayFilesystems(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.OverlayMountPoints()
		if err != nil {
			return err
		}

		return mount.Unmount(mountpoints)
	}, "unmountOverlayFilesystems"
}

// UnmountUserDisks represents the UnmountUserDisks task.
func UnmountUserDisks(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if r.Config() == nil {
			return nil
		}

		mountpoints := mount.NewMountPoints()

		for _, disk := range r.Config().Machine().Disks() {
			bd, err := blockdevice.Open(disk.Device(), blockdevice.WithMode(blockdevice.ReadonlyMode))
			if err != nil {
				return err
			}

			deviceName := bd.Device().Name()

			if deviceName != disk.Device() {
				logger.Printf("using device name %q instead of %q", deviceName, disk.Device())
			}

			if err = bd.Close(); err != nil {
				return err
			}

			for i, part := range disk.Partitions() {
				var partname string

				partname, err = util.PartPath(deviceName, i+1)
				if err != nil {
					return err
				}

				mountpoints.Set(partname, mount.NewMountPoint(partname, part.MountPoint(), "xfs", unix.MS_NOATIME, ""))
			}
		}

		return mount.Unmount(mountpoints)
	}, "unmountUserDisks"
}

// UnmountPodMounts represents the UnmountPodMounts task.
func UnmountPodMounts(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var b []byte

		if b, err = os.ReadFile("/proc/self/mounts"); err != nil {
			return err
		}

		rdr := bytes.NewReader(b)

		scanner := bufio.NewScanner(rdr)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())

			if len(fields) < 2 {
				continue
			}

			mountpoint := fields[1]
			if strings.HasPrefix(mountpoint, constants.EphemeralMountPoint+"/") {
				logger.Printf("unmounting %s\n", mountpoint)

				if err = mount.SafeUnmount(ctx, logger, mountpoint); err != nil {
					if errors.Is(err, syscall.EINVAL) {
						log.Printf("ignoring unmount error %s: %v", mountpoint, err)
					} else {
						return fmt.Errorf("error unmounting %s: %w", mountpoint, err)
					}
				}
			}
		}

		return scanner.Err()
	}, "unmountPodMounts"
}

// UnmountSystemDiskBindMounts represents the UnmountSystemDiskBindMounts task.
func UnmountSystemDiskBindMounts(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		systemDisk := r.State().Machine().Disk()
		if systemDisk == nil {
			return nil
		}

		devname := systemDisk.BlockDevice.Device().Name()

		f, err := os.Open("/proc/mounts")
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())

			if len(fields) < 2 {
				continue
			}

			device := strings.ReplaceAll(fields[0], "/dev/mapper", "/dev")
			mountpoint := fields[1]

			if strings.HasPrefix(device, devname) && device != devname {
				logger.Printf("unmounting %s\n", mountpoint)

				if err = mount.SafeUnmount(ctx, logger, mountpoint); err != nil {
					if errors.Is(err, syscall.EINVAL) {
						log.Printf("ignoring unmount error %s: %v", mountpoint, err)
					} else {
						return fmt.Errorf("error unmounting %s: %w", mountpoint, err)
					}
				}
			}
		}

		return scanner.Err()
	}, "unmountSystemDiskBindMounts"
}

// CordonAndDrainNode represents the task for stop all containerd tasks in the
// k8s.io namespace.
func CordonAndDrainNode(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// skip not exist error as it means that the node hasn't fully joined yet
		if _, err = os.Stat("/var/lib/kubelet/pki/kubelet-client-current.pem"); err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}

		var nodename string

		if nodename, err = r.NodeName(); err != nil {
			return err
		}

		// controllers will automatically cordon the node when the node enters appropriate phase,
		// so here we just wait for the node to be cordoned
		if err = waitForNodeCordoned(ctx, logger, r, nodename); err != nil {
			return err
		}

		var kubeHelper *kubernetes.Client

		if kubeHelper, err = kubernetes.NewClientFromKubeletKubeconfig(); err != nil {
			return err
		}

		defer kubeHelper.Close() //nolint:errcheck

		return kubeHelper.Drain(ctx, nodename)
	}, "cordonAndDrainNode"
}

func waitForNodeCordoned(ctx context.Context, logger *log.Logger, r runtime.Runtime, nodename string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	logger.Print("waiting for node to be cordoned")

	_, err := r.State().V1Alpha2().Resources().WatchFor(
		ctx,
		k8s.NewNodeStatus(k8s.NamespaceName, nodename).Metadata(),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			if resource.IsTombstone(r) {
				return false, nil
			}

			nodeStatus, ok := r.(*k8s.NodeStatus)
			if !ok {
				return false, nil
			}

			return nodeStatus.TypedSpec().Unschedulable, nil
		}),
	)

	return err
}

// LeaveEtcd represents the task for removing a control plane node from etcd.
//
//nolint:gocyclo
func LeaveEtcd(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		_, err = os.Stat(filepath.Join(constants.EtcdDataPath, "/member"))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}

		etcdID := (&services.Etcd{}).ID(r)

		services := system.Services(r).List()

		shouldLeaveEtcd := false

		for _, service := range services {
			if service.AsProto().Id != etcdID {
				continue
			}

			switch service.GetState() { //nolint:exhaustive
			case events.StateRunning:
				fallthrough
			case events.StateStopping:
				fallthrough
			case events.StateFailed:
				shouldLeaveEtcd = true
			}

			break
		}

		if !shouldLeaveEtcd {
			return nil
		}

		client, err := etcd.NewClientFromControlPlaneIPs(ctx, r.State().V1Alpha2().Resources())
		if err != nil {
			return fmt.Errorf("failed to create etcd client: %w", err)
		}

		//nolint:errcheck
		defer client.Close()

		ctx = clientv3.WithRequireLeader(ctx)

		if err = client.LeaveCluster(ctx, r.State().V1Alpha2().Resources()); err != nil {
			return fmt.Errorf("failed to leave cluster: %w", err)
		}

		return nil
	}, "leaveEtcd"
}

// RemoveAllPods represents the task for stopping and removing all pods.
func RemoveAllPods(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return stopAndRemoveAllPods(cri.StopAndRemove), "removeAllPods"
}

// StopAllPods represents the task for stopping all pods.
func StopAllPods(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return stopAndRemoveAllPods(cri.StopOnly), "stopAllPods"
}

func waitForKubeletLifecycleFinalizers(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
	logger.Printf("waiting for kubelet lifecycle finalizers")

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	lifecycle := resource.NewMetadata(k8s.NamespaceName, k8s.KubeletLifecycleType, k8s.KubeletLifecycleID, resource.VersionUndefined)

	for {
		ok, err := r.State().V1Alpha2().Resources().Teardown(ctx, lifecycle)
		if err != nil {
			return err
		}

		if ok {
			break
		}

		_, err = r.State().V1Alpha2().Resources().WatchFor(ctx, lifecycle, state.WithFinalizerEmpty())
		if err != nil {
			return err
		}
	}

	return r.State().V1Alpha2().Resources().Destroy(ctx, lifecycle)
}

func stopAndRemoveAllPods(stopAction cri.StopAction) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = waitForKubeletLifecycleFinalizers(ctx, logger, r); err != nil {
			logger.Printf("failed waiting for kubelet lifecycle finalizers: %s", err)
		}

		logger.Printf("shutting down kubelet gracefully")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(ctx, logind.InhibitMaxDelay)
		defer shutdownCtxCancel()

		if err = r.State().Machine().DBus().WaitShutdown(shutdownCtx); err != nil {
			logger.Printf("failed waiting for inhibit shutdown lock: %s", err)
		}

		if err = system.Services(nil).Stop(ctx, "kubelet"); err != nil {
			return err
		}

		// check that the CRI is running and the socket is available, if not, skip the rest
		if _, err = os.Stat(constants.CRIContainerdAddress); os.IsNotExist(err) {
			return nil
		}

		client, err := cri.NewClient("unix://"+constants.CRIContainerdAddress, 10*time.Second)
		if err != nil {
			return err
		}

		//nolint:errcheck
		defer client.Close()

		ctx, cancel := context.WithTimeout(ctx, time.Minute*3)
		defer cancel()

		// We remove pods with POD network mode first so that the CNI can perform
		// any cleanup tasks. If we don't do this, we run the risk of killing the
		// CNI, preventing the CRI from cleaning up the pod's networking.

		if err = client.StopAndRemovePodSandboxes(ctx, stopAction, runtimeapi.NamespaceMode_POD, runtimeapi.NamespaceMode_CONTAINER); err != nil {
			return err
		}

		// With the POD network mode pods out of the way, we kill the remaining
		// pods.

		return client.StopAndRemovePodSandboxes(ctx, stopAction)
	}
}

// ResetSystemDiskPartitions represents the task for wiping the system disk partitions.
func ResetSystemDiskPartitions(seq runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
	wipeStr := procfs.ProcCmdline().Get(constants.KernelParamWipe).First()
	reboot, _ := Reboot(seq, nil)

	if pointer.SafeDeref(wipeStr) == "" {
		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			return errors.New("no wipe target specified")
		}, "wipeSystemDisk"
	}

	if *wipeStr == "system" {
		resetSystemDisk, _ := ResetSystemDisk(seq, nil)

		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			logger.Printf("resetting system disks")

			err := resetSystemDisk(ctx, logger, r)
			if err != nil {
				logger.Printf("resetting system disks failed")

				return err
			}

			logger.Printf("finished resetting system disks")

			return reboot(ctx, logger, r) // only reboot when we wiped boot partition
		}, "wipeSystemDisk"
	}

	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		targets, err := parseTargets(r, *wipeStr)
		if err != nil {
			return err
		}

		fn, _ := ResetSystemDiskSpec(seq, targets)
		diskTargets := targets.GetSystemDiskTargets()

		logger.Printf("resetting system disks %s", diskTargets)

		err = fn(ctx, logger, r)
		if err != nil {
			logger.Printf("resetting system disks %s failed", diskTargets)

			return err
		}

		logger.Printf("finished resetting system disks %s", diskTargets)

		bootWiped := slices.ContainsFunc(diskTargets, func(t runtime.PartitionTarget) bool {
			return t.GetLabel() == constants.BootPartitionLabel
		})

		if bootWiped {
			return reboot(ctx, logger, r) // only reboot when we wiped boot partition
		}

		return nil
	}, "wipeSystemDiskPartitions"
}

// ResetSystemDisk represents the task to reset the system disk.
func ResetSystemDisk(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var dev *blockdevice.BlockDevice

		disk := r.State().Machine().Disk()

		if disk == nil {
			return nil
		}

		dev, err = blockdevice.Open(disk.Device().Name())
		if err != nil {
			return err
		}

		defer dev.Close() //nolint:errcheck

		return dev.FastWipe()
	}, "resetSystemDisk"
}

// ResetUserDisks represents the task to reset the user disks.
func ResetUserDisks(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		in, ok := data.(runtime.ResetOptions)
		if !ok {
			return errors.New("unexpected runtime data")
		}

		wipeDevice := func(deviceName string) error {
			dev, err := blockdevice.Open(deviceName)
			if err != nil {
				return err
			}

			defer func() {
				if closeErr := dev.Close(); closeErr != nil {
					logger.Printf("failed to close device %s: %s", deviceName, closeErr)
				}
			}()

			logger.Printf("wiping user disk %s", deviceName)

			return dev.FastWipe()
		}

		for _, deviceName := range in.GetUserDisksToWipe() {
			if err := wipeDevice(deviceName); err != nil {
				return err
			}
		}

		return nil
	}, "resetUserDisks"
}

type targets struct {
	systemDiskTargets []*installer.Target
}

func (opt targets) GetSystemDiskTargets() []runtime.PartitionTarget {
	return xslices.Map(opt.systemDiskTargets, func(t *installer.Target) runtime.PartitionTarget { return t })
}

func parseTargets(r runtime.Runtime, wipeStr string) (targets, error) {
	after, found := strings.CutPrefix(wipeStr, "system:")
	if !found {
		return targets{}, fmt.Errorf("invalid wipe labels string: %q", wipeStr)
	}

	var result []*installer.Target //nolint:prealloc

	for _, part := range strings.Split(after, ",") {
		bd := r.State().Machine().Disk().BlockDevice

		target, err := installer.ParseTarget(part, bd.Device().Name())
		if err != nil {
			return targets{}, fmt.Errorf("error parsing target label %q: %w", part, err)
		}

		pt, err := bd.PartitionTable()
		if err != nil {
			return targets{}, fmt.Errorf("error reading partition table: %w", err)
		}

		_, err = target.Locate(pt)
		if err != nil {
			return targets{}, fmt.Errorf("error locating partition %q: %w", part, err)
		}

		result = append(result, target)
	}

	if len(result) == 0 {
		return targets{}, errors.New("no wipe labels specified")
	}

	return targets{systemDiskTargets: result}, nil
}

// SystemDiskTargets represents the interface for getting the system disk targets.
// It's a subset of [runtime.ResetOptions].
type SystemDiskTargets interface {
	GetSystemDiskTargets() []runtime.PartitionTarget
}

// ResetSystemDiskSpec represents the task to reset the system disk by spec.
func ResetSystemDiskSpec(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		in, ok := data.(SystemDiskTargets)
		if !ok {
			return errors.New("unexpected runtime data")
		}

		for _, target := range in.GetSystemDiskTargets() {
			if err = target.Format(logger.Printf); err != nil {
				return fmt.Errorf("failed wiping partition %s: %w", target, err)
			}
		}

		stateWiped := slices.ContainsFunc(in.GetSystemDiskTargets(), func(t runtime.PartitionTarget) bool {
			return t.GetLabel() == constants.StatePartitionLabel
		})

		metaWiped := slices.ContainsFunc(in.GetSystemDiskTargets(), func(t runtime.PartitionTarget) bool {
			return t.GetLabel() == constants.MetaPartitionLabel
		})

		if stateWiped && !metaWiped {
			var removed bool

			removed, err = r.State().Machine().Meta().DeleteTag(ctx, meta.StateEncryptionConfig)
			if err != nil {
				return fmt.Errorf("failed to remove state encryption META config tag: %w", err)
			}

			if removed {
				if err = r.State().Machine().Meta().Flush(); err != nil {
					return fmt.Errorf("failed to flush META: %w", err)
				}

				logger.Printf("reset the state encryption META config tag")
			}
		}

		logger.Printf("successfully reset system disk by the spec")

		return nil
	}, "resetSystemDiskSpec"
}

// VerifyDiskAvailability represents the task for verifying that the system
// disk is not in use.
func VerifyDiskAvailability(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		devname := r.State().Machine().Disk().BlockDevice.Device().Name()

		// We MUST close this in order to avoid EBUSY.
		if err = r.State().Machine().Close(); err != nil {
			return err
		}

		// TODO(andrewrynhard): This should be more dynamic. If we ever change the
		// partition scheme there is the chance that 2 is not the correct parition to
		// check.
		partname, err := util.PartPath(devname, 2)
		if err != nil {
			return err
		}

		if _, err = os.Stat(partname); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("ephemeral partition not found: %w", err)
		}

		mountsReported := false

		return retry.Constant(3*time.Minute, retry.WithUnits(500*time.Millisecond)).Retry(func() error {
			if err = tryLock(partname); err != nil {
				if err == unix.EBUSY {
					if !mountsReported {
						// if disk is busy, report mounts for debugging purposes but just once
						// otherwise console might be flooded with messages
						dumpMounts(logger)

						mountsReported = true
					}

					return retry.ExpectedErrorf("ephemeral partition in use: %q", partname)
				}

				return fmt.Errorf("failed to verify ephemeral partition not in use: %w", err)
			}

			return nil
		})
	}, "verifyDiskAvailability"
}

func tryLock(path string) error {
	fd, errno := unix.Open(path, unix.O_RDONLY|unix.O_EXCL|unix.O_CLOEXEC, 0)

	//nolint:errcheck
	defer unix.Close(fd)

	return errno
}

func dumpMounts(logger *log.Logger) {
	mounts, err := os.Open("/proc/mounts")
	if err != nil {
		logger.Printf("failed to read mounts: %s", err)

		return
	}

	defer mounts.Close() //nolint:errcheck

	logger.Printf("contents of /proc/mounts:")

	_, _ = io.Copy(log.Writer(), mounts) //nolint:errcheck
}

// Upgrade represents the task for performing an upgrade.
func Upgrade(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// This should be checked by the gRPC server, but we double check here just
		// to be safe.
		in, ok := data.(*machineapi.UpgradeRequest)
		if !ok {
			return runtime.ErrInvalidSequenceData
		}

		devname := r.State().Machine().Disk().BlockDevice.Device().Name()

		logger.Printf("performing upgrade via %q", in.GetImage())

		// We pull the installer image when we receive an upgrade request. No need
		// to pull it again.
		err = install.RunInstallerContainer(
			devname, r.State().Platform().Name(),
			in.GetImage(),
			r.Config(),
			r.ConfigContainer(),
			install.OptionsFromUpgradeRequest(r, in)...,
		)
		if err != nil {
			return err
		}

		logger.Println("upgrade successful")

		return nil
	}, "upgrade"
}

// Reboot represents the Reboot task.
func Reboot(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		rebootCmd := unix.LINUX_REBOOT_CMD_RESTART

		if r.State().Machine().IsKexecPrepared() {
			rebootCmd = unix.LINUX_REBOOT_CMD_KEXEC
		}

		r.Events().Publish(ctx, &machineapi.RestartEvent{
			Cmd: int64(rebootCmd),
		})

		platform.FireEvent(
			ctx,
			r.State().Platform(),
			platform.Event{
				Type:    platform.EventTypeRebooted,
				Message: "Talos rebooted.",
			},
		)

		return runtime.RebootError{Cmd: rebootCmd}
	}, "reboot"
}

// Shutdown represents the Shutdown task.
func Shutdown(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		cmd := unix.LINUX_REBOOT_CMD_POWER_OFF

		if p := procfs.ProcCmdline().Get(constants.KernelParamShutdown).First(); p != nil {
			if *p == "halt" {
				cmd = unix.LINUX_REBOOT_CMD_HALT
			}
		}

		r.Events().Publish(ctx, &machineapi.RestartEvent{
			Cmd: int64(cmd),
		})

		return runtime.RebootError{Cmd: cmd}
	}, "shutdown"
}

// SaveStateEncryptionConfig saves state partition encryption info in the meta partition.
func SaveStateEncryptionConfig(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		config := r.Config()
		if config == nil {
			return nil
		}

		encryption := config.Machine().SystemDiskEncryption().Get(constants.StatePartitionLabel)
		if encryption == nil {
			return nil
		}

		var data []byte

		if data, err = json.Marshal(encryption); err != nil {
			return err
		}

		ok, err := r.State().Machine().Meta().SetTagBytes(ctx, meta.StateEncryptionConfig, data)
		if err != nil {
			return err
		}

		if !ok {
			return errors.New("failed to save state encryption config in the META partition")
		}

		return r.State().Machine().Meta().Flush()
	}, "SaveStateEncryptionConfig"
}

// MountEFIPartition mounts the EFI partition.
func MountEFIPartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return mount.SystemPartitionMount(ctx, r, logger, constants.EFIPartitionLabel)
	}, "mountEFIPartition"
}

// UnmountEFIPartition unmounts the EFI partition.
func UnmountEFIPartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return mount.SystemPartitionUnmount(r, logger, constants.EFIPartitionLabel)
	}, "unmountEFIPartition"
}

// MountStatePartition mounts the system partition.
func MountStatePartition(seq runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		flags := mount.SkipIfMounted

		if seq == runtime.SequenceInitialize {
			flags |= mount.SkipIfNoFilesystem
		}

		opts := []mount.Option{mount.WithFlags(flags)}

		var encryption config.Encryption
		// first try reading encryption from the config
		// which always has the priority here
		if r.Config() != nil && r.Config().Machine() != nil {
			encryption = r.Config().Machine().SystemDiskEncryption().Get(constants.StatePartitionLabel)
		}

		// then try reading it from the META partition
		if encryption == nil {
			var encryptionFromMeta *v1alpha1.EncryptionConfig

			data, ok := r.State().Machine().Meta().ReadTagBytes(meta.StateEncryptionConfig)
			if ok {
				if err = json.Unmarshal(data, &encryptionFromMeta); err != nil {
					return err
				}

				encryption = encryptionFromMeta
			}
		}

		if encryption != nil {
			opts = append(opts, mount.WithEncryptionConfig(encryption), mount.WithSystemInformationGetter(r.GetSystemInformation))
		}

		return mount.SystemPartitionMount(ctx, r, logger, constants.StatePartitionLabel, opts...)
	}, "mountStatePartition"
}

// UnmountStatePartition unmounts the system partition.
func UnmountStatePartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return mount.SystemPartitionUnmount(r, logger, constants.StatePartitionLabel)
	}, "unmountStatePartition"
}

// MountEphemeralPartition mounts the ephemeral partition.
func MountEphemeralPartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return mount.SystemPartitionMount(ctx, r, logger, constants.EphemeralPartitionLabel,
			mount.WithFlags(mount.Resize),
			mount.WithProjectQuota(r.Config().Machine().Features().DiskQuotaSupportEnabled()))
	}, "mountEphemeralPartition"
}

// UnmountEphemeralPartition unmounts the ephemeral partition.
func UnmountEphemeralPartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return mount.SystemPartitionUnmount(r, logger, constants.EphemeralPartitionLabel)
	}, "unmountEphemeralPartition"
}

// Install mounts or installs the system partitions.
func Install(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		switch {
		case !r.State().Machine().Installed():
			installerImage := r.Config().Machine().Install().Image()
			if installerImage == "" {
				installerImage = images.DefaultInstallerImage
			}

			var disk string

			disk, err = r.Config().Machine().Install().Disk()
			if err != nil {
				return err
			}

			err = install.RunInstallerContainer(
				disk,
				r.State().Platform().Name(),
				installerImage,
				r.Config(),
				r.ConfigContainer(),
				install.WithForce(true),
				install.WithZero(r.Config().Machine().Install().Zero()),
				install.WithExtraKernelArgs(r.Config().Machine().Install().ExtraKernelArgs()),
			)
			if err != nil {
				platform.FireEvent(
					ctx,
					r.State().Platform(),
					platform.Event{
						Type:    platform.EventTypeFailure,
						Message: "Talos install failed.",
						Error:   err,
					},
				)

				return err
			}

			platform.FireEvent(
				ctx,
				r.State().Platform(),
				platform.Event{
					Type:    platform.EventTypeInstalled,
					Message: "Talos installed successfully.",
				},
			)

			logger.Println("install successful")

		case r.State().Machine().IsInstallStaged():
			devname := r.State().Machine().Disk().BlockDevice.Device().Name()

			var options install.Options

			if err = json.Unmarshal(r.State().Machine().StagedInstallOptions(), &options); err != nil {
				return fmt.Errorf("error unserializing install options: %w", err)
			}

			logger.Printf("performing staged upgrade via %q", r.State().Machine().StagedInstallImageRef())

			err = install.RunInstallerContainer(
				devname, r.State().Platform().Name(),
				r.State().Machine().StagedInstallImageRef(),
				r.Config(),
				r.ConfigContainer(),
				install.WithOptions(options),
			)
			if err != nil {
				platform.FireEvent(
					ctx,
					r.State().Platform(),
					platform.Event{
						Type:    platform.EventTypeFailure,
						Message: "Talos staged upgrade failed.",
						Error:   err,
					},
				)

				return err
			}

			// nb: we don't fire an "activate" event after this one
			// b/c we'd only ever get here if Talos was already
			// installed I believe.
			platform.FireEvent(
				ctx,
				r.State().Platform(),
				platform.Event{
					Type:    platform.EventTypeUpgraded,
					Message: "Talos staged upgrade successful.",
				},
			)

			logger.Println("staged upgrade successful")

		default:
			return errors.New("unsupported configuration for install task")
		}

		return nil
	}, "install"
}

// ActivateLogicalVolumes represents the task for activating logical volumes.
func ActivateLogicalVolumes(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if _, err = cmd.Run("/sbin/lvm", "vgchange", "-ay"); err != nil {
			return fmt.Errorf("failed to activate logical volumes: %w", err)
		}

		return nil
	}, "activateLogicalVolumes"
}

// KexecPrepare loads next boot kernel via kexec_file_load.
//
//nolint:gocyclo
func KexecPrepare(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if req, ok := data.(*machineapi.RebootRequest); ok {
			if req.Mode == machineapi.RebootRequest_POWERCYCLE {
				log.Print("kexec skipped as reboot with power cycle was requested")

				return nil
			}
		}

		if r.Config() == nil {
			return nil
		}

		// check if partition with label BOOT exists
		if device := r.State().Machine().Disk(disk.WithPartitionLabel(constants.BootPartitionLabel)); device == nil {
			return nil
		}

		// BOOT partition exists and we can mount it
		if err := mount.SystemPartitionMount(ctx, r, logger, constants.BootPartitionLabel); err != nil {
			return err
		}

		defer mount.SystemPartitionUnmount(r, logger, constants.BootPartitionLabel) //nolint:errcheck

		conf, err := grub.Read(grub.ConfigPath)
		if err != nil {
			return err
		}

		if conf == nil {
			return nil
		}

		defaultEntry, ok := conf.Entries[conf.Default]
		if !ok {
			return nil
		}

		kernelPath := filepath.Join(constants.BootMountPoint, defaultEntry.Linux)
		initrdPath := filepath.Join(constants.BootMountPoint, defaultEntry.Initrd)

		kernel, err := os.Open(kernelPath)
		if err != nil {
			return err
		}

		defer kernel.Close() //nolint:errcheck

		initrd, err := os.Open(initrdPath)
		if err != nil {
			return err
		}

		defer initrd.Close() //nolint:errcheck

		cmdline := strings.TrimSpace(defaultEntry.Cmdline)

		if err = unix.KexecFileLoad(int(kernel.Fd()), int(initrd.Fd()), cmdline, 0); err != nil {
			switch {
			case errors.Is(err, unix.ENOSYS):
				log.Printf("kexec support is disabled in the kernel")

				return nil
			case errors.Is(err, unix.EPERM):
				log.Printf("kexec support is disabled via sysctl")

				return nil
			case errors.Is(err, unix.EBUSY):
				log.Printf("kexec is busy")

				return nil
			default:
				return fmt.Errorf("error loading kernel for kexec: %w", err)
			}
		}

		log.Printf("prepared kexec environment kernel=%q initrd=%q cmdline=%q", kernelPath, initrdPath, cmdline)

		r.State().Machine().KexecPrepared(true)

		return nil
	}, "kexecPrepare"
}

// StartDBus starts the D-Bus mock.
func StartDBus(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return r.State().Machine().DBus().Start()
	}, "startDBus"
}

// StopDBus stops the D-Bus mock.
func StopDBus(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if err := r.State().Machine().DBus().Stop(); err != nil {
			logger.Printf("error stopping D-Bus: %s, ignored", err)
		}

		return nil
	}, "stopDBus"
}

// ForceCleanup kills remaining procs and forces partitions unmount.
func ForceCleanup(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if err := proc.KillAll(); err != nil {
			logger.Printf("error killing all procs: %s", err)
		}

		if err := mount.UnmountAll(); err != nil {
			logger.Printf("error unmounting: %s", err)
		}

		return nil
	}, "forceCleanup"
}

// ReloadMeta reloads META partition after disk mount, installer run, etc.
//
//nolint:gocyclo
func ReloadMeta(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		err := r.State().Machine().Meta().Reload(ctx)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// attempt to populate meta from the environment if Talos is not installed (yet)
		if os.IsNotExist(err) {
			env := environment.Get(r.Config())

			prefix := constants.MetaValuesEnvVar + "="

			for _, e := range env {
				if !strings.HasPrefix(e, prefix) {
					continue
				}

				values, err := metamachinery.DecodeValues(e[len(prefix):])
				if err != nil {
					return fmt.Errorf("error decoding meta values: %w", err)
				}

				for _, value := range values {
					_, err = r.State().Machine().Meta().SetTag(ctx, value.Key, value.Value)
					if err != nil {
						return fmt.Errorf("error setting meta tag %x: %w", value.Key, err)
					}
				}
			}
		}

		if _, err := safe.ReaderGetByID[*resourceruntime.MetaLoaded](
			ctx,
			r.State().V1Alpha2().Resources(),
			resourceruntime.MetaLoadedID,
		); err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error reading MetaLoaded resource: %w", err)
			}

			// create MetaLoaded resource signaling that META is now loaded
			loaded := resourceruntime.NewMetaLoaded()
			loaded.TypedSpec().Done = true

			err = r.State().V1Alpha2().Resources().Create(ctx, loaded)
			if err != nil {
				return fmt.Errorf("error creating MetaLoaded resource: %w", err)
			}
		}

		return nil
	}, "reloadMeta"
}

// FlushMeta flushes META partition after install run.
func FlushMeta(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return r.State().Machine().Meta().Flush()
	}, "flushMeta"
}

// StoreShutdownEmergency stores shutdown emergency state.
func StoreShutdownEmergency(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		// for shutdown sequence, store power_off as the intent, it will be picked up
		// by emergency handled in machined/main.go if the Shutdown sequence fails
		emergency.RebootCmd.Store(unix.LINUX_REBOOT_CMD_POWER_OFF)

		return nil
	}, "storeShutdownEmergency"
}

// SendResetSignal func represents the task to send the final reset signal.
func SendResetSignal(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return r.State().V1Alpha2().Resources().Create(ctx, resourceruntime.NewMachineResetSignal())
	}, "sendResetSignal"
}

func pauseOnFailure(callback func(runtime.Sequence, any) (runtime.TaskExecutionFunc, string),
	timeout time.Duration,
) func(seq runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(seq runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
		f, name := callback(seq, data)

		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			err := f(ctx, logger, r)
			if err != nil {
				logger.Printf("%s failed, rebooting in %.0f minutes. You can use talosctl apply-config or talosctl edit mc to fix the issues, error:\n%s", name, timeout.Minutes(), err)

				timer := time.NewTimer(time.Minute * 5)
				defer timer.Stop()

				select {
				case <-timer.C:
				case <-ctx.Done():
				}
			}

			return err
		}, name
	}
}

func taskErrorHandler(handler func(error, *log.Logger) error, task runtime.TaskSetupFunc) runtime.TaskSetupFunc {
	return func(seq runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
		f, name := task(seq, data)

		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			err := f(ctx, logger, r)
			if err != nil {
				return handler(err, logger)
			}

			return nil
		}, name
	}
}

func phaseListErrorHandler(handler func(error, *log.Logger) error, phases ...runtime.Phase) PhaseList {
	for _, phase := range phases {
		for i, task := range phase.Tasks {
			phase.Tasks[i] = taskErrorHandler(handler, task)
		}
	}

	return phases
}

func logError(err error, logger *log.Logger) error {
	logger.Printf("WARNING: task failed: %s", err)

	return nil
}
