// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.etcd.io/etcd/clientv3"
	"golang.org/x/sys/unix"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/talos-systems/go-retry/retry"

	installer "github.com/talos-systems/talos/cmd/installer/pkg/install"
	"github.com/talos-systems/talos/internal/app/machined/internal/install"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/internal/pkg/cri"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/kernel/kspp"
	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/table"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/cmd"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/kubernetes"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/sysctl"
	"github.com/talos-systems/talos/pkg/version"
)

// SetupLogger represents the SetupLogger task.
func SetupLogger(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if r.State().Platform().Mode() == runtime.ModeContainer {
			return nil
		}

		machinedLog, err := r.Logging().ServiceLog("machined").Writer()
		if err != nil {
			return err
		}

		if err = kmsg.SetupLogger(nil, "[talos]", machinedLog); err != nil {
			return fmt.Errorf("failed to setup logging: %w", err)
		}

		return nil
	}, "setupLogger"
}

// EnforceKSPPRequirements represents the EnforceKSPPRequirements task.
func EnforceKSPPRequirements(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = kspp.EnforceKSPPKernelParameters(); err != nil {
			return err
		}

		if err = kspp.EnforceKSPPSysctls(); err != nil {
			return err
		}

		return nil
	}, "enforceKSPPRequirements"
}

// SetupSystemDirectory represents the SetupSystemDirectory task.
func SetupSystemDirectory(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, p := range []string{constants.SystemEtcPath, constants.SystemRunPath, constants.SystemVarPath, constants.StateMountPoint} {
			if err = os.MkdirAll(p, 0o700); err != nil {
				return err
			}
		}

		return nil
	}, "setupSystemDirectory"
}

// MountBPFFS represents the MountBPFFS task.
func MountBPFFS(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.BPFMountPoints()
		if err != nil {
			return err
		}

		if err = mount.Mount(mountpoints); err != nil {
			return err
		}

		return nil
	}, "mountBPFFS"
}

const (
	memoryCgroup                  = "memory"
	memoryUseHierarchy            = "memory.use_hierarchy"
	memoryUseHierarchyPermissions = os.FileMode(400)
)

var memoryUseHierarchyContents = []byte(strconv.Itoa(1))

// MountCgroups represents the MountCgroups task.
func MountCgroups(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.CGroupMountPoints()
		if err != nil {
			return err
		}

		if err = mount.Mount(mountpoints); err != nil {
			return err
		}

		// See https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
		target := path.Join("/sys/fs/cgroup", memoryCgroup, memoryUseHierarchy)
		if err = ioutil.WriteFile(target, memoryUseHierarchyContents, memoryUseHierarchyPermissions); err != nil {
			return fmt.Errorf("failed to enable memory hierarchy support: %w", err)
		}

		return nil
	}, "mountCgroups"
}

// MountPseudoFilesystems represents the MountPseudoFilesystems task.
func MountPseudoFilesystems(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.PseudoSubMountPoints()
		if err != nil {
			return err
		}

		if err = mount.Mount(mountpoints); err != nil {
			return err
		}

		return nil
	}, "mountPseudoFilesystems"
}

// WriteRequiredSysctlsForContainer represents the WriteRequiredSysctlsForContainer task.
func WriteRequiredSysctlsForContainer(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var multiErr *multierror.Error

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.ipv4.ip_forward: %w", err))
		}

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv6.conf.default.forwarding", Value: "1"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.ipv6.conf.default.forwarding: %w", err))
		}

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "kernel.pid_max", Value: "262144"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set kernel.pid_max: %w", err))
		}

		return multiErr.ErrorOrNil()
	}, "writeRequiredSysctlsForContainer"
}

// WriteRequiredSysctls represents the WriteRequiredSysctls task.
func WriteRequiredSysctls(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var multiErr *multierror.Error

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.ipv4.ip_forward: %w", err))
		}

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.bridge.bridge-nf-call-iptables", Value: "1"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.bridge.bridge-nf-call-iptables: %w", err))
		}

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.bridge.bridge-nf-call-ip6tables", Value: "1"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.bridge.bridge-nf-call-ip6tables: %w", err))
		}

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv6.conf.default.forwarding", Value: "1"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.ipv6.conf.default.forwarding: %w", err))
		}

		if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "kernel.pid_max", Value: "262144"}); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set kernel.pid_max: %w", err))
		}

		return multiErr.ErrorOrNil()
	}, "writeRequiredSysctls"
}

