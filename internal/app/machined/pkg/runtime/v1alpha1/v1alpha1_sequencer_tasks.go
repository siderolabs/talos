// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/foxboron/go-uefi/efi"
	"github.com/hashicorp/go-multierror"
	pprocfs "github.com/prometheus/procfs"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"github.com/siderolabs/go-cmd/pkg/cmd/proc"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sys/unix"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/emergency"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/internal/pkg/cri"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/internal/pkg/install"
	"github.com/siderolabs/talos/internal/pkg/logind"
	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/kernel/kspp"
	"github.com/siderolabs/talos/pkg/kubernetes"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	metamachinery "github.com/siderolabs/talos/pkg/machinery/meta"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
	resourcefiles "github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	resourceruntime "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	resourcev1alpha1 "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
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

// Sleep represents the Sleep task.
func Sleep(d time.Duration) func(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(_ runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		}, "sleep"
	}
}

// MemorySizeCheck represents the MemorySizeCheck task.
func MemorySizeCheck(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if r.State().Platform().Mode() == runtime.ModeContainer {
			logger.Println("skipping memory size check in the container")

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
			logger.Println("WARNING: memory size is less than recommended")
			logger.Println("WARNING: Talos may not work properly")
			logger.Println("WARNING: minimum memory size is", minimum/humanize.MiByte, "MiB")
			logger.Println("WARNING: recommended memory size is", recommended/humanize.MiByte, "MiB")
			logger.Println("WARNING: current total memory size is", memTotal/humanize.MiByte, "MiB")
		case memTotal < recommended:
			logger.Println("NOTE: recommended memory size is", recommended/humanize.MiByte, "MiB")
			logger.Println("NOTE: current total memory size is", memTotal/humanize.MiByte, "MiB")
		default:
			logger.Println("memory size is OK")
			logger.Println("memory size is", memTotal/humanize.MiByte, "MiB")
		}

		return nil
	}, "memorySizeCheck"
}

