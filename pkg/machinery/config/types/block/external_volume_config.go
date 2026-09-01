// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"path"
	"strconv"
	"strings"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// ExternalVolumeConfigKind is a config document kind.
const ExternalVolumeConfigKind = "ExternalVolumeConfig"

func init() {
	registry.Register(ExternalVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ExternalVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ExternalVolumeConfig = &ExternalVolumeConfigV1Alpha1{}
	_ config.NamedDocument        = &ExternalVolumeConfigV1Alpha1{}
	_ config.Validator            = &ExternalVolumeConfigV1Alpha1{}
)

const maxExternalVolumeNameLength = constants.PartitionLabelLength - len(constants.ExternalVolumePrefix)

// FilesystemType is an alias for block.FilesystemType.
type FilesystemType = block.FilesystemType

// NFSVersion is an alias for block.NFSVersion.
type NFSVersion = block.NFSVersion

// NFSLocking is an alias for block.NFSLocking.
type NFSLocking = block.NFSLocking

// NFSRecovery is an alias for block.NFSRecovery.
type NFSRecovery = block.NFSRecovery

// NFSSecurity is an alias for block.NFSSecurity.
type NFSSecurity = block.NFSSecurity

// NFSTransport is an alias for block.NFSTransport.
type NFSTransport = block.NFSTransport

// ExternalVolumeConfigV1Alpha1 is an external disk mount configuration document.
//
//	description: |
//	  External volumes allow to mount volumes that were created outside of Talos,
//	  over the network or API. Volume will be mounted under `/var/mnt/<name>`.
//	  The external volume config name should not conflict with user volume names.
//	examples:
//	  - value: exampleExternalVolumeConfigV1Alpha1Virtiofs()
//	  - value: exampleExternalVolumeConfigV1Alpha1NFS()
//	alias: ExternalVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ExternalVolumeConfig
type ExternalVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the mount.
	//
	//     Name might be between 1 and 34 characters long and can only contain:
	//     lowercase and uppercase ASCII letters, digits, and hyphens.
	MetaName string `yaml:"name"`
	//   description: |
	//     Filesystem type.
	//   values:
	//     - virtiofs
	//     - nfs
	//  schema:
	//    type: string
	FilesystemType FilesystemType `yaml:"filesystemType"`
	//   description: |
	//     The mount describes additional mount options.
	MountSpec ExternalMountSpec `yaml:"mount,omitempty"`
}

// ExternalMountSpec describes how the external volume is mounted.
type ExternalMountSpec struct {
	//   description: |
	//     Mount the volume read-only.
	MountReadOnly *bool `yaml:"readOnly,omitempty"`
	//   description: |
	//     If true, disable file access time updates.
	MountDisableAccessTime *bool `yaml:"disableAccessTime,omitempty"`
	//   description: |
	//     Enable secure mount options (nosuid, nodev, noexec).
	//
	//     Defaults to true for better security.
	MountSecure *bool `yaml:"secure,omitempty"`

	//   description: |
	//     Virtiofs mount options.
	MountVirtiofs *VirtiofsMountSpec `yaml:"virtiofs,omitempty"`
	//   description: |
	//     NFS mount options.
	MountNFS *NFSMountSpec `yaml:"nfs,omitempty"`
}

// VirtiofsMountSpec describes Virtiofs mount options.
type VirtiofsMountSpec struct {
	//   description: |
	//     Selector tag for the Virtiofs mount.
	VirtiofsTag string `yaml:"tag"`
}

