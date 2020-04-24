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
	"golang.org/x/sys/unix"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/cmd/installer/pkg/manifest"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/cri"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/kernel/kspp"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/retry"
	"github.com/talos-systems/talos/pkg/sysctl"
	"github.com/talos-systems/talos/pkg/universe"
	"github.com/talos-systems/talos/pkg/version"
)

// EnforceKSPPRequirements represents the EnforceKSPPRequirements task.
func EnforceKSPPRequirements(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = kspp.EnforceKSPPKernelParameters(); err != nil {
			return err
		}

		if err = kspp.EnforceKSPPSysctls(); err != nil {
			return err
		}

		return nil
	}
}

// SetupSystemDirectory represents the SetupSystemDirectory task.
func SetupSystemDirectory(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, p := range []string{"etc", "log"} {
			if err = os.MkdirAll(filepath.Join(universe.SystemRunPath, p), 0700); err != nil {
				return err
			}
		}

		return nil
	}
}

// MountBPFFS represents the MountBPFFS task.
func MountBPFFS(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

const (
	memoryCgroup                  = "memory"
	memoryUseHierarchy            = "memory.use_hierarchy"
	memoryUseHierarchyPermissions = os.FileMode(400)
)

var memoryUseHierarchyContents = []byte(strconv.Itoa(1))

// MountCgroups represents the MountCgroups task.
func MountCgroups(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// MountPseudoFilesystems represents the MountPseudoFilesystems task.
func MountPseudoFilesystems(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// WriteRequiredSysctlsForContainer represents the WriteRequiredSysctlsForContainer task.
func WriteRequiredSysctlsForContainer(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// WriteRequiredSysctls represents the WriteRequiredSysctls task.
func WriteRequiredSysctls(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// SetRLimit represents the SetRLimit task.
func SetRLimit(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// TODO(andrewrynhard): Should we read limit from /proc/sys/fs/nr_open?
		if err = unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 1048576, Max: 1048576}); err != nil {
			return err
		}

		return nil
	}
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
func WriteIMAPolicy(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if _, err = os.Stat("/sys/kernel/security/ima/policy"); os.IsNotExist(err) {
			return fmt.Errorf("policy file does not exist: %w", err)
		}

		f, err := os.OpenFile("/sys/kernel/security/ima/policy", os.O_APPEND|os.O_WRONLY, 0644)
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
	}
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
	return createBindMount("/run/system/etc/hosts", "/etc/hosts")
}

// ResolvConf creates a persistent and writable /etc/resolv.conf file.
func ResolvConf() (err error) {
	return createBindMount("/run/system/etc/resolv.conf", "/etc/resolv.conf")
}

// OSRelease renders a valid /etc/os-release file and writes it to disk. The
// node's OS Image field is reported by the node from /etc/os-release.
func OSRelease() (err error) {
	if err = createBindMount("/run/system/etc/os-release", "/etc/os-release"); err != nil {
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

	return ioutil.WriteFile("/run/system/etc/os-release", writer.Bytes(), 0644)
}

// createBindMount creates a common way to create a writable source file with a
// bind mounted destination. This is most commonly used for well known files
// under /etc that need to be adjusted during startup.
func createBindMount(src, dst string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(src, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		return err
	}

	// nolint: errcheck
	if err = f.Close(); err != nil {
		return err
	}

	if err = unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for %s: %w", dst, err)
	}

	return nil
}

// CreateEtcNetworkFiles represents the CreateEtcNetworkFiles task.
func CreateEtcNetworkFiles(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// CreateOSReleaseFile represents the CreateOSReleaseFile task.
func CreateOSReleaseFile(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		// Create /etc/os-release.
		return OSRelease()
	}
}

// SetupDiscoveryNetwork represents the task for setting up the initial network.
func SetupDiscoveryNetwork(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		nwd, err := networkd.New(r.Config())
		if err != nil {
			return err
		}

		if err = nwd.Configure(); err != nil {
			return err
		}

		return nil
	}
}

// LoadConfig represents the LoadConfig task.
func LoadConfig(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		download := func() error {
			var b []byte

			b, e := fetchConfig(r)
			if e != nil {
				return e
			}

			logger.Printf("storing config in memory")

			r.State().SetConfig(b)

			return nil
		}

		cfg, err := config.NewFromFile(universe.ConfigPath)
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

		r.State().SetConfig(b)

		return nil
	}
}