// DiskSizeCheck represents the DiskSizeCheck task.
func DiskSizeCheck(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if r.State().Platform().Mode() == runtime.ModeContainer {
			logger.Println("skipping disk size check in the container")

			return nil
		}

		volumeStatus, err := r.State().V1Alpha2().Resources().WatchFor(ctx,
			blockres.NewVolumeStatus(blockres.NamespaceName, constants.EphemeralPartitionLabel).Metadata(),
			state.WithCondition(func(r resource.Resource) (bool, error) {
				volumeStatus, ok := r.(*blockres.VolumeStatus)
				if !ok {
					return false, nil
				}

				return volumeStatus.TypedSpec().Size > 0, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("error waiting for volume %q to be discovered: %w", constants.EphemeralPartitionLabel, err)
		}

		diskSize := volumeStatus.(*blockres.VolumeStatus).TypedSpec().Size

		if minimum := minimal.DiskSize(); diskSize < minimum {
			logger.Println("WARNING: disk size is less than recommended")
			logger.Println("WARNING: Talos may not work properly")
			logger.Println("WARNING: minimum recommended disk size is", minimum/humanize.MiByte, "MiB")
			logger.Println("WARNING: current total disk size is", diskSize/humanize.MiByte, "MiB")
		} else {
			logger.Println("disk size is OK")
			logger.Println("disk size is", diskSize/humanize.MiByte, "MiB")
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

		if err = selinux.SetLabel(constants.UdevRulesPath, constants.UdevRulesLabel); err != nil {
			return fmt.Errorf("failed labeling custom udev rules: %w", err)
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

// StartAuditd represents the task to start auditd.
func StartAuditd(r runtime.Sequence, _ any) (runtime.TaskExecutionFunc, string) {
	return func(_ context.Context, logger *log.Logger, r runtime.Runtime) error {
		if !r.State().Platform().Mode().InContainer() {
			disabledStr := procfs.ProcCmdline().Get(constants.KernelParamAuditdDisabled).First()
			disabled, _ := strconv.ParseBool(pointer.SafeDeref(disabledStr)) //nolint:errcheck

			if disabled {
				logger.Printf("auditd is disabled by kernel parameter %s", constants.KernelParamAuditdDisabled)

				return nil
			}
		}

		system.Services(r).LoadAndStart(&services.Auditd{})

		return nil
	}, "startAuditd"
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
		mp := mountv2.NewSystemOverlay([]string{constants.UdevDir}, constants.UdevDir, mountv2.WithShared(), mountv2.WithFlags(unix.MS_I_VERSION), mountv2.WithSelinuxLabel(constants.UdevRulesLabel))

		if _, err = mp.Mount(); err != nil {
			return err
		}

		var extraSettleTime time.Duration

		settleTimeStr := procfs.ProcCmdline().Get(constants.KernelParamDeviceSettleTime).First()
		if settleTimeStr != nil {
			extraSettleTime, err = time.ParseDuration(*settleTimeStr)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", constants.KernelParamDeviceSettleTime, err)
			}

			logger.Printf("extra settle time: %s", extraSettleTime)
		}

		svc := &services.Udevd{
			ExtraSettleTime: extraSettleTime,
		}

		system.Services(r).LoadAndStart(svc)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx)
	}, "startUdevd"
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
		return system.Services(nil).StopWithRevDepenencies(ctx, "cri", "trustd")
	}, "stopServicesForUpgrade"
}

// StopAllServices represents the StopAllServices task.
func StopAllServices(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		system.Services(nil).Shutdown(ctx)

		return nil
	}, "stopAllServices"
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

// MountUserDisks represents the MountUserDisks task.
func MountUserDisks(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		// wait for user disk config to be ready
		_, err := r.State().V1Alpha2().Resources().WatchFor(ctx,
			blockres.NewUserDiskConfigStatus(blockres.NamespaceName, blockres.UserDiskConfigStatusID).Metadata(),
			state.WithEventTypes(state.Created, state.Updated),
			state.WithCondition(func(r resource.Resource) (bool, error) {
				return r.(*blockres.UserDiskConfigStatus).TypedSpec().Ready, nil
			}),
		)

		return err
	}, "mountUserDisks"
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

	etcFileSpec := resourcefiles.NewEtcFileSpec(resourcefiles.NamespaceName, constants.CRICustomizationConfigPart)
	etcFileSpec.TypedSpec().Mode = 0o600
	etcFileSpec.TypedSpec().Contents = content
	etcFileSpec.TypedSpec().SelinuxLabel = constants.EtcSelinuxLabel

	if err := st.Create(ctx, etcFileSpec); err != nil {
		return err
	}

	checksumRaw := sha256.Sum256(content)
	expectedChecksum := hex.EncodeToString(checksumRaw[:])
	expectedAnnotation := resourcefiles.SourceFileAnnotation + ":" + filepath.Join("/etc", etcFileSpec.Metadata().ID())

	fileSpec, err := st.WatchFor(ctx, resourcefiles.NewEtcFileSpec(resourcefiles.NamespaceName, constants.CRIConfig).Metadata(),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			spec, ok := r.(*resourcefiles.EtcFileSpec)
			if !ok {
				return false, nil
			}

			value, ok := spec.Metadata().Annotations().Get(expectedAnnotation)

			return ok && value == expectedChecksum, nil
		}))
	if err != nil {
		return fmt.Errorf("error waiting for file %q to be updated: %w", constants.CRIConfig, err)
	}

	// wait for the file to be rendered
	_, err = st.WatchFor(ctx, resourcefiles.NewEtcFileStatus(resourcefiles.NamespaceName, constants.CRIConfig).Metadata(), state.WithCondition(func(r resource.Resource) (bool, error) {
		fileStatus, ok := r.(*resourcefiles.EtcFileStatus)
		if !ok {
			return false, nil
		}

		return fileStatus.TypedSpec().SpecVersion == fileSpec.Metadata().Version().String(), nil
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

				if err = mountv2.SafeUnmount(ctx, logger.Printf, mountpoint); err != nil {
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
//
//nolint:gocyclo
func UnmountSystemDiskBindMounts(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		ephemeralStatus, err := safe.StateGetByID[*blockres.VolumeStatus](ctx, r.State().V1Alpha2().Resources(), constants.EphemeralPartitionLabel)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if ephemeralStatus == nil {
			return nil
		}

		devname := ephemeralStatus.TypedSpec().MountLocation

		if devname == "" {
			return nil
		}

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

			device, mountpoint := fields[0], fields[1]

			if device != devname || mountpoint == constants.EphemeralMountPoint {
				continue
			}

			logger.Printf("unmounting %s\n", mountpoint)

			if err = mountv2.SafeUnmount(ctx, logger.Printf, mountpoint); err != nil {
				if errors.Is(err, syscall.EINVAL) {
					log.Printf("ignoring unmount error %s: %v", mountpoint, err)
				} else {
					return fmt.Errorf("error unmounting %s: %w", mountpoint, err)
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
			logger.Printf("failed to stop and remove pods with POD network mode: %s", err)
		}

		// With the POD network mode pods out of the way, we kill the remaining
		// pods.

		if err = client.StopAndRemovePodSandboxes(ctx, stopAction); err != nil {
			logger.Printf("failed to stop and remove pods: %s", err)
		}

		return nil
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
		return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
			systemDiskPaths, err := blockres.GetSystemDiskPaths(ctx, r.State().V1Alpha2().Resources())
			if err != nil {
				return err
			}

			targets := targets{
				systemDiskPaths: systemDiskPaths,
			}

			logger.Printf("resetting system disks")

			resetSystemDisk, _ := ResetSystemDisk(seq, targets)

			err = resetSystemDisk(ctx, logger, r)
			if err != nil {
				logger.Printf("resetting system disks failed")

				return err
			}

			logger.Printf("finished resetting system disks")

			return reboot(ctx, logger, r) // only reboot when we wiped boot partition
		}, "wipeSystemDisk"
	}

	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		targets, err := parseTargets(ctx, r, *wipeStr)
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

		return reboot(ctx, logger, r)
	}, "wipeSystemDiskPartitions"
}

// ResetSystemDisk represents the task to reset the system disk.
//
//nolint:gocyclo
func ResetSystemDisk(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		in, ok := data.(SystemDiskTargets)
		if !ok {
			return errors.New("unexpected runtime data")
		}

		for _, systemDiskPath := range in.GetSystemDiskPaths() {
			if err := func(devPath string) error {
				logger.Printf("wiping system disk %s", devPath)

				dev, err := block.NewFromPath(devPath, block.OpenForWrite())
				if err != nil {
					return err
				}

				if err = dev.RetryLockWithTimeout(ctx, true, time.Minute); err != nil {
					return fmt.Errorf("failed to lock device %s: %w", devPath, err)
				}

				defer dev.Close() //nolint:errcheck

				return dev.FastWipe()
			}(systemDiskPath); err != nil {
				return fmt.Errorf("failed to wipe system disk %s: %w", systemDiskPath, err)
			}
		}

		return nil
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
			dev, err := block.NewFromPath(deviceName, block.OpenForWrite())
			if err != nil {
				return err
			}

			defer func() {
				if closeErr := dev.Close(); closeErr != nil {
					logger.Printf("failed to close device %s: %s", deviceName, closeErr)
				}
			}()

			if err = dev.RetryLockWithTimeout(ctx, true, time.Minute); err != nil {
				return fmt.Errorf("failed to lock device %s: %w", deviceName, err)
			}

			defer dev.Unlock() //nolint:errcheck

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
	systemDiskTargets []*partition.VolumeWipeTarget
	systemDiskPaths   []string
}

var _ SystemDiskTargets = targets{}

func (opt targets) GetSystemDiskTargets() []runtime.PartitionTarget {
	return xslices.Map(opt.systemDiskTargets, func(t *partition.VolumeWipeTarget) runtime.PartitionTarget { return t })
}

func (opt targets) GetSystemDiskPaths() []string {
	return opt.systemDiskPaths
}

func (opt targets) String() string {
	return strings.Join(xslices.Map(opt.systemDiskTargets, func(t *partition.VolumeWipeTarget) string { return t.String() }), ", ")
}

func parseTargets(ctx context.Context, r runtime.Runtime, wipeStr string) (SystemDiskTargets, error) {
	after, found := strings.CutPrefix(wipeStr, "system:")
	if !found {
		return targets{}, fmt.Errorf("invalid wipe labels string: %q", wipeStr)
	}

	var result []*partition.VolumeWipeTarget

	// in this early phase, we don't have VolumeStatus resources, so instead we'd use DiscoveredVolumes
	// to get the volume paths
	discoveredVolumes, err := safe.StateListAll[*blockres.DiscoveredVolume](ctx, r.State().V1Alpha2().Resources())
	if err != nil {
		return targets{}, err
	}

	for _, label := range strings.Split(after, ",") {
		found := false

		for discoveredVolume := range discoveredVolumes.All() {
			if discoveredVolume.TypedSpec().PartitionLabel != label {
				continue
			}

			result = append(result, partition.VolumeWipeTargetFromDiscoveredVolume(discoveredVolume))

			found = true

			break
		}

		if !found {
			return targets{}, fmt.Errorf("failed to get volume status with label %q", label)
		}
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
	GetSystemDiskPaths() []string
	fmt.Stringer
}

// ResetSystemDiskSpec represents the task to reset the system disk by spec.
func ResetSystemDiskSpec(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		in, ok := data.(SystemDiskTargets)
		if !ok {
			return errors.New("unexpected runtime data")
		}

		for _, target := range in.GetSystemDiskTargets() {
			if err = target.Wipe(ctx, logger.Printf); err != nil {
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

			removed, err = r.State().Machine().Meta().DeleteTag(ctx, metamachinery.StateEncryptionConfig)
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

// Upgrade represents the task for performing an upgrade.
func Upgrade(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// This should be checked by the gRPC server, but we double check here just
		// to be safe.
		in, ok := data.(*machineapi.UpgradeRequest)
		if !ok {
			return runtime.ErrInvalidSequenceData
		}

		systemDisk, err := blockres.GetSystemDisk(ctx, r.State().V1Alpha2().Resources())
		if err != nil {
			return err
		}

		if systemDisk == nil {
			return fmt.Errorf("system disk not found")
		}

		devname := systemDisk.DevPath

		logger.Printf("performing upgrade via %q", in.GetImage())

		// We pull the installer image when we receive an upgrade request. No need
		// to pull it again.
		err = install.RunInstallerContainer(
			devname, r.State().Platform().Name(),
			in.GetImage(),
			r.Config(),
			r.ConfigContainer(),
			crires.RegistryBuilder(r.State().V1Alpha2().Resources()),
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

		ok, err := r.State().Machine().Meta().SetTagBytes(ctx, metamachinery.StateEncryptionConfig, data)
		if err != nil {
			return err
		}

		if !ok {
			return errors.New("failed to save state encryption config in the META partition")
		}

		return r.State().Machine().Meta().Flush()
	}, "SaveStateEncryptionConfig"
}

// haltIfInstalled halts the boot process if Talos is installed to disk but booted from ISO.
func haltIfInstalled(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		ctx, cancel := context.WithTimeout(ctx, constants.BootTimeout)
		defer cancel()

		timer := time.NewTicker(30 * time.Second)
		defer timer.Stop()

		for {
			logger.Printf("Talos is already installed to disk but booted from another media and %s kernel parameter is set. Please reboot from the disk.", constants.KernelParamHaltIfInstalled)

			select {
			case <-timer.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}, "haltIfInstalled"
}

// CleanupBootloader cleans up the ununsed bootloader if booted from a disk image with both bootloaders present.
func CleanupBootloader(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		systemDisk, err := blockres.GetSystemDisk(ctx, r.State().V1Alpha2().Resources())
		if err != nil {
			return err
		}

		if systemDisk == nil {
			return nil // no system disk, we can't do anything
		}

		if err := bootloader.CleanupBootloader(systemDisk.DevPath, sdboot.IsBootedUsingSDBoot()); err != nil {
			return err
		}

		if _, err := r.State().Machine().Meta().DeleteTag(ctx, metamachinery.DiskImageBootloader); err != nil {
			return fmt.Errorf("failed to delete tag %q: %w", metamachinery.DiskImageBootloader, err)
		}

		return r.State().Machine().Meta().Flush()
	}, "cleanupBootloader"
}

// MountEphemeralPartition mounts the ephemeral partition.
func MountEphemeralPartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		mountRequest := blockres.NewVolumeMountRequest(blockres.NamespaceName, constants.EphemeralPartitionLabel)
		mountRequest.TypedSpec().VolumeID = constants.EphemeralPartitionLabel
		mountRequest.TypedSpec().Requester = "sequencer"

		if err := r.State().V1Alpha2().Resources().Create(ctx, mountRequest); err != nil {
			return fmt.Errorf("failed to create EPHEMERAL mount request: %w", err)
		}

		if _, err := r.State().V1Alpha2().Resources().WatchFor(
			ctx,
			blockres.NewVolumeMountStatus(blockres.NamespaceName, constants.EphemeralPartitionLabel).Metadata(),
			state.WithEventTypes(state.Created, state.Updated),
		); err != nil {
			return fmt.Errorf("failed to wait for EPHEMERAL to be mounted: %w", err)
		}

		return nil
	}, "mountEphemeralPartition"
}

// UnmountEphemeralPartition unmounts the ephemeral partition.
func UnmountEphemeralPartition(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		mountRequest := blockres.NewVolumeMountRequest(blockres.NamespaceName, constants.EphemeralPartitionLabel).Metadata()

		err := r.State().V1Alpha2().Resources().Destroy(ctx, mountRequest)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return fmt.Errorf("failed to destroy EPHEMERAL mount request: %w", err)
		}

		return nil
	}, "unmountEphemeralPartition"
}

// Install mounts or installs the system partitions.
//
//nolint:gocyclo
func Install(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		switch {
		case !r.State().Machine().Installed():
			installerImage := r.Config().Machine().Install().Image()
			if installerImage == "" {
				installerImage = images.DefaultInstallerImage
			}

			logger.Printf("waiting for the image cache")

			if err = crires.WaitForImageCache(ctx, r.State().V1Alpha2().Resources()); err != nil {
				return fmt.Errorf("failed to wait for the image cache: %w", err)
			}

			var disk string

			matchExpr, err := r.Config().Machine().Install().DiskMatchExpression()
			if err != nil {
				return fmt.Errorf("failed to get disk match expression: %w", err)
			}

			switch {
			case matchExpr != nil:
				logger.Printf("using disk match expression: %s", matchExpr)

				matchedDisks, err := blockhelpers.MatchDisks(ctx, r.State().V1Alpha2().Resources(), matchExpr)
				if err != nil {
					return err
				}

				if len(matchedDisks) == 0 {
					return fmt.Errorf("no disks matched the expression: %s", matchExpr)
				}

				disk = matchedDisks[0].TypedSpec().DevPath
			case r.Config().Machine().Install().Disk() != "":
				disk = r.Config().Machine().Install().Disk()
			}

			disk, err = filepath.EvalSymlinks(disk)
			if err != nil {
				return err
			}

			logger.Printf("installing Talos to disk %s", disk)

			err = install.RunInstallerContainer(
				disk,
				r.State().Platform().Name(),
				installerImage,
				r.Config(),
				r.ConfigContainer(),
				crires.RegistryBuilder(r.State().V1Alpha2().Resources()),
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

			logger.Printf("waiting for the image cache copy")

			if err = crires.WaitForImageCacheCopy(ctx, r.State().V1Alpha2().Resources()); err != nil {
				return fmt.Errorf("failed to wait for the image cache: %w", err)
			}
		case r.State().Machine().IsInstallStaged():
			systemDisk, err := blockres.GetSystemDisk(ctx, r.State().V1Alpha2().Resources())
			if err != nil {
				return err
			}

			if systemDisk == nil {
				return fmt.Errorf("system disk not found")
			}

			devname := systemDisk.DevPath

			var options install.Options

			if err = json.Unmarshal(r.State().Machine().StagedInstallOptions(), &options); err != nil {
				return fmt.Errorf("error unserializing install options: %w", err)
			}

			logger.Printf("waiting for the image cache")

			if err = crires.WaitForImageCache(ctx, r.State().V1Alpha2().Resources()); err != nil {
				return fmt.Errorf("failed to wait for the image cache: %w", err)
			}

			logger.Printf("performing staged upgrade via %q", r.State().Machine().StagedInstallImageRef())

			err = install.RunInstallerContainer(
				devname, r.State().Platform().Name(),
				r.State().Machine().StagedInstallImageRef(),
				r.Config(),
				r.ConfigContainer(),
				crires.RegistryBuilder(r.State().V1Alpha2().Resources()),
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

// KexecPrepare loads next boot kernel via kexec_file_load.
func KexecPrepare(_ runtime.Sequence, data any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		if req, ok := data.(*machineapi.RebootRequest); ok {
			if req.Mode == machineapi.RebootRequest_POWERCYCLE {
				log.Print("kexec skipped as reboot with power cycle was requested")

				return nil
			}
		}

		if efi.GetSecureBoot() {
			log.Print("kexec skipped as secure boot is enabled")

			return nil
		}

		systemDisk, err := blockres.GetSystemDisk(ctx, r.State().V1Alpha2().Resources())
		if err != nil {
			return err
		}

		if systemDisk == nil {
			log.Print("kexec skipped as system disk is not found")

			return nil // no system disk, no kexec
		}

		dev, err := block.NewFromPath(systemDisk.DevPath)
		if err != nil {
			return err
		}

		defer dev.Close() //nolint:errcheck

		if err = dev.RetryLockWithTimeout(ctx, false, 3*time.Minute); err != nil {
			log.Print("kexec skipped as system disk is busy")

			return nil
		}

		defer dev.Unlock() //nolint:errcheck

		bootloaderInfo, err := bootloader.Probe(systemDisk.DevPath, options.ProbeOptions{})
		if err != nil {
			return fmt.Errorf("failed to probe system disk: %w", err)
		}

		return bootloaderInfo.KexecLoad(r, systemDisk.DevPath)
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

		if err := mountv2.UnmountAll(); err != nil {
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
		// META partition should be created at this point.
		if _, err := waitForVolumeReady(ctx, r, constants.MetaPartitionLabel); err != nil {
			return err
		}

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

// WaitForCARoots represents the WaitForCARoots task.
//
//nolint:gocyclo
func WaitForCARoots(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// watch EtcFileSpec & Status for CA roots and ensure they match
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		ch := make(chan state.Event)

		if err = r.State().V1Alpha2().Resources().Watch(ctx, resourcefiles.NewEtcFileSpec(resourcefiles.NamespaceName, constants.DefaultTrustedRelativeCAFile).Metadata(), ch); err != nil {
			return err
		}

		if err = r.State().V1Alpha2().Resources().Watch(ctx, resourcefiles.NewEtcFileStatus(resourcefiles.NamespaceName, constants.DefaultTrustedRelativeCAFile).Metadata(), ch); err != nil {
			return err
		}

		var specVersion, statusVersion string

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case e := <-ch:
				switch e.Type {
				case state.Errored:
					return e.Error
				case state.Bootstrapped, state.Destroyed, state.Noop: // ignore
				case state.Created, state.Updated:
					switch res := e.Resource.(type) {
					case *resourcefiles.EtcFileSpec:
						specVersion = res.Metadata().Version().String()
					case *resourcefiles.EtcFileStatus:
						statusVersion = res.TypedSpec().SpecVersion
					}
				}
			}

			if specVersion != "" && statusVersion != "" && specVersion == statusVersion {
				// success
				return nil
			}
		}
	}, "waitForCARoots"
}

// TeardownVolumeLifecycle tears down volume lifecycle resource.
func TeardownVolumeLifecycle(runtime.Sequence, any) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		volumeLifecycle := blockres.NewVolumeLifecycle(blockres.NamespaceName, blockres.VolumeLifecycleID).Metadata()

		_, err := r.State().V1Alpha2().Resources().Teardown(ctx, volumeLifecycle)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		_, err = r.State().V1Alpha2().Resources().WatchFor(ctx, volumeLifecycle, state.WithFinalizerEmpty())
		if err != nil {
			return err
		}

		return r.State().V1Alpha2().Resources().Destroy(ctx, volumeLifecycle)
	}, "teardownLifecycle"
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

func waitForVolumeReady(ctx context.Context, r runtime.Runtime, volumeID string) (*blockres.VolumeStatus, error) {
	return blockres.WaitForVolumePhase(ctx, r.State().V1Alpha2().Resources(), volumeID, blockres.VolumePhaseReady)
}