// NFSMountSpec describes NFS mount options.
type NFSMountSpec struct {
	//   description: |
	//     NFS server hostname or IP address.
	NFSServer string `yaml:"server"`
	//   description: |
	//     Absolute path of the NFS export.
	NFSPath string `yaml:"path"`
	//   description: |
	//     NFS protocol version.
	//   values:
	//     - "3"
	//     - "4"
	//     - "4.1"
	//     - "4.2"
	NFSVersion NFSVersion `yaml:"version"`
	//   description: |
	//     NFS server port. If unset, the kernel default is used.
	NFSPort uint16 `yaml:"port,omitempty"`
	//   description: |
	//     NFS transport protocol. If unset, the kernel default is used.
	//   values:
	//     - tcp
	//     - tcp6
	//     - udp
	//     - udp6
	NFSTransport *NFSTransport `yaml:"transport,omitempty"`
	//   description: |
	//     NFS mount protocol port. Only valid with NFSv3. If unset, rpcbind discovery is used.
	NFSMountPort uint16 `yaml:"mountPort,omitempty"`
	//   description: |
	//     NFS mount transport protocol. Only valid with NFSv3. Must use the same address family as
	//     `transport`. If unset, the kernel default is used.
	//   values:
	//     - tcp
	//     - tcp6
	//     - udp
	//     - udp6
	NFSMountTransport *NFSTransport `yaml:"mountTransport,omitempty"`
	//   description: |
	//     NFSv3 locking mode. Defaults to local because Talos does not run rpc.statd by default.
	//   values:
	//     - local
	//     - remote
	NFSLocking *NFSLocking `yaml:"locking,omitempty"`
	//   description: |
	//     Recovery behavior after an NFS request times out. Soft modes can risk data corruption.
	//   values:
	//     - hard
	//     - soft
	//     - soft-error
	NFSRecovery *NFSRecovery `yaml:"recovery,omitempty"`
	//   description: |
	//     NFS request timeout in deciseconds.
	NFSTimeout uint32 `yaml:"timeout,omitempty"`
	//   description: |
	//     Number of NFS request retransmissions before recovery action is taken.
	NFSRetransmissions *uint32 `yaml:"retransmissions,omitempty"`
	//   description: |
	//     Maximum NFS read request payload in bytes. Must be a multiple of 1024 between 1024 and 1048576.
	NFSReadSize uint32 `yaml:"readSize,omitempty"`
	//   description: |
	//     Maximum NFS write request payload in bytes. Must be a multiple of 1024 between 1024 and 1048576.
	NFSWriteSize uint32 `yaml:"writeSize,omitempty"`
	//   description: |
	//     Number of TCP connections to the NFS server. Must be between 1 and 16.
	NFSConnections uint8 `yaml:"connections,omitempty"`
	//   description: |
	//     Use a privileged source port. The kernel default is used when unset.
	NFSReservedPort *bool `yaml:"reservedPort,omitempty"`
	//   description: |
	//     NFS RPC security flavor. Kerberos flavors are not supported because Talos does not run rpc.gssd.
	//   values:
	//     - none
	//     - sys
	NFSSecurity *NFSSecurity `yaml:"security,omitempty"`
}

// NewExternalVolumeConfigV1Alpha1 creates a new user mount config document.
func NewExternalVolumeConfigV1Alpha1() *ExternalVolumeConfigV1Alpha1 {
	return &ExternalVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ExternalVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleExternalVolumeConfigV1Alpha1Virtiofs() *ExternalVolumeConfigV1Alpha1 {
	cfg := NewExternalVolumeConfigV1Alpha1()
	cfg.MetaName = "mount1"
	cfg.FilesystemType = block.FilesystemTypeVirtiofs
	cfg.MountSpec.MountVirtiofs = &VirtiofsMountSpec{
		VirtiofsTag: "Data",
	}

	return cfg
}

func exampleExternalVolumeConfigV1Alpha1NFS() *ExternalVolumeConfigV1Alpha1 {
	cfg := NewExternalVolumeConfigV1Alpha1()
	cfg.MetaName = "mount2"
	cfg.FilesystemType = block.FilesystemTypeNFS
	cfg.MountSpec.MountNFS = &NFSMountSpec{
		NFSServer:  "10.0.0.10",
		NFSPath:    "/export",
		NFSVersion: block.NFSVersion4Point1,
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *ExternalVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *ExternalVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,dupl
func (s *ExternalVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.MetaName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	}

	if len(s.MetaName) < 1 || len(s.MetaName) > maxExternalVolumeNameLength {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name must be between 1 and %d characters long", maxExternalVolumeNameLength))
	}

	if strings.ContainsFunc(s.MetaName, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		case r >= '0' && r <= '9':
			return false
		case r == '-':
			return false
		default: // invalid symbol
			return true
		}
	}) {
		validationErrors = errors.Join(validationErrors, errors.New("name can only contain lowercase and uppercase ASCII letters, digits, and hyphens"))
	}

	switch s.FilesystemType {
	case block.FilesystemTypeVirtiofs:
		extraWarnings, extraErrors := s.MountSpec.MountVirtiofs.Validate()

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.FilesystemTypeNFS:
		extraWarnings, extraErrors := s.MountSpec.MountNFS.Validate()

		warnings = append(warnings, extraWarnings...)
		validationErrors = errors.Join(validationErrors, extraErrors)

	case block.FilesystemTypeNone, block.FilesystemTypeXFS, block.FilesystemTypeVFAT, block.FilesystemTypeEXT4, block.FilesystemTypeISO9660, block.FilesystemTypeSwap:
		fallthrough

	default:
		validationErrors = errors.Join(validationErrors, fmt.Errorf("invalid filesystem type: %s", s.FilesystemType))
	}

	return warnings, validationErrors
}