// SetRLimit represents the SetRLimit task.
func SetRLimit(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// TODO(andrewrynhard): Should we read limit from /proc/sys/fs/nr_open?
		if err = unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 1048576, Max: 1048576}); err != nil {
			return err
		}

		return nil
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
	"measure func=MMAP_CHECK mask=MAY_EXEC",
	"measure func=BPRM_CHECK mask=MAY_EXEC",
	"measure func=FILE_CHECK mask=^MAY_READ euid=0",
	"measure func=FILE_CHECK mask=^MAY_READ uid=0",
	"measure func=MODULE_CHECK",
	"measure func=FIRMWARE_CHECK",
	"measure func=POLICY_CHECK",
}

// WriteIMAPolicy represents the WriteIMAPolicy task.
func WriteIMAPolicy(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if _, err = os.Stat("/sys/kernel/security/ima/policy"); os.IsNotExist(err) {
			return fmt.Errorf("policy file does not exist: %w", err)
		}

		f, err := os.OpenFile("/sys/kernel/security/ima/policy", os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}

		defer f.Close() //nolint: errcheck

		for _, line := range rules {
			if _, err = f.WriteString(line + "\n"); err != nil {
				return fmt.Errorf("rule %q is invalid", err)
			}
		}

		return nil
	}, "writeIMAPolicy"
}

const osReleaseTemplate = `
NAME="{{ .Name }}"
ID={{ .ID }}
VERSION_ID={{ .Version }}
PRETTY_NAME="{{ .Name }} ({{ .Version }})"
HOME_URL="https://docs.talos-systems.com/"
BUG_REPORT_URL="https://github.com/talos-systems/talos/issues"
`

// Hosts creates a persistent and writable /etc/hosts file.
func Hosts() (err error) {
	return createBindMount(filepath.Join(constants.SystemEtcPath, "hosts"), "/etc/hosts")
}

// ResolvConf creates a persistent and writable /etc/resolv.conf file.
func ResolvConf() (err error) {
	return createBindMount(filepath.Join(constants.SystemEtcPath, "resolv.conf"), "/etc/resolv.conf")
}

// OSRelease renders a valid /etc/os-release file and writes it to disk. The
// node's OS Image field is reported by the node from /etc/os-release.
func OSRelease() (err error) {
	if err = createBindMount(filepath.Join(constants.SystemEtcPath, "os-release"), "/etc/os-release"); err != nil {
		return err
	}

	var (
		v    string
		tmpl *template.Template
	)

	switch version.Tag {
	case "none":
		v = version.SHA
	default:
		v = version.Tag
	}

	data := struct {
		Name    string
		ID      string
		Version string
	}{
		Name:    version.Name,
		ID:      strings.ToLower(version.Name),
		Version: v,
	}

	tmpl, err = template.New("").Parse(osReleaseTemplate)
	if err != nil {
		return err
	}

	var buf []byte

	writer := bytes.NewBuffer(buf)

	err = tmpl.Execute(writer, data)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(constants.SystemEtcPath, "os-release"), writer.Bytes(), 0o644)
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

// CreateEtcNetworkFiles represents the CreateEtcNetworkFiles task.
func CreateEtcNetworkFiles(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// Create /etc/resolv.conf.
		if err = ResolvConf(); err != nil {
			return err
		}

		// Create /etc/hosts
		if err = Hosts(); err != nil {
			return err
		}

		return nil
	}, "createEtcNetworkFiles"
}

// CreateOSReleaseFile represents the CreateOSReleaseFile task.
func CreateOSReleaseFile(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// Create /etc/os-release.
		return OSRelease()
	}, "createOSReleaseFile"
}

// SetupDiscoveryNetwork represents the task for setting up the initial network.
func SetupDiscoveryNetwork(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		nwd, err := networkd.New(r.Config())
		if err != nil {
			return err
		}

		if err = nwd.Configure(); err != nil {
			return err
		}

		return nil
	}, "setupDiscoveryNetwork"
}