// SaveConfig represents the SaveConfig task.
func SaveConfig(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return ioutil.WriteFile(universe.ConfigPath, r.State().Config(), 0600)
	}
}

func fetchConfig(r runtime.Runtime) (out []byte, err error) {
	var b []byte

	if b, err = r.Platform().Configuration(); err != nil {
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
func ValidateConfig(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return r.Config().Validate(r.Platform().Mode())
	}
}

// ValidateConfigFromState validates the config.
func ValidateConfigFromState(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		cfg, err := config.NewFromBytes(r.State().Config())
		if err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}

		return cfg.Validate(r.Platform().Mode())
	}
}

// ResetNetwork resets the network.
//
// nolint: gocyclo
func ResetNetwork(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		nwd, err := networkd.New(r.Config())
		if err != nil {
			return err
		}

		nwd.Reset()

		return nil
	}
}

// SetUserEnvVars represents the SetUserEnvVars task.
func SetUserEnvVars(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for key, val := range r.Config().Machine().Env() {
			if err = os.Setenv(key, val); err != nil {
				return fmt.Errorf("failed to set enivronment variable: %w", err)
			}
		}

		return nil
	}
}

// StartStage1SystemServices represents the task to start the system services.
func StartStage1SystemServices(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svcs := system.Services(r.Config())

		svcs.Load(
			&services.Containerd{},
		)

		system.Services(r.Config()).StartAll()

		all := []conditions.Condition{}

		for _, svc := range svcs.List() {
			cond := system.WaitForService(system.StateEventUp, svc.AsProto().GetId())
			all = append(all, cond)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		defer cancel()

		return conditions.WaitForAll(all...).Wait(ctx)
	}
}

// StartStage2SystemServicesForContainer represents the task to start the system services.
func StartStage2SystemServicesForContainer(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svcs := system.Services(r.Config())

		svcs.Load(
			&services.APID{},
			&services.Routerd{},
			&services.Networkd{},
			&services.OSD{},
		)

		switch r.Config().Machine().Type() {
		case runtime.MachineTypeInit, runtime.MachineTypeControlPlane:
			svcs.Load(
				&services.Trustd{},
			)
		}

		system.Services(r.Config()).StartAll()

		all := []conditions.Condition{}

		for _, svc := range svcs.List() {
			cond := system.WaitForService(system.StateEventUp, svc.AsProto().GetId())
			all = append(all, cond)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		defer cancel()

		return conditions.WaitForAll(all...).Wait(ctx)
	}
}

// StartStage2SystemServices represents the task to start the system services.
func StartStage2SystemServices(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svcs := system.Services(r.Config())

		svcs.Load(
			&services.APID{},
			&services.Routerd{},
			&services.Networkd{},
			&services.OSD{},
			&services.Timed{},
			&services.Udevd{},
			&services.UdevdTrigger{},
		)

		switch r.Config().Machine().Type() {
		case runtime.MachineTypeInit, runtime.MachineTypeControlPlane:
			svcs.Load(
				&services.Trustd{},
			)
		}

		system.Services(r.Config()).StartAll()

		all := []conditions.Condition{}

		for _, svc := range svcs.List() {
			cond := system.WaitForService(system.StateEventUp, svc.AsProto().GetId())
			all = append(all, cond)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		defer cancel()

		return conditions.WaitForAll(all...).Wait(ctx)
	}
}

// PullPlatformMetadata represents the PullPlatformMetadata task.
func PullPlatformMetadata(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		hostname, err := r.Platform().Hostname()
		if err != nil {
			return err
		}

		if hostname != nil {
			r.Config().Machine().Network().SetHostname(string(hostname))
		}

		addrs, err := r.Platform().ExternalIPs()
		if err != nil {
			logger.Printf("certificates will be created without external IPs: %v\n", err)
		}

		sans := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			sans = append(sans, addr.String())
		}

		r.Config().Machine().Security().SetCertSANs(sans)
		r.Config().Cluster().SetCertSANs(sans)

		return nil
	}
}

// VerifyInstallation represents the VerifyInstallation task.
func VerifyInstallation(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var (
			current string
			next    string
		)

		current, next, err = syslinux.Labels()
		if err != nil {
			return err
		}

		if current == "" && next == "" {
			return fmt.Errorf("syslinux.cfg is not configured")
		}

		return err
	}
}