// ExternalVolumeConfigSignal is a signal for user mount config.
func (s *ExternalVolumeConfigV1Alpha1) ExternalVolumeConfigSignal() {}

// Type implements config.ExternalVolumeConfig interface.
func (s *ExternalVolumeConfigV1Alpha1) Type() FilesystemType {
	return s.FilesystemType
}

// Mount implements config.ExternalVolumeConfig interface.
func (s *ExternalVolumeConfigV1Alpha1) Mount() config.ExternalVolumeMountConfig {
	return s.MountSpec
}

// ReadOnly implements config.ExternalVolumeMountConfig interface.
func (s ExternalMountSpec) ReadOnly() bool {
	return pointer.SafeDeref(s.MountReadOnly)
}

// DisableAccessTime implements config.ExternalVolumeMountConfig interface.
func (s ExternalMountSpec) DisableAccessTime() bool {
	return pointer.SafeDeref(s.MountDisableAccessTime)
}

// Secure implements config.ExternalVolumeMountConfig interface.
func (s ExternalMountSpec) Secure() bool {
	if s.MountSecure == nil {
		return true
	}

	return *s.MountSecure
}

// Virtiofs implements config.VolumeMountConfig interface.
func (s ExternalMountSpec) Virtiofs() optional.Optional[config.ExternalVolumeMountConfigSpec] {
	if s.MountVirtiofs == nil {
		return optional.None[config.ExternalVolumeMountConfigSpec]()
	}

	return optional.Some[config.ExternalVolumeMountConfigSpec](*s.MountVirtiofs)
}

// NFS implements config.ExternalVolumeMountConfig interface.
func (s ExternalMountSpec) NFS() optional.Optional[config.ExternalVolumeMountConfigSpec] {
	if s.MountNFS == nil {
		return optional.None[config.ExternalVolumeMountConfigSpec]()
	}

	return optional.Some[config.ExternalVolumeMountConfigSpec](*s.MountNFS)
}

// Source implements config.ExternalVolumeMountConfigSpec interface.
func (s VirtiofsMountSpec) Source() string {
	return s.VirtiofsTag
}

// Parameters implements config.ExternalVolumeMountConfigSpec interface.
func (s VirtiofsMountSpec) Parameters() ([]block.ParameterSpec, error) {
	return nil, nil
}

// Validate implements config.Validator interface.
func (s *VirtiofsMountSpec) Validate() ([]string, error) {
	var validationErrors error

	if s == nil {
		return nil, errors.New("virtiofs mount spec is required")
	}

	if s.VirtiofsTag == "" {
		validationErrors = errors.Join(validationErrors, errors.New("virtiofs tag is required"))
	}

	return nil, validationErrors
}

// Source implements config.ExternalVolumeMountConfigSpec interface.
func (s NFSMountSpec) Source() string {
	server := s.NFSServer

	if addr, err := netip.ParseAddr(server); err == nil && addr.Is6() {
		server = "[" + server + "]"
	}

	return server + ":" + s.NFSPath
}

// Parameters implements config.ExternalVolumeMountConfigSpec interface.
func (s NFSMountSpec) Parameters() ([]block.ParameterSpec, error) {
	networkParameters := s.networkParameters()
	lockingParameters := s.lockingParameters()
	recoveryParameters := s.recoveryParameters()
	tuningParameters := s.tuningParameters()

	params := make([]block.ParameterSpec, 0, 1+len(networkParameters)+len(lockingParameters)+len(recoveryParameters)+len(tuningParameters))
	params = append(params, block.NewStringParameter("vers", s.NFSVersion.String()))
	params = append(params, networkParameters...)
	params = append(params, lockingParameters...)
	params = append(params, recoveryParameters...)
	params = append(params, tuningParameters...)

	return params, nil
}