// LoadConfig represents the LoadConfig task.
func LoadConfig(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		download := func() error {
			var b []byte

			b, e := fetchConfig(r)
			if e != nil {
				return e
			}

			logger.Printf("storing config in memory")

			if err = r.SetConfig(b); err != nil {
				return err
			}

			return nil
		}

		cfg, err := configloader.NewFromFile(constants.ConfigPath)
		if err != nil {
			logger.Printf("downloading config")

			return download()
		}

		if !cfg.Persist() {
			logger.Printf("found existing config, but peristence is disabled, downloading config")

			return download()
		}

		logger.Printf("peristence is enabled, using existing config on disk")

		b, err := cfg.Bytes()
		if err != nil {
			return err
		}

		if err = r.SetConfig(b); err != nil {
			return err
		}

		return nil
	}, "loadConfig"
}

// SaveConfig represents the SaveConfig task.
func SaveConfig(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		hostname, err := r.State().Platform().Hostname()
		if err != nil {
			return err
		}

		if hostname != nil {
			r.Config().Machine().Network().SetHostname(string(hostname))
		}

		addrs, err := r.State().Platform().ExternalIPs()
		if err != nil {
			logger.Printf("certificates will be created without external IPs: %v", err)
		}

		sans := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			sans = append(sans, addr.String())
		}

		r.Config().Machine().Security().SetCertSANs(sans)
		r.Config().Cluster().SetCertSANs(sans)

		var b []byte

		b, err = r.Config().Bytes()
		if err != nil {
			return err
		}

		return ioutil.WriteFile(constants.ConfigPath, b, 0o600)
	}, "saveConfig"
}

func fetchConfig(r runtime.Runtime) (out []byte, err error) {
	var b []byte

	if b, err = r.State().Platform().Configuration(); err != nil {
		return nil, err
	}

	// Detect if config is a gzip archive and unzip it if so
	contentType := http.DetectContentType(b)
	if contentType == "application/x-gzip" {
		var gzipReader *gzip.Reader

		gzipReader, err = gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %w", err)
		}

		// nolint: errcheck
		defer gzipReader.Close()

		var unzippedData []byte

		unzippedData, err = ioutil.ReadAll(gzipReader)
		if err != nil {
			return nil, fmt.Errorf("error unzipping machine config: %w", err)
		}

		b = unzippedData
	}

	return b, nil
}

// ValidateConfig validates the config.
func ValidateConfig(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return r.Config().Validate(r.State().Platform().Mode())
	}, "validateConfig"
}

// ResetNetwork resets the network.
//
// nolint: gocyclo
func ResetNetwork(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		nwd, err := networkd.New(r.Config())
		if err != nil {
			return err
		}

		nwd.Reset()

		return nil
	}, "resetNetwork"
}

// SetUserEnvVars represents the SetUserEnvVars task.
func SetUserEnvVars(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for key, val := range r.Config().Machine().Env() {
			if err = os.Setenv(key, val); err != nil {
				return fmt.Errorf("failed to set enivronment variable: %w", err)
			}
		}

		return nil
	}, "setUserEnvVars"
}

// StartContainerd represents the task to start containerd.
func StartContainerd(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svc := &services.Containerd{}

		system.Services(r).LoadAndStart(svc)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx)
	}, "startContainerd"
}

// StartUdevd represents the task to start udevd.
func StartUdevd(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svc := &services.Udevd{}

		system.Services(r).LoadAndStart(svc)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		if err = system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx); err != nil {
			return err
		}

		return nil
	}, "startUdevd"
}

// StartAllServices represents the task to start the system services.
func StartAllServices(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svcs := system.Services(r)

		svcs.Load(
			&services.APID{},
			&services.Routerd{},
			&services.Networkd{},
			&services.CRI{},
			&services.Kubelet{},
		)

		if r.State().Platform().Mode() != runtime.ModeContainer && r.Config().Machine().Time().Enabled() {
			svcs.Load(
				&services.Timed{},
			)
		}

		switch r.Config().Machine().Type() {
		case machine.TypeInit:
			svcs.Load(
				&services.Trustd{},
				&services.Etcd{Bootstrap: true},
				&services.Bootkube{},
			)
		case machine.TypeControlPlane:
			svcs.Load(
				&services.Trustd{},
				&services.Etcd{},
			)
		case machine.TypeJoin:
		case machine.TypeUnknown:
			return fmt.Errorf("unexpected machine type: %s", r.Config().Machine().Type())
		}

		system.Services(r).StartAll()

		all := []conditions.Condition{}

		logger.Printf("waiting for %d services", len(svcs.List()))

		for _, svc := range svcs.List() {
			cond := system.WaitForService(system.StateEventUp, svc.AsProto().GetId())
			all = append(all, cond)
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)

		defer cancel()

		return conditions.WaitForAll(all...).Wait(ctx)
	}, "startAllServices"
}