// MountOverlayFilesystems represents the MountOverlayFilesystems task.
func MountOverlayFilesystems(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// MountVolumesAsSharedForContainer represents the MountVolumesAsSharedForContainer task.
func MountVolumesAsSharedForContainer(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		targets := []string{"/", "/var/lib/kubelet", "/etc/cni", "/run"}
		for _, t := range targets {
			if err = unix.Mount("", t, "", unix.MS_SHARED|unix.MS_REC, ""); err != nil {
				return err
			}
		}

		return nil
	}
}

// SetupVarDirectory represents the SetupVarDirectory task.
func SetupVarDirectory(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		for _, p := range []string{"/var/log/pods", "/var/lib/kubelet", "/var/run/lock"} {
			if err = os.MkdirAll(p, 0700); err != nil {
				return err
			}
		}

		return nil
	}
}

// MountUserDisks represents the MountUserDisks task.
func MountUserDisks(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = partitionAndFormatDisks(logger, r); err != nil {
			return err
		}

		return mountDisks(r)
	}
}

func partitionAndFormatDisks(logger *log.Logger, r runtime.Runtime) (err error) {
	m := &manifest.Manifest{
		Targets: map[string][]*manifest.Target{},
	}

	for _, disk := range r.Config().Machine().Disks() {
		if m.Targets[disk.Device] == nil {
			m.Targets[disk.Device] = []*manifest.Target{}
		}

		for _, part := range disk.Partitions {
			extraTarget := &manifest.Target{
				Device: disk.Device,
				Size:   part.Size,
				Force:  true,
				Test:   false,
			}

			m.Targets[disk.Device] = append(m.Targets[disk.Device], extraTarget)
		}
	}

	probed, err := probe.All()
	if err != nil {
		return err
	}

	// TODO(andrewrynhard): This is disgusting, but it works. We should revisit
	// this at a later time.
	for _, p := range probed {
		for _, disk := range r.Config().Machine().Disks() {
			for i := range disk.Partitions {
				partname := util.PartPath(disk.Device, i+1)
				if p.Path == partname {
					logger.Printf(("found existing partitions for %q"), disk.Device)
					return nil
				}
			}
		}
	}

	if err = m.ExecuteManifest(); err != nil {
		return err
	}

	return nil
}

func mountDisks(r runtime.Runtime) (err error) {
	mountpoints := mount.NewMountPoints()

	for _, extra := range r.Config().Machine().Disks() {
		for i, part := range extra.Partitions {
			partname := util.PartPath(extra.Device, i+1)
			mountpoints.Set(partname, mount.NewMountPoint(partname, part.MountPoint, "xfs", unix.MS_NOATIME, ""))
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
func WriteUserFiles(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var result *multierror.Error

		files, err := r.Config().Machine().Files()
		if err != nil {
			return fmt.Errorf("error generating extra files: %w", err)
		}

		for _, f := range files {
			content := f.Content

			switch f.Op {
			case "create":
				if err = doesNotExists(f.Path); err != nil {
					result = multierror.Append(result, fmt.Errorf("file must not exist: %q", f.Path))
					continue
				}
			case "overwrite":
				if err = existsAndIsFile(f.Path); err != nil {
					result = multierror.Append(result, err)
					continue
				}
			case "append":
				if err = existsAndIsFile(f.Path); err != nil {
					result = multierror.Append(result, err)
					continue
				}

				var existingFileContents []byte

				existingFileContents, err = ioutil.ReadFile(f.Path)
				if err != nil {
					result = multierror.Append(result, err)
					continue
				}

				content = string(existingFileContents) + "\n" + f.Content
			default:
				result = multierror.Append(result, fmt.Errorf("unknown operation for file %q: %q", f.Path, f.Op))
				continue
			}

			// Determine if supplied path is in /var or not.
			// If not, we'll write it to /var anyways and bind mount below
			p := f.Path
			inVar := true
			explodedPath := strings.Split(
				strings.TrimLeft(f.Path, "/"),
				string(os.PathSeparator),
			)

			if explodedPath[0] != "var" {
				p = filepath.Join("/var", f.Path)
				inVar = false
			}

			// We do not want to support creating new files anywhere outside of
			// /var. If a valid use case comes up, we can reconsider then.
			if !inVar && f.Op == "create" {
				return fmt.Errorf("create operation not allowed outside of /var: %q", f.Path)
			}

			if err = os.MkdirAll(filepath.Dir(p), os.ModeDir); err != nil {
				result = multierror.Append(result, err)
				continue
			}

			if err = ioutil.WriteFile(p, []byte(content), f.Permissions); err != nil {
				result = multierror.Append(result, err)
				continue
			}

			// File path was not /var/... so we assume a bind mount is wanted
			if !inVar {
				if err = unix.Mount(p, f.Path, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
					result = multierror.Append(result, fmt.Errorf("failed to create bind mount for %s: %w", p, err))
				}
			}
		}

		return result.ErrorOrNil()
	}
}

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
func WriteUserSysctls(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		var result *multierror.Error

		for k, v := range r.Config().Machine().Sysctls() {
			if err = sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: k, Value: v}); err != nil {
				return err
			}
		}

		return result.ErrorOrNil()
	}
}