func (s NFSMountSpec) networkParameters() []block.ParameterSpec {
	var params []block.ParameterSpec

	if s.NFSPort != 0 {
		params = append(params, block.NewStringParameter("port", strconv.FormatUint(uint64(s.NFSPort), 10)))
	}

	if s.NFSTransport != nil {
		params = append(params, block.NewStringParameter("proto", s.NFSTransport.String()))
	}

	if s.NFSMountPort != 0 {
		params = append(params, block.NewStringParameter("mountport", strconv.FormatUint(uint64(s.NFSMountPort), 10)))
	}

	if s.NFSMountTransport != nil {
		params = append(params, block.NewStringParameter("mountproto", s.NFSMountTransport.String()))
	}

	return params
}

func (s NFSMountSpec) lockingParameters() []block.ParameterSpec {
	if s.NFSVersion != block.NFSVersion3 {
		return nil
	}

	if s.NFSLocking != nil && *s.NFSLocking == block.NFSLockingRemote {
		return []block.ParameterSpec{block.NewBooleanParameter("lock")}
	}

	return []block.ParameterSpec{block.NewBooleanParameter("nolock")}
}

func (s NFSMountSpec) recoveryParameters() []block.ParameterSpec {
	if s.NFSRecovery == nil {
		return nil
	}

	switch *s.NFSRecovery {
	case block.NFSRecoveryHard:
		return []block.ParameterSpec{block.NewBooleanParameter("hard")}
	case block.NFSRecoverySoft:
		return []block.ParameterSpec{block.NewBooleanParameter("soft")}
	case block.NFSRecoverySoftError:
		return []block.ParameterSpec{block.NewBooleanParameter("softerr")}
	default:
		return nil
	}
}

func (s NFSMountSpec) tuningParameters() []block.ParameterSpec {
	var params []block.ParameterSpec

	if s.NFSTimeout != 0 {
		params = append(params, block.NewStringParameter("timeo", strconv.FormatUint(uint64(s.NFSTimeout), 10)))
	}

	if s.NFSRetransmissions != nil {
		params = append(params, block.NewStringParameter("retrans", strconv.FormatUint(uint64(*s.NFSRetransmissions), 10)))
	}

	if s.NFSReadSize != 0 {
		params = append(params, block.NewStringParameter("rsize", strconv.FormatUint(uint64(s.NFSReadSize), 10)))
	}

	if s.NFSWriteSize != 0 {
		params = append(params, block.NewStringParameter("wsize", strconv.FormatUint(uint64(s.NFSWriteSize), 10)))
	}

	if s.NFSConnections != 0 {
		params = append(params, block.NewStringParameter("nconnect", strconv.FormatUint(uint64(s.NFSConnections), 10)))
	}

	params = append(params, s.reservedPortParameters()...)

	if s.NFSSecurity != nil {
		params = append(params, block.NewStringParameter("sec", s.NFSSecurity.String()))
	}

	return params
}

func (s NFSMountSpec) reservedPortParameters() []block.ParameterSpec {
	if s.NFSReservedPort == nil {
		return nil
	}

	if *s.NFSReservedPort {
		return []block.ParameterSpec{block.NewBooleanParameter("resvport")}
	}

	return []block.ParameterSpec{block.NewBooleanParameter("noresvport")}
}

// Validate implements config.Validator interface.
func (s *NFSMountSpec) Validate() ([]string, error) {
	if s == nil {
		return nil, errors.New("NFS mount spec is required")
	}

	return nil, errors.Join(
		s.validateRequiredFields(),
		s.validateVersion(),
		s.validateTransport(),
		s.validateV3Options(),
		s.validateLocking(),
		s.validateRecovery(),
		s.validateIOSizes(),
		s.validateConnections(),
		s.validateSecurity(),
	)
}

func (s *NFSMountSpec) validateRequiredFields() error {
	var validationErrors error

	if s.NFSServer == "" {
		validationErrors = errors.Join(validationErrors, errors.New("NFS server is required"))
	}

	if s.NFSPath == "" {
		validationErrors = errors.Join(validationErrors, errors.New("NFS path is required"))
	} else if !path.IsAbs(s.NFSPath) {
		validationErrors = errors.Join(validationErrors, errors.New("NFS path must be absolute"))
	}

	return validationErrors
}