// StopServicesForUpgrade represents the StopServicesForUpgrade task.
func StopServicesForUpgrade(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, service := range []string{"kubelet", "cri", "udevd"} {
			if err = system.Services(nil).Stop(ctx, service); err != nil {
				return err
			}
		}

		return nil
	}, "stopServicesForUpgrade"
}

// StopAllServices represents the StopAllServices task.
func StopAllServices(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		system.Services(nil).Shutdown()

		return nil
	}, "stopAllServices"
}

// VerifyInstallation represents the VerifyInstallation task.
func VerifyInstallation(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var (
			current string
			next    string
		)

		grub := &grub.Grub{
			BootDisk: r.Config().Machine().Install().Disk(),
		}

		current, next, err = grub.Labels()
		if err != nil {
			return err
		}

		if current == "" && next == "" {
			return fmt.Errorf("bootloader is not configured")
		}

		return err
	}, "verifyInstallation"
}

// MountOverlayFilesystems represents the MountOverlayFilesystems task.
func MountOverlayFilesystems(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.OverlayMountPoints()
		if err != nil {
			return err
		}

		if err = mount.Mount(mountpoints); err != nil {
			return err
		}

		return nil
	}, "mountOverlayFilesystems"
}

// SetupSharedFilesystems represents the SetupSharedFilesystems task.
func SetupSharedFilesystems(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		targets := []string{"/", "/var/lib/kubelet", "/etc/cni", "/run"}
		for _, t := range targets {
			if err = unix.Mount("", t, "", unix.MS_SHARED|unix.MS_REC, ""); err != nil {
				return err
			}
		}

		return nil
	}, "setupSharedFilesystems"
}

// SetupVarDirectory represents the SetupVarDirectory task.
func SetupVarDirectory(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, p := range []string{"/var/log/pods", "/var/lib/kubelet", "/var/run/lock"} {
			if err = os.MkdirAll(p, 0o700); err != nil {
				return err
			}
		}

		return nil
	}, "setupVarDirectory"
}

// MountUserDisks represents the MountUserDisks task.
func MountUserDisks(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = partitionAndFormatDisks(logger, r); err != nil {
			return err
		}

		return mountDisks(r)
	}, "mountUserDisks"
}

// TODO(andrewrynhard): We shouldn't pull in the installer command package
// here.
func partitionAndFormatDisks(logger *log.Logger, r runtime.Runtime) (err error) {
	m := &installer.Manifest{
		Targets: map[string][]*installer.Target{},
	}

	for _, disk := range r.Config().Machine().Disks() {
		var bd *blockdevice.BlockDevice

		bd, err = blockdevice.Open(disk.Device())
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer bd.Close()

		var pt table.PartitionTable

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
			if len(pt.Partitions()) > 0 {
				logger.Printf(("skipping setup of %q, found existing partitions"), disk.Device())
				continue
			}
		}

		if m.Targets[disk.Device()] == nil {
			m.Targets[disk.Device()] = []*installer.Target{}
		}

		for _, part := range disk.Partitions() {
			extraTarget := &installer.Target{
				Device: disk.Device(),
				Size:   part.Size(),
				Force:  true,
				Test:   false,
			}

			m.Targets[disk.Device()] = append(m.Targets[disk.Device()], extraTarget)
		}
	}

	if err = m.ExecuteManifest(); err != nil {
		return err
	}

	return nil
}

func mountDisks(r runtime.Runtime) (err error) {
	mountpoints := mount.NewMountPoints()

	for _, disk := range r.Config().Machine().Disks() {
		for i, part := range disk.Partitions() {
			var partname string

			partname, err = util.PartPath(disk.Device(), i+1)
			if err != nil {
				return err
			}

			if _, err = os.Stat(part.MountPoint()); errors.Is(err, os.ErrNotExist) {
				if err = os.MkdirAll(part.MountPoint(), 0o700); err != nil {
					return err
				}
			}

			mountpoints.Set(partname, mount.NewMountPoint(partname, part.MountPoint(), "xfs", unix.MS_NOATIME, ""))
		}
	}

	if err = mount.Mount(mountpoints); err != nil {
		return err
	}

	return nil
}