// StartOrchestrationServices represents the task to start the system services.
func StartOrchestrationServices(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		svcs := system.Services(r.Config())

		svcs.Load(
			&services.CRI{},
			&services.Kubelet{},
		)

		switch r.Config().Machine().Type() {
		case runtime.MachineTypeInit:
			svcs.Load(
				&services.Etcd{},
				&services.Bootkube{},
			)
		case runtime.MachineTypeControlPlane:
			svcs.Load(
				&services.Etcd{},
			)
		}

		system.Services(r.Config()).StartAll()

		all := []conditions.Condition{}

		for _, svc := range svcs.List() {
			cond := system.WaitForService(system.StateEventUp, svc.AsProto().GetId())
			all = append(all, cond)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)

		defer cancel()

		return conditions.WaitForAll(all...).Wait(ctx)
	}
}

// StopServices represents the StopServices task.
func StopServices(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		system.Services(nil).Shutdown()

		return nil
	}
}

// UnmountOverlayFilesystems represents the UnmountOverlayFilesystems task.
func UnmountOverlayFilesystems(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// UnmountPodMounts represents the UnmountPodMounts task.
func UnmountPodMounts(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
			if strings.HasPrefix(mountpoint, universe.EphemeralMountPoint+"/") {
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
	}
}

// UnmountSystemDiskBindMounts represents the UnmountSystemDiskBindMounts task.
func UnmountSystemDiskBindMounts(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		devname := r.State().Disk().BlockDevice.Device().Name()

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
	}
}

// CordonAndDrainNode represents the task for stop all containerd tasks in the
// k8s.io namespace.
func CordonAndDrainNode(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
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
	}
}

// LeaveEtcd represents the task for removing a control plane node from etcd.
// nolint: gocyclo
func LeaveEtcd(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	var preserve bool

	switch in := data.(type) {
	case *machine.UpgradeRequest:
		preserve = in.GetPreserve()
	case *machine.ResetRequest:
		preserve = in.GetGraceful()
	default:
		return nil
	}

	if preserve {
		return nil
	}

	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if r.Config().Machine().Type() == runtime.MachineTypeJoin {
			return nil
		}

		hostname, err := os.Hostname()
		if err != nil {
			return err
		}

		client, err := etcd.NewClientFromControlPlaneIPs(r.Config().Cluster().CA(), r.Config().Cluster().Endpoint())
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer client.Close()

		resp, err := client.MemberList(context.Background())
		if err != nil {
			return err
		}

		var id *uint64

		for _, member := range resp.Members {
			if member.Name == hostname {
				id = &member.ID
			}
		}

		if id == nil {
			return fmt.Errorf("failed to find %q in list of etcd members", hostname)
		}

		logger.Println("leaving etcd cluster")

		_, err = client.MemberRemove(context.Background(), *id)
		if err != nil {
			return err
		}

		if err = system.Services(nil).Stop(context.Background(), "etcd"); err != nil {
			return err
		}

		// Once the member is removed, the data is no longer valid.
		if err = os.RemoveAll(universe.EtcdDataPath); err != nil {
			return err
		}

		return nil
	}
}

// RemoveAllPods represents the task for stopping all pods.
func RemoveAllPods(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if err = system.Services(nil).Stop(context.Background(), "kubelet"); err != nil {
			return err
		}

		client, err := cri.NewClient("unix://"+universe.ContainerdAddress, 10*time.Second)
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
	}
}