func (s *NFSMountSpec) validateVersion() error {
	if s.NFSVersion != block.NFSVersion3 && s.NFSVersion != block.NFSVersion4 && s.NFSVersion != block.NFSVersion4Point1 && s.NFSVersion != block.NFSVersion4Point2 {
		return errors.New("NFS version must be one of 3, 4, 4.1, or 4.2")
	}

	return nil
}

func (s *NFSMountSpec) validateTransport() error {
	var validationErrors error

	if s.NFSVersion != block.NFSVersion3 && s.NFSTransport != nil && s.NFSTransport.IsUDP() {
		validationErrors = errors.Join(validationErrors, errors.New("NFSv4 transport must be tcp or tcp6"))
	}

	// Talos never emits a mountaddr parameter, so the kernel checks the mount transport netid against
	// the NFS server address: a family mismatch between the two can only ever fail at mount time.
	if s.NFSTransport != nil && s.NFSMountTransport != nil && s.NFSTransport.IsIPv6() != s.NFSMountTransport.IsIPv6() {
		validationErrors = errors.Join(validationErrors, errors.New("NFS mount transport address family must match NFS transport address family"))
	}

	return validationErrors
}

func (s *NFSMountSpec) validateV3Options() error {
	var validationErrors error

	if s.NFSVersion != block.NFSVersion3 && s.NFSMountPort != 0 {
		validationErrors = errors.Join(validationErrors, errors.New("NFS mount port is only valid with NFSv3"))
	}

	if s.NFSVersion != block.NFSVersion3 && s.NFSMountTransport != nil {
		validationErrors = errors.Join(validationErrors, errors.New("NFS mount transport is only valid with NFSv3"))
	}

	return validationErrors
}

func (s *NFSMountSpec) validateLocking() error {
	if s.NFSLocking != nil && *s.NFSLocking != block.NFSLockingLocal && *s.NFSLocking != block.NFSLockingRemote {
		return errors.New("NFS locking must be either local or remote")
	}

	if s.NFSVersion != block.NFSVersion3 && s.NFSLocking != nil {
		return errors.New("NFS locking is only configurable with NFSv3")
	}

	return nil
}

func (s *NFSMountSpec) validateRecovery() error {
	if s.NFSRecovery != nil && *s.NFSRecovery != block.NFSRecoveryHard && *s.NFSRecovery != block.NFSRecoverySoft && *s.NFSRecovery != block.NFSRecoverySoftError {
		return errors.New("NFS recovery must be one of hard, soft, or soft-error")
	}

	return nil
}

func (s *NFSMountSpec) validateIOSizes() error {
	var validationErrors error

	if s.NFSReadSize != 0 && (s.NFSReadSize < 1024 || s.NFSReadSize > 1048576 || s.NFSReadSize%1024 != 0) {
		validationErrors = errors.Join(validationErrors, errors.New("NFS read size must be a multiple of 1024 between 1024 and 1048576"))
	}

	if s.NFSWriteSize != 0 && (s.NFSWriteSize < 1024 || s.NFSWriteSize > 1048576 || s.NFSWriteSize%1024 != 0) {
		validationErrors = errors.Join(validationErrors, errors.New("NFS write size must be a multiple of 1024 between 1024 and 1048576"))
	}

	return validationErrors
}

func (s *NFSMountSpec) validateConnections() error {
	var validationErrors error

	if s.NFSConnections > 16 {
		validationErrors = errors.Join(validationErrors, errors.New("NFS connections must be between 1 and 16"))
	}

	if s.NFSConnections != 0 && s.NFSTransport != nil && s.NFSTransport.IsUDP() {
		validationErrors = errors.Join(validationErrors, errors.New("NFS connections require TCP transport"))
	}

	return validationErrors
}

func (s *NFSMountSpec) validateSecurity() error {
	if s.NFSSecurity != nil && *s.NFSSecurity != block.NFSSecurityNone && *s.NFSSecurity != block.NFSSecuritySys {
		return errors.New("NFS security must be either none or sys")
	}

	return nil
}