// WriteUserFiles represents the WriteUserFiles task.
//
// nolint: gocyclo
func WriteUserFiles(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var result *multierror.Error

		files, err := r.Config().Machine().Files()
		if err != nil {
			return fmt.Errorf("error generating extra files: %w", err)
		}

		extra, err := containerd.GenerateRegistriesConfig(r.Config().Machine().Registries())
		if err != nil {
			return err
		}

		files = append(files, extra...)

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

				existingFileContents, err = ioutil.ReadFile(f.Path())
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
				if err = ioutil.WriteFile(f.Path(), []byte(content), f.Permissions()); err != nil {
					result = multierror.Append(result, err)
					continue
				}

				if err = os.Chmod(f.Path(), f.Permissions()); err != nil {
					result = multierror.Append(result, err)
					continue
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

			if err = ioutil.WriteFile(p, []byte(content), f.Permissions()); err != nil {
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

// nolint: deadcode,unused
func doesNotExists(p string) (err error) {
	_, err = os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	return fmt.Errorf("file exists")
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

// WriteUserSysctls represents the WriteUserSysctls task.
func WriteUserSysctls(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var result *multierror.Error

		for k, v := range r.Config().Machine().Sysctls() {
			if err = sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: k, Value: v}); err != nil {
				return err
			}
		}

		return result.ErrorOrNil()
	}, "writeUserSysctls"
}

// UnmountOverlayFilesystems represents the UnmountOverlayFilesystems task.
func UnmountOverlayFilesystems(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var mountpoints *mount.Points

		mountpoints, err = mount.OverlayMountPoints()
		if err != nil {
			return err
		}

		if err = mount.Unmount(mountpoints); err != nil {
			return err
		}

		return nil
	}, "unmountOverlayFilesystems"
}

// UnmountPodMounts represents the UnmountPodMounts task.
func UnmountPodMounts(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var b []byte

		if b, err = ioutil.ReadFile("/proc/self/mounts"); err != nil {
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

				if err = unix.Unmount(mountpoint, 0); err != nil {
					return fmt.Errorf("error unmounting %s: %w", mountpoint, err)
				}
			}
		}

		if err = scanner.Err(); err != nil {
			return err
		}

		return nil
	}, "unmountPodMounts"
}

// UnmountSystemDiskBindMounts represents the UnmountSystemDiskBindMounts task.
func UnmountSystemDiskBindMounts(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		devname := r.State().Machine().Disk().BlockDevice.Device().Name()

		f, err := os.Open("/proc/mounts")
		if err != nil {
			return err
		}

		defer f.Close() //nolint: errcheck

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())

			if len(fields) < 2 {
				continue
			}

			device := fields[0]
			mountpoint := fields[1]

			if strings.HasPrefix(device, devname) {
				logger.Printf("unmounting %s\n", mountpoint)

				if err = unix.Unmount(mountpoint, 0); err != nil {
					return fmt.Errorf("error unmounting %s: %w", mountpoint, err)
				}
			}
		}

		return scanner.Err()
	}, "unmountSystemDiskBindMounts"
}

// CordonAndDrainNode represents the task for stop all containerd tasks in the
// k8s.io namespace.
//
//nolint: dupl
func CordonAndDrainNode(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var hostname string

		if hostname, err = os.Hostname(); err != nil {
			return err
		}

		var kubeHelper *kubernetes.Client

		if kubeHelper, err = kubernetes.NewClientFromKubeletKubeconfig(); err != nil {
			return err
		}

		if err = kubeHelper.CordonAndDrain(hostname); err != nil {
			return err
		}

		return nil
	}, "cordonAndDrainNode"
}

// UncordonNode represents the task for mark node as scheduling enabled.
//
// This action undoes the CordonAndDrainNode task.
//
//nolint: dupl
func UncordonNode(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var hostname string

		if hostname, err = os.Hostname(); err != nil {
			return err
		}

		var kubeHelper *kubernetes.Client

		if kubeHelper, err = kubernetes.NewClientFromKubeletKubeconfig(); err != nil {
			return err
		}

		if err = kubeHelper.WaitUntilReady(hostname); err != nil {
			return err
		}

		if err = kubeHelper.Uncordon(hostname, false); err != nil {
			return err
		}

		return nil
	}, "uncordonNode"
}