// ResetSystemDisk represents the task to reset the system disk.
func ResetSystemDisk(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return r.State().Disk().BlockDevice.Reset()
	}
}

// VerifyDiskAvailability represents the task for verifying that the system
// disk is not in use.
func VerifyDiskAvailability(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		//  We only need to verify system disk availability if we are going to
		// reformat the ephemeral partition.
		if r.Config().Machine().Install().Force() {
			return nil
		}

		devname := r.State().Disk().BlockDevice.Device().Name()

		// TODO(andrewrynhard): This should be more dynamic. If we ever change the
		// partition scheme there is the chance that 2 is not the correct parition to
		// check.
		partname := util.PartPath(devname, 2)

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

					return retry.ExpectedError(errors.New("ephemeral partition in use"))
				}

				return retry.UnexpectedError(fmt.Errorf("failed to verify ephemeral partition not in use: %w", err))
			}

			return nil
		})
	}
}

func tryLock(devname string) error {
	fd, errno := unix.Open(devname, unix.O_RDONLY|unix.O_EXCL|unix.O_CLOEXEC, 0)

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

// Upgrade represents the task for stop all containerd tasks in the
// k8s.io namespace.
func Upgrade(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		in, ok := data.(*machine.UpgradeRequest)
		if !ok {
			return runtime.ErrInvalidSequenceData
		}

		devname := r.State().Disk().BlockDevice.Device().Name()

		logger.Printf("performing upgrade via %q", in.GetImage())

		c := r.Config()
		if cfg, ok := c.(*v1alpha1.Config); ok {
			cfg.MachineConfig.MachineInstall.InstallDisk = devname
			cfg.MachineConfig.MachineInstall.InstallImage = in.GetImage()

			r = NewRuntime(r.Platform(), runtime.Configurator(cfg), r.State())
		}

		// We pull the installer image when we receive an upgrade request. No need to re-pull inside of installer container
		// TODO: Should we pass in the raw API request here?
		if err = install.RunInstallerContainer(r, seq, install.WithImagePull(false), install.WithPreserve(in.GetPreserve())); err != nil {
			return err
		}

		return nil
	}
}

// LabelNodeAsMaster represents the LabelNodeAsMaster task.
func LabelNodeAsMaster(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if r.Config().Machine().Type() == runtime.MachineTypeJoin {
			return nil
		}

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
	}
}

// UpdateBootloader represents the UpdateBootloader task.
func UpdateBootloader(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		f, err := os.OpenFile(syslinux.SyslinuxLdlinux, os.O_RDWR, 0700)
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer f.Close()

		adv, err := syslinux.NewADV(f)
		if err != nil {
			return err
		}

		if ok := adv.DeleteTag(syslinux.AdvUpgrade); ok {
			logger.Println("removing fallback")

			if _, err = f.Write(adv); err != nil {
				return err
			}
		}

		return nil
	}
}

// Reboot represents the Reboot task.
func Reboot(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		SyncNonVolatileStorageBuffers()

		return unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	}
}

// Shutdown represents the Shutdown task.
func Shutdown(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		SyncNonVolatileStorageBuffers()

		return unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	}
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
func MountBootPartition(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return mountSystemPartition(universe.BootPartitionLabel)
	}
}

// UnmountBootPartition unmounts the boot partition.
func UnmountBootPartition(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return unmountSystemPartition(universe.BootPartitionLabel)
	}
}

// MountEphermeralPartition mounts the ephemeral partition.
func MountEphermeralPartition(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) error {
		return mountSystemPartition(universe.EphemeralPartitionLabel)
	}
}

// UnmountEphemeralPartition unmounts the ephemeral partition.
func UnmountEphemeralPartition(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		return unmountSystemPartition(universe.EphemeralPartitionLabel)
	}
}

func mountSystemPartition(label string) (err error) {
	mountpoints := mount.NewMountPoints()

	mountpoint, err := mount.SystemMountPointForLabel(label)
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
func Install(seq runtime.Sequence, data interface{}) runtime.TaskExecutionFunc {
	return func(ctx context.Context, logger *log.Logger, r runtime.Runtime) (err error) {
		if r.Config().Machine().Install().Image() == "" {
			return errors.New("an install image is required")
		}

		if err = install.RunInstallerContainer(r, seq); err != nil {
			return err
		}

		return nil
	}
}