// LeaveEtcd represents the task for removing a control plane node from etcd.
// nolint: gocyclo
func LeaveEtcd(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}

		client, err := etcd.NewClientFromControlPlaneIPs(ctx, r.Config().Cluster().CA(), r.Config().Cluster().Endpoint())
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer client.Close()

		resp, err := client.MemberList(ctx)
		if err != nil {
			return err
		}

		var id *uint64

		for _, member := range resp.Members {
			if member.Name == hostname {
				member := member
				id = &member.ID

				break
			}
		}

		if id == nil {
			return fmt.Errorf("failed to find %q in list of etcd members", hostname)
		}

		logger.Println("leaving etcd cluster")

		_, err = client.MemberRemove(ctx, *id)
		if err != nil {
			return err
		}

		if err = system.Services(nil).Stop(ctx, "etcd"); err != nil {
			return err
		}

		// Once the member is removed, the data is no longer valid.
		if err = os.RemoveAll(constants.EtcdDataPath); err != nil {
			return err
		}

		return nil
	}, "leaveEtcd"
}

// RemoveAllPods represents the task for stopping all pods.
func RemoveAllPods(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = system.Services(nil).Stop(context.Background(), "kubelet"); err != nil {
			return err
		}

		client, err := cri.NewClient("unix://"+constants.ContainerdAddress, 10*time.Second)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer client.Close()

		// We remove pods with POD network mode first so that the CNI can perform
		// any cleanup tasks. If we don't do this, we run the risk of killing the
		// CNI, preventing the CRI from cleaning up the pod's netwokring.

		if err = client.RemovePodSandboxes(runtimeapi.NamespaceMode_POD, runtimeapi.NamespaceMode_CONTAINER); err != nil {
			return err
		}

		// With the POD network mode pods out of the way, we kill the remaining
		// pods.

		if err = client.RemovePodSandboxes(); err != nil {
			return err
		}

		return nil
	}, "removeAllPods"
}

// ResetSystemDisk represents the task to reset the system disk.
func ResetSystemDisk(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return r.State().Machine().Disk().BlockDevice.Reset()
	}, "resetSystemDisk"
}

// VerifyDiskAvailability represents the task for verifying that the system
// disk is not in use.
func VerifyDiskAvailability(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		//  We only need to verify system disk availability if we are going to
		// reformat the ephemeral partition.
		if !r.Config().Machine().Install().Force() {
			return nil
		}

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

					return retry.ExpectedError(fmt.Errorf("ephemeral partition in use: %q", partname))
				}

				return retry.UnexpectedError(fmt.Errorf("failed to verify ephemeral partition not in use: %w", err))
			}

			return nil
		})
	}, "verifyDiskAvailability"
}

func tryLock(path string) error {
	fd, errno := unix.Open(path, unix.O_RDONLY|unix.O_EXCL|unix.O_CLOEXEC, 0)

	// nolint: errcheck
	defer unix.Close(fd)

	return errno
}

func dumpMounts(logger *log.Logger) {
	mounts, err := os.Open("/proc/mounts")
	if err != nil {
		logger.Printf("failed to read mounts: %s", err)
		return
	}

	defer mounts.Close() //nolint: errcheck

	logger.Printf("contents of /proc/mounts:")

	_, _ = io.Copy(log.Writer(), mounts) //nolint: errcheck
}

// Upgrade represents the task for performing an upgrade.
func Upgrade(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
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
			r.Config().Machine().Registries(),
			install.WithPull(false),
			install.WithUpgrade(true),
			install.WithForce(!in.GetPreserve()),
			install.WithExtraKernelArgs(r.Config().Machine().Install().ExtraKernelArgs()),
		)
		if err != nil {
			return err
		}

		logger.Println("upgrade successful")

		return nil
	}, "upgrade"
}

// LabelNodeAsMaster represents the LabelNodeAsMaster task.
func LabelNodeAsMaster(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		h, err := kubernetes.NewTemporaryClientFromPKI(r.Config().Cluster().CA(), r.Config().Cluster().Endpoint())
		if err != nil {
			return err
		}

		hostname, err := os.Hostname()
		if err != nil {
			return err
		}

		err = retry.Constant(10*time.Minute, retry.WithUnits(3*time.Second)).Retry(func() error {
			if err = h.LabelNodeAsMaster(hostname); err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to label node as master: %w", err)
		}

		return nil
	}, "labelNodeAsMaster"
}

// UpdateBootloader represents the UpdateBootloader task.
func UpdateBootloader(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		meta, err := bootloader.NewMeta()
		if err != nil {
			return err
		}
		// nolint: errcheck
		defer meta.Close()

		if ok := meta.DeleteTag(bootloader.AdvUpgrade); ok {
			logger.Println("removing fallback")

			if _, err = meta.Write(); err != nil {
				return err
			}
		}

		return nil
	}, "updateBootloader"
}

// Reboot represents the Reboot task.
func Reboot(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		SyncNonVolatileStorageBuffers()

		return unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	}, "reboot"
}

// Shutdown represents the Shutdown task.
func Shutdown(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		SyncNonVolatileStorageBuffers()

		return unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	}, "shutdown"
}

// SyncNonVolatileStorageBuffers invokes unix.Sync and waits up to 30 seconds
// for it to finish.
//
// See http://man7.org/linux/man-pages/man2/reboot.2.html.
func SyncNonVolatileStorageBuffers() {
	syncdone := make(chan struct{})

	go func() {
		defer close(syncdone)

		unix.Sync()
	}()

	log.Printf("waiting for sync...")

	for i := 29; i >= 0; i-- {
		select {
		case <-syncdone:
			log.Printf("sync done")
			return
		case <-time.After(time.Second):
		}

		if i != 0 {
			log.Printf("waiting %d more seconds for sync to finish", i)
		}
	}

	log.Printf("sync hasn't completed in time, aborting...")
}

// MountBootPartition mounts the boot partition.
func MountBootPartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return mountSystemPartition(constants.BootPartitionLabel)
	}, "mountBootPartition"
}

// UnmountBootPartition unmounts the boot partition.
func UnmountBootPartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return unmountSystemPartition(constants.BootPartitionLabel)
	}, "unmountBootPartition"
}

// MountEFIPartition mounts the EFI partition.
func MountEFIPartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return mountSystemPartition(constants.EFIPartitionLabel)
	}, "mountEFIPartition"
}

// UnmountEFIPartition unmounts the EFI partition.
func UnmountEFIPartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return unmountSystemPartition(constants.EFIPartitionLabel)
	}, "unmountEFIPartition"
}

// MountStatePartition mounts the system partition.
func MountStatePartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return mountSystemPartition(constants.StatePartitionLabel, mount.WithSkipIfMounted(true))
	}, "mountStatePartition"
}

// UnmountStatePartition unmounts the system partition.
func UnmountStatePartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return unmountSystemPartition(constants.StatePartitionLabel)
	}, "unmountStatePartition"
}

// MountEphermeralPartition mounts the ephemeral partition.
func MountEphermeralPartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return mountSystemPartition(constants.EphemeralPartitionLabel)
	}, "mountEphermeralPartition"
}

// UnmountEphemeralPartition unmounts the ephemeral partition.
func UnmountEphemeralPartition(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return unmountSystemPartition(constants.EphemeralPartitionLabel)
	}, "unmountEphemeralPartition"
}

func mountSystemPartition(label string, opts ...mount.Option) (err error) {
	mountpoints := mount.NewMountPoints()

	mountpoint, err := mount.SystemMountPointForLabel(label, opts...)
	if err != nil {
		return err
	}

	if mountpoint == nil {
		return fmt.Errorf("no mountpoints for label %q", label)
	}

	mountpoints.Set(label, mountpoint)

	if err = mount.Mount(mountpoints); err != nil {
		return err
	}

	return nil
}

func unmountSystemPartition(label string) (err error) {
	mountpoints := mount.NewMountPoints()

	mountpoint, err := mount.SystemMountPointForLabel(label)
	if err != nil {
		return err
	}

	if mountpoint == nil {
		return fmt.Errorf("no mountpoints for label %q", label)
	}

	mountpoints.Set(label, mountpoint)

	if err = mount.Unmount(mountpoints); err != nil {
		return err
	}

	return nil
}

// Install mounts or installs the system partitions.
func Install(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		err = install.RunInstallerContainer(
			r.Config().Machine().Install().Disk(),
			r.State().Platform().Name(),
			r.Config().Machine().Install().Image(),
			r.Config().Machine().Registries(),
			install.WithForce(r.Config().Machine().Install().Force()),
			install.WithZero(r.Config().Machine().Install().Zero()),
			install.WithExtraKernelArgs(r.Config().Machine().Install().ExtraKernelArgs()),
		)
		if err != nil {
			return err
		}

		logger.Println("install successful")

		return nil
	}, "install"
}

// Recover attempts to recover the control plane.
//
// nolint: gocyclo
func Recover(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var (
			in *machineapi.RecoverRequest
			ok bool
		)

		if in, ok = data.(*machineapi.RecoverRequest); !ok {
			return runtime.ErrInvalidSequenceData
		}

		var b bytes.Buffer

		if err = kubeconfig.GenerateAdmin(r.Config().Cluster(), &b); err != nil {
			return err
		}

		if err = ioutil.WriteFile(constants.RecoveryKubeconfig, b.Bytes(), 0o600); err != nil {
			return fmt.Errorf("failed to create recovery kubeconfig: %w", err)
		}

		// nolint: errcheck
		defer os.Remove(constants.RecoveryKubeconfig)

		svc := &services.Bootkube{
			Recover: true,
			Source:  in.Source,
		}

		// unload bootkube (if any instance ran before)
		if err = system.Services(r).Unload(ctx, svc.ID(r)); err != nil {
			return err
		}

		system.Services(r).Load(svc)

		if err = system.Services(r).Start(svc.ID(r)); err != nil {
			return fmt.Errorf("failed to start bootkube: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventFinished, svc.ID(r)).Wait(ctx)
	}, "recover"
}

// BootstrapEtcd represents the task for bootstrapping etcd.
func BootstrapEtcd(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = system.Services(r).Stop(ctx, "etcd"); err != nil {
			return fmt.Errorf("failed to stop etcd: %w", err)
		}

		// This is hack. We need to fake a finished state so that we can get the
		// wait in the boot sequence to unblock.
		for _, svc := range system.Services(r).List() {
			if svc.AsProto().GetId() == "etcd" {
				svc.UpdateState(events.StateFinished, "Bootstrap requested")

				break
			}
		}

		// Since etcd has already attempted to start, we must delete the data. If
		// we don't, then an initial cluster state of "new" will fail.
		dir, err := os.Open(constants.EtcdDataPath)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer dir.Close()

		files, err := dir.Readdir(0)
		if err != nil {
			return err
		}

		for _, file := range files {
			fullPath := filepath.Join(constants.EtcdDataPath, file.Name())

			if err = os.RemoveAll(fullPath); err != nil {
				return fmt.Errorf("failed to remove %q: %w", file.Name(), err)
			}
		}

		svc := &services.Etcd{Bootstrap: true}

		if err = system.Services(r).Unload(ctx, svc.ID(r)); err != nil {
			return err
		}

		system.Services(r).Load(svc)

		if err = system.Services(r).Start(svc.ID(r)); err != nil {
			return fmt.Errorf("error starting etcd in bootstrap mode: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx)
	}, "bootstrapEtcd"
}

// BootstrapKubernetes represents the task for bootstrapping Kubernetes.
func BootstrapKubernetes(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svc := &services.Bootkube{}

		system.Services(r).LoadAndStart(svc)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		return system.WaitForService(system.StateEventFinished, svc.ID(r)).Wait(ctx)
	}, "bootstrapKubernetes"
}

// SetInitStatus represents the task for setting the initialization status
// in etcd.
func SetInitStatus(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer client.Close()

		err = retry.Exponential(15*time.Second, retry.WithUnits(50*time.Millisecond), retry.WithJitter(25*time.Millisecond)).Retry(func() error {
			ctx := clientv3.WithRequireLeader(context.Background())
			if _, err = client.Put(ctx, constants.InitializedKey, "true"); err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to put state into etcd: %w", err)
		}

		logger.Println("updated initialization status in etcd")

		return nil
	}, "SetInitStatus"
}

// ActivateLogicalVolumes represents the task for activating logical volumes.
func ActivateLogicalVolumes(seq runtime.Sequence, data interface{}) (runtime.TaskExecutionFunc, string) {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if _, err = cmd.Run("/sbin/lvm", "vgchange", "-ay"); err != nil {
			return fmt.Errorf("failed to activate logical volumes: %w", err)
		}

		return nil
	}, "activateLogicalVolumes"
}
