// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runtime-spec/specs-go"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
	"github.com/siderolabs/talos/pkg/machinery/labels"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

var (
	// General.

	// ErrRequiredSection denotes a section is required.
	ErrRequiredSection = errors.New("required config section")
	// ErrRequiredSectionOptions denotes at least one section is required.
	ErrRequiredSectionOptions = errors.New("required either config section to be set")
	// ErrInvalidVersion denotes that the config file version is invalid.
	ErrInvalidVersion = errors.New("invalid config version")
	// ErrMutuallyExclusive denotes that config sections are mutually exclusive.
	ErrMutuallyExclusive = errors.New("config sections are mutually exclusive")
	// ErrEmpty denotes that config section should have at least a single field defined.
	ErrEmpty = errors.New("config section should contain at least one field")

	// Security.

	// ErrEmptyKeyCert denotes that crypto key/cert combination should not be empty.
	ErrEmptyKeyCert = errors.New("key/cert combination should not be empty")
	// ErrInvalidCert denotes that the certificate specified is invalid.
	ErrInvalidCert = errors.New("certificate is invalid")
	// ErrInvalidCertType denotes that the certificate type is invalid.
	ErrInvalidCertType = errors.New("certificate type is invalid")

	// Services.

	// ErrUnsupportedCNI denotes that the specified CNI is invalid.
	ErrUnsupportedCNI = errors.New("unsupported CNI driver")
	// ErrInvalidTrustdToken denotes that a trustd token has not been specified.
	ErrInvalidTrustdToken = errors.New("trustd token is invalid")

	// Networking.

	// ErrInvalidAddress denotes that a bad address was provided.
	ErrInvalidAddress = errors.New("invalid network address")
)

// NetworkDeviceCheck defines the function type for checks.
type NetworkDeviceCheck func(*Device, map[string]string) ([]string, error)

// Validate implements the config.Provider interface.
//
//nolint:gocyclo,cyclop
func (c *Config) Validate(mode validation.RuntimeMode, options ...validation.Option) ([]string, error) {
	var (
		warnings []string
		result   *multierror.Error
	)

	opts := validation.NewOptions(options...)

	if c.MachineConfig == nil {
		result = multierror.Append(result, errors.New("machine instructions are required"))

		return nil, result.ErrorOrNil()
	}

	if err := c.ClusterConfig.Validate(c.Machine().Type().IsControlPlane()); err != nil {
		result = multierror.Append(result, err)
	}

	if mode.RequiresInstall() {
		if c.MachineConfig.MachineInstall == nil {
			result = multierror.Append(result, fmt.Errorf("install instructions are required in %q mode", mode))
		} else {
			matcher, err := c.MachineConfig.MachineInstall.DiskMatchExpression()
			if err != nil {
				result = multierror.Append(result, fmt.Errorf("install disk selector is invalid: %w", err))
			}

			if c.MachineConfig.MachineInstall.InstallDisk == "" && matcher == nil {
				result = multierror.Append(result, errors.New("either install disk or diskSelector should be defined"))
			}
		}
	}

	if mode.InContainer() {
		// require that HostDNS features are enabled to passthrough container DNS to kube-dns
		if !c.Machine().Features().HostDNS().Enabled() {
			result = multierror.Append(result, errors.New("feature HostDNS should be enabled in container mode (.machine.features.hostDNS.enabled)"))
		}

		if !c.Machine().Features().HostDNS().ForwardKubeDNSToHost() {
			result = multierror.Append(result, errors.New("feature HostDNS should forward kube-dns to host in container mode (.machine.features.hostDNS.forwardKubeDNSToHost)"))
		}
	}

	if t := c.Machine().Type(); t != machine.TypeUnknown && t.String() != c.MachineConfig.MachineType {
		warnings = append(warnings, fmt.Sprintf("use %q instead of %q for machine type", t.String(), c.MachineConfig.MachineType))
	}

	if c.Machine().Security().IssuingCA() == nil && len(c.Machine().Security().AcceptedCAs()) == 0 {
		result = multierror.Append(result, errors.New("issuing CA or some accepted CAs are required (.machine.ca, machine.acceptedCAs)"))
	}

	switch c.Machine().Type() {
	case machine.TypeInit, machine.TypeControlPlane:
		warn, err := ValidateCNI(c.Cluster().Network().CNI())
		warnings = append(warnings, warn...)
		result = multierror.Append(result, err)

		if c.Machine().Security().IssuingCA() == nil {
			result = multierror.Append(result, errors.New("issuing CA is required (.machine.ca)"))
		} else if len(c.Machine().Security().IssuingCA().Key) == 0 {
			result = multierror.Append(result, errors.New("issuing CA key is required for controlplane nodes (.machine.ca.key)"))
		}
	case machine.TypeWorker:
		for _, d := range c.Machine().Network().Devices() {
			if d.VIPConfig() != nil {
				result = multierror.Append(result, errors.New("virtual (shared) IP is not allowed on non-controlplane nodes"))
			}

			for _, vlan := range d.Vlans() {
				if vlan.VIPConfig() != nil {
					result = multierror.Append(result, errors.New("virtual (shared) IP is not allowed on non-controlplane nodes"))
				}
			}
		}

		if c.Machine().Security().IssuingCA() != nil {
			if len(c.Machine().Security().IssuingCA().Key) > 0 {
				result = multierror.Append(result, errors.New("issuing Talos API CA key is not allowed on non-controlplane nodes (.machine.ca)"))
			}

			if len(c.Machine().Security().IssuingCA().Crt) == 0 && len(c.Machine().Security().AcceptedCAs()) == 0 {
				result = multierror.Append(result, errors.New("trusted CA certificates are required on non-controlplane nodes (.machine.ca.crt, .machine.acceptedCAs)"))
			}
		}

		if c.Cluster().IssuingCA() != nil && len(c.Cluster().IssuingCA().Key) > 0 {
			result = multierror.Append(result, errors.New("issuing Kubernetes API CA key is not allowed on non-controlplane nodes (.cluster.ca)"))
		}
	case machine.TypeUnknown:
		fallthrough

	default:
		result = multierror.Append(result, fmt.Errorf("unknown machine type %q", c.MachineConfig.MachineType))
	}

	if c.MachineConfig.MachineNetwork != nil {
		allSecondaryInterfaces := map[string]string{}

		for _, device := range c.MachineConfig.MachineNetwork.NetworkInterfaces {
			if device.Bond() != nil && device.Bridge() != nil {
				result = multierror.Append(result, fmt.Errorf("interface has both bridge and bond sections set %q: %w", device.Interface(), ErrMutuallyExclusive))
			}

			var myInterfaces []string
			if device.Bond() != nil {
				myInterfaces = device.Bond().Interfaces()

				if len(device.Bond().Interfaces()) > 0 && len(device.Bond().Selectors()) > 0 {
					result = multierror.Append(result, fmt.Errorf("interface %q has both interfaces and selectors set: %w", device.Interface(), ErrMutuallyExclusive))
				}
			}

			if device.Bridge() != nil {
				myInterfaces = device.Bridge().Interfaces()
			}

			for _, iface := range myInterfaces {
				if otherIface, exists := allSecondaryInterfaces[iface]; exists && otherIface != device.Interface() {
					result = multierror.Append(result, fmt.Errorf("interface %q is declared as part of two separate links: %q and %q", iface, otherIface, device.Interface()))
				}

				allSecondaryInterfaces[iface] = device.Interface()
			}
		}

		for _, device := range c.MachineConfig.MachineNetwork.NetworkInterfaces {
			warn, err := ValidateNetworkDevices(device, allSecondaryInterfaces, CheckDeviceInterface, CheckDeviceAddressing, CheckDeviceRoutes)
			warnings = append(warnings, warn...)
			result = multierror.Append(result, err)
		}

		if c.Machine().Network().KubeSpan().Enabled() {
			if c.Machine().Network().KubeSpan().MTU() < constants.KubeSpanLinkMinimumMTU {
				result = multierror.Append(result, fmt.Errorf("kubespan link MTU must be at least %d", constants.KubeSpanLinkMinimumMTU))
			}
		}
	}

	for i, disk := range c.MachineConfig.MachineDisks {
		if disk == nil {
			result = multierror.Append(result, fmt.Errorf("machine.disks[%d] is null", i))

			continue
		}

		for i, pt := range disk.DiskPartitions {
			if pt.DiskSize == 0 && i != len(disk.DiskPartitions)-1 {
				result = multierror.Append(result, fmt.Errorf("partition for disk %q is set to occupy full disk, but it's not the last partition in the list", disk.Device()))
			}
		}
	}

	if c.MachineConfig.MachineKubelet != nil {
		warn, err := c.MachineConfig.MachineKubelet.Validate()
		warnings = append(warnings, warn...)
		result = multierror.Append(result, err)
	}

	for _, label := range []string{constants.EphemeralPartitionLabel, constants.StatePartitionLabel} {
		encryptionConfig := c.MachineConfig.SystemDiskEncryption().Get(label)
		if encryptionConfig != nil {
			if len(encryptionConfig.Keys()) == 0 {
				result = multierror.Append(result, fmt.Errorf("partition %q: no encryption keys provided", label))
			}

			slotsInUse := map[int]struct{}{}
			for _, key := range encryptionConfig.Keys() {
				if _, inUse := slotsInUse[key.Slot()]; inUse {
					result = multierror.Append(result, fmt.Errorf("partition %q: encryption key slot %d is already in use", label, key.Slot()))
				}

				slotsInUse[key.Slot()] = struct{}{}

				if key.NodeID() == nil && key.Static() == nil && key.KMS() == nil && key.TPM() == nil {
					result = multierror.Append(result, fmt.Errorf("partition %q: encryption key at slot %d doesn't have the configuration parameters", label, key.Slot()))
				}
			}
		}
	}

	if c.Machine().Network().KubeSpan().Enabled() {
		if !c.Cluster().Discovery().Enabled() {
			result = multierror.Append(result, errors.New(".cluster.discovery should be enabled when .machine.network.kubespan is enabled"))
		}

		if c.Cluster().ID() == "" {
			result = multierror.Append(result, errors.New(".cluster.id should be set when .machine.network.kubespan is enabled"))
		}

		if c.Cluster().Secret() == "" {
			result = multierror.Append(result, errors.New(".cluster.secret should be set when .machine.network.kubespan is enabled"))
		}

		for _, cidr := range c.Machine().Network().KubeSpan().Filters().Endpoints() {
			cidr = strings.TrimPrefix(cidr, "!")

			if _, err := sideronet.ParseSubnetOrAddress(cidr); err != nil {
				result = multierror.Append(result, fmt.Errorf("KubeSpan endpoint filer is not valid: %q", cidr))
			}
		}
	}

	if c.MachineConfig.MachineLogging != nil {
		err := c.MachineConfig.MachineLogging.Validate()
		result = multierror.Append(result, err)
	}

	if c.MachineConfig.MachineInstall != nil {
		extensions := map[string]struct{}{}

		for _, ext := range c.MachineConfig.MachineInstall.InstallExtensions {
			if _, exists := extensions[ext.Image()]; exists {
				result = multierror.Append(result, fmt.Errorf("duplicate system extension %q", ext.Image()))
			}

			extensions[ext.Image()] = struct{}{}
		}

		if len(extensions) > 0 {
			warnings = append(warnings, ".machine.install.extensions is deprecated, please see https://www.talos.dev/latest/talos-guides/install/boot-assets/")
		}
	}

	if err := labels.Validate(c.MachineConfig.MachineNodeLabels); err != nil {
		result = multierror.Append(result, fmt.Errorf("invalid machine node labels: %w", err))
	}

	if err := labels.ValidateAnnotations(c.MachineConfig.MachineNodeAnnotations); err != nil {
		result = multierror.Append(result, fmt.Errorf("invalid machine node annotations: %w", err))
	}

	if err := labels.ValidateTaints(c.MachineConfig.MachineNodeTaints); err != nil {
		result = multierror.Append(result, fmt.Errorf("invalid machine node taints: %w", err))
	}

	if c.Machine().Features().KubernetesTalosAPIAccess().Enabled() {
		if !c.Machine().Features().RBACEnabled() {
			result = multierror.Append(result, errors.New("feature API RBAC should be enabled when Kubernetes Talos API Access feature is enabled"))
		}

		if !c.Machine().Type().IsControlPlane() {
			result = multierror.Append(result, errors.New("feature Kubernetes Talos API Access can only be enabled on control plane machines"))
		}

		for _, r := range c.Machine().Features().KubernetesTalosAPIAccess().AllowedRoles() {
			if !role.All.Includes(role.Role(r)) {
				result = multierror.Append(result, fmt.Errorf("invalid role %q in allowed roles for Kubernetes Talos API Access", r))
			}
		}
	}

	if c.MachineConfig.MachineFeatures != nil && c.MachineConfig.MachineFeatures.FeatureNodeAddressSortAlgorithm != "" {
		if _, err := nethelpers.AddressSortAlgorithmString(c.MachineConfig.MachineFeatures.FeatureNodeAddressSortAlgorithm); err != nil {
			result = multierror.Append(result, fmt.Errorf("invalid node address sort algorithm: %w", err))
		}
	}

	if c.ConfigPersist != nil && !*c.ConfigPersist {
		result = multierror.Append(result, errors.New(".persist should be enabled"))
	}

	if len(c.Machine().BaseRuntimeSpecOverrides()) > 0 {
		// try to unmarshal the overrides to ensure they are valid
		jsonSpec, err := json.Marshal(c.Machine().BaseRuntimeSpecOverrides())
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to marshal base runtime spec overrides: %w", err))
		} else {
			var ociSpec specs.Spec

			if err := json.Unmarshal(jsonSpec, &ociSpec); err != nil {
				result = multierror.Append(result, fmt.Errorf("failed to unmarshal base runtime spec overrides: %w", err))
			}
		}
	}

	for key, val := range c.MachineConfig.MachineRegistries.RegistryConfig {
		if val == nil {
			result = multierror.Append(result, fmt.Errorf("registries.config[%q] is null", key))
		}
	}

	for key, val := range c.MachineConfig.MachineRegistries.RegistryMirrors {
		if val == nil {
			result = multierror.Append(result, fmt.Errorf("registries.mirrors[%q] is null", key))
		}
	}

	if opts.Strict {
		for _, w := range warnings {
			result = multierror.Append(result, fmt.Errorf("warning: %s", w))
		}

		warnings = nil
	}

	return warnings, result.ErrorOrNil()
}

var rxDNSNameRegexp = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^([a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62}){1}(\.[a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62})*[\._]?$`)
})

func isValidDNSName(name string) bool {
	if name == "" || len(name)-strings.Count(name, ".") > 255 {
		return false
	}

	return rxDNSNameRegexp().MatchString(name)
}

// Validate validates the config.
//
//nolint:gocyclo
func (c *ClusterConfig) Validate(isControlPlane bool) error {
	var result *multierror.Error

	if c == nil {
		return errors.New("cluster instructions are required")
	}

	if c.ControlPlane == nil || c.ControlPlane.Endpoint == nil {
		return errors.New("cluster controlplane endpoint is required")
	}

	if err := sideronet.ValidateEndpointURI(c.ControlPlane.Endpoint.URL.String()); err != nil {
		result = multierror.Append(result, fmt.Errorf("invalid controlplane endpoint: %w", err))
	}

	if c.ClusterNetwork != nil && c.ClusterNetwork.DNSDomain != "" && !isValidDNSName(c.ClusterNetwork.DNSDomain) {
		result = multierror.Append(result, fmt.Errorf("%q is not a valid DNS name", c.ClusterNetwork.DNSDomain))
	}

	if ecp := c.ExternalCloudProviderConfig; ecp != nil {
		result = multierror.Append(result, ecp.Validate())
	}

	if c.EtcdConfig != nil {
		if isControlPlane {
			result = multierror.Append(result, c.EtcdConfig.Validate())
		} else {
			result = multierror.Append(result, errors.New("etcd config is only allowed on control plane machines"))
		}
	}

	if c.ClusterCA != nil && !isControlPlane && len(c.ClusterCA.Key) > 0 {
		result = multierror.Append(result, errors.New("cluster CA key is not allowed on non-controlplane nodes (.cluster.ca)"))
	}

	result = multierror.Append(
		result,
		c.ClusterInlineManifests.Validate(),
		c.ClusterDiscoveryConfig.Validate(c),
		c.APIServerConfig.Validate(),
		c.ControllerManagerConfig.Validate(),
		c.SchedulerConfig.Validate(),
	)

	return result.ErrorOrNil()
}

// ValidateCNI validates CNI config.
//
//nolint:gocyclo
func ValidateCNI(cni config.CNI) ([]string, error) {
	var (
		warnings []string
		result   *multierror.Error
	)

	switch cni.Name() {
	case constants.FlannelCNI:
		if len(cni.URLs()) != 0 {
			err := fmt.Errorf(`"urls" field should be empty for %q CNI`, cni.Name())
			result = multierror.Append(result, err)
		}

	case constants.NoneCNI:
		if len(cni.URLs()) != 0 {
			err := fmt.Errorf(`"urls" field should be empty for %q CNI`, cni.Name())
			result = multierror.Append(result, err)
		}

		if len(cni.Flannel().ExtraArgs()) != 0 {
			err := fmt.Errorf(`"flanneldExtraArgs" field should be empty for %q CNI`, cni.Name())
			result = multierror.Append(result, err)
		}

	case constants.CustomCNI:
		if len(cni.URLs()) == 0 {
			warn := fmt.Sprintf(`"urls" field should not be empty for %q CNI`, cni.Name())
			warnings = append(warnings, warn)
		}

		if len(cni.Flannel().ExtraArgs()) != 0 {
			err := fmt.Errorf(`"flanneldExtraArgs" field should be empty for %q CNI`, cni.Name())
			result = multierror.Append(result, err)
		}

		for _, u := range cni.URLs() {
			if err := sideronet.ValidateEndpointURI(u); err != nil {
				result = multierror.Append(result, err)
			}
		}

	default:
		err := fmt.Errorf("cni name should be one of [%q, %q, %q]", constants.FlannelCNI, constants.CustomCNI, constants.NoneCNI)
		result = multierror.Append(result, err)
	}

	return warnings, result.ErrorOrNil()
}

// Validate validates external cloud provider configuration.
func (ecp *ExternalCloudProviderConfig) Validate() error {
	if !ecp.Enabled() && (len(ecp.ExternalManifests) != 0) {
		return errors.New("external cloud provider is disabled, but manifests are provided")
	}

	var result *multierror.Error

	for _, url := range ecp.ExternalManifests {
		if err := sideronet.ValidateEndpointURI(url); err != nil {
			err = fmt.Errorf("invalid external cloud provider manifest url %q: %w", url, err)
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

// Validate the inline manifests.
func (manifests ClusterInlineManifests) Validate() error {
	var result *multierror.Error

	manifestNames := map[string]struct{}{}

	for _, manifest := range manifests {
		if strings.TrimSpace(manifest.InlineManifestName) == "" {
			result = multierror.Append(result, errors.New("inline manifest name can't be empty"))
		}

		if _, ok := manifestNames[manifest.InlineManifestName]; ok {
			result = multierror.Append(result, fmt.Errorf("inline manifest name %q is duplicate", manifest.InlineManifestName))
		}

		manifestNames[manifest.InlineManifestName] = struct{}{}
	}

	return result.ErrorOrNil()
}

// Validate the discovery config.
func (c *ClusterDiscoveryConfig) Validate(clusterCfg *ClusterConfig) error {
	var result *multierror.Error

	if c == nil || !c.Enabled() {
		return nil
	}

	if c.Registries().Service().Enabled() {
		url, err := url.ParseRequestURI(c.Registries().Service().Endpoint())
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("cluster discovery service registry endpoint is invalid: %w", err))
		} else if url.Path != "" && url.Path != "/" {
			result = multierror.Append(result, errors.New("cluster discovery service path should be empty"))
		}

		if clusterCfg.ID() == "" {
			result = multierror.Append(result, errors.New("cluster discovery service requires .cluster.id"))
		}

		if clusterCfg.Secret() == "" {
			result = multierror.Append(result, errors.New("cluster discovery service requires .cluster.secret"))
		}
	}

	return result.ErrorOrNil()
}

// ValidateNetworkDevices runs the specified validation checks specific to the
// network devices.
func ValidateNetworkDevices(d *Device, secondaryInterfaces map[string]string, checks ...NetworkDeviceCheck) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, errors.New("empty device")
	}

	if d.Ignore() {
		return nil, result.ErrorOrNil()
	}

	var warnings []string

	for _, check := range checks {
		warn, err := check(d, secondaryInterfaces)
		warnings = append(warnings, warn...)
		result = multierror.Append(result, err)
	}

	return warnings, result.ErrorOrNil()
}

// CheckDeviceInterface ensures that the interface has been specified.
//
//nolint:gocyclo
func CheckDeviceInterface(d *Device, _ map[string]string) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, errors.New("empty device")
	}

	if d.DeviceInterface == "" && d.DeviceSelector == nil {
		result = multierror.Append(result, fmt.Errorf("[%s], [%s]: %w", "networking.os.device.interface", "networking.os.device.deviceSelector", ErrRequiredSectionOptions))
	} else if d.DeviceInterface != "" && d.DeviceSelector != nil {
		result = multierror.Append(result, fmt.Errorf("[%s], [%s]: %w", "networking.os.device.interface", "networking.os.device.deviceSelector", ErrMutuallyExclusive))
	}

	if d.DeviceSelector != nil && reflect.ValueOf(d.DeviceSelector).Elem().IsZero() {
		result = multierror.Append(result, fmt.Errorf("[%s]: %w", "networking.os.device.deviceSelector", ErrEmpty))
	}

	if d.DeviceBond != nil {
		result = multierror.Append(result, checkBond(d.DeviceBond))
	}

	if d.DeviceWireguardConfig != nil {
		result = multierror.Append(result, checkWireguard(d.DeviceWireguardConfig))
	}

	if d.DeviceVlans != nil {
		result = multierror.Append(result, checkVlans(d))
	}

	return nil, result.ErrorOrNil()
}

//nolint:gocyclo,cyclop
func checkBond(b *Bond) error {
	var result *multierror.Error

	bondMode, err := nethelpers.BondModeByName(b.BondMode)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.BondXmitHashPolicyByName(b.BondHashPolicy)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.LACPRateByName(b.BondLACPRate)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.ARPValidateByName(b.BondARPValidate)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.ARPAllTargetsByName(b.BondARPAllTargets)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.PrimaryReselectByName(b.BondPrimaryReselect)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.FailOverMACByName(b.BondFailOverMac)
	if err != nil {
		result = multierror.Append(result, err)
	}

	_, err = nethelpers.ADSelectByName(b.BondADSelect)
	if err != nil {
		result = multierror.Append(result, err)
	}

	if b.BondMIIMon == 0 {
		if b.BondUpDelay != 0 {
			result = multierror.Append(result, errors.New("bond.upDelay can't be set if miiMon is zero"))
		}

		if b.BondDownDelay != 0 {
			result = multierror.Append(result, errors.New("bond.downDelay can't be set if miiMon is zero"))
		}
	} else {
		if b.BondUpDelay%b.BondMIIMon != 0 {
			result = multierror.Append(result, errors.New("bond.upDelay should be a multiple of miiMon"))
		}

		if b.BondDownDelay%b.BondMIIMon != 0 {
			result = multierror.Append(result, errors.New("bond.downDelay should be a multiple of miiMon"))
		}
	}

	if len(b.BondARPIPTarget) > 0 {
		result = multierror.Append(result, errors.New("bond.arpIPTarget is not supported"))
	}

	if b.BondLACPRate != "" && bondMode != nethelpers.BondMode8023AD {
		result = multierror.Append(result, errors.New("bond.lacpRate is only available in 802.3ad mode"))
	}

	if b.BondADActorSystem != "" {
		result = multierror.Append(result, errors.New("bond.adActorSystem is not supported"))
	}

	if (bondMode == nethelpers.BondMode8023AD || bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondARPValidate != "" {
		result = multierror.Append(result, fmt.Errorf("bond.arpValidate is not available in %s mode", bondMode))
	}

	if !(bondMode == nethelpers.BondModeActiveBackup || bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondPrimary != "" {
		result = multierror.Append(result, fmt.Errorf("bond.primary is not available in %s mode", bondMode))
	}

	if (bondMode == nethelpers.BondMode8023AD || bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondARPInterval > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.arpInterval is not available in %s mode", bondMode))
	}

	if bondMode != nethelpers.BondModeRoundrobin && b.BondPacketsPerSlave > 1 {
		result = multierror.Append(result, fmt.Errorf("bond.packetsPerSlave is not available in %s mode", bondMode))
	}

	if !(bondMode == nethelpers.BondModeALB || bondMode == nethelpers.BondModeTLB) && b.BondTLBDynamicLB > 0 {
		result = multierror.Append(result, fmt.Errorf("bond.tlbDynamicTLB is not available in %s mode", bondMode))
	}

	if bondMode != nethelpers.BondMode8023AD && b.BondADActorSysPrio > 0 {
		result = multierror.Append(result, errors.New("bond.adActorSysPrio is only available in 802.3ad mode"))
	}

	if bondMode != nethelpers.BondMode8023AD && b.BondADUserPortKey > 0 {
		result = multierror.Append(result, errors.New("bond.adUserPortKey is only available in 802.3ad mode"))
	}

	return result.ErrorOrNil()
}

func checkWireguard(b *DeviceWireguardConfig) error {
	var result *multierror.Error

	// avoid pulling in wgctrl code to keep machinery dependencies slim
	checkKey := func(key string) error {
		raw, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return err
		}

		if len(raw) != 32 {
			return fmt.Errorf("wrong key %q length: %d", key, len(raw))
		}

		return nil
	}

	if err := checkKey(b.WireguardPrivateKey); err != nil {
		result = multierror.Append(result, fmt.Errorf("private key is invalid: %w", err))
	}

	for _, peer := range b.WireguardPeers {
		if err := checkKey(peer.WireguardPublicKey); err != nil {
			result = multierror.Append(result, fmt.Errorf("public key invalid: %w", err))
		}

		if peer.WireguardEndpoint != "" {
			if !sideronet.AddressContainsPort(peer.WireguardEndpoint) {
				result = multierror.Append(result, fmt.Errorf("peer endpoint %q is invalid", peer.WireguardEndpoint))
			}
		}

		for _, allowedIP := range peer.WireguardAllowedIPs {
			if _, _, err := net.ParseCIDR(allowedIP); err != nil {
				result = multierror.Append(result, fmt.Errorf("peer allowed IP %q is invalid: %w", allowedIP, err))
			}
		}
	}

	return result.ErrorOrNil()
}

func checkVlans(d *Device) error {
	var result *multierror.Error

	// check VLAN addressing
	for _, vlan := range d.DeviceVlans {
		if len(vlan.VlanAddresses) > 0 && vlan.VlanCIDR != "" {
			result = multierror.Append(result, fmt.Errorf("[%s] %s.%d: %s", "networking.os.device.vlan", d.DeviceInterface, vlan.VlanID, "vlan can't have both .cidr and .addresses set"))
		}

		if vlan.VlanCIDR != "" {
			if err := validateIPOrCIDR(vlan.VlanCIDR); err != nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %s.%d: %w", "networking.os.device.vlan.CIDR", d.DeviceInterface, vlan.VlanID, err))
			}
		}

		for _, address := range vlan.VlanAddresses {
			if err := validateIPOrCIDR(address); err != nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %s.%d: %w", "networking.os.device.vlan.addresses", d.DeviceInterface, vlan.VlanID, err))
			}
		}
	}

	return result.ErrorOrNil()
}

func validateIPOrCIDR(address string) error {
	if strings.IndexByte(address, '/') >= 0 {
		_, _, err := net.ParseCIDR(address)

		return err
	}

	if ip := net.ParseIP(address); ip == nil {
		return fmt.Errorf("failed to parse IP address %q", address)
	}

	return nil
}

// CheckDeviceAddressing ensures that an appropriate addressing method.
// has been specified.
//
//nolint:gocyclo
func CheckDeviceAddressing(d *Device, secondaryInterfaces map[string]string) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, errors.New("empty device")
	}

	var warnings []string

	if _, paired := secondaryInterfaces[d.Interface()]; paired {
		if d.DHCP() || d.DeviceCIDR != "" || len(d.DeviceAddresses) > 0 || d.DeviceVIPConfig != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %s", "networking.os.device", d.DeviceInterface, "bonded/bridged interface shouldn't have any addressing methods configured"))
		}
	}

	// ensure either legacy CIDR is set or new addresses, but not both
	if len(d.DeviceAddresses) > 0 && d.DeviceCIDR != "" {
		result = multierror.Append(result, fmt.Errorf("[%s] %q: %s", "networking.os.device", d.DeviceInterface, "interface can't have both .cidr and .addresses set"))
	}

	// ensure cidr is a valid address
	if d.DeviceCIDR != "" {
		if err := validateIPOrCIDR(d.DeviceCIDR); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.CIDR", d.DeviceInterface, err))
		}

		warnings = append(warnings, fmt.Sprintf("%q: machine.network.interface.cidr is deprecated, please use machine.network.interface.addresses", d.DeviceInterface))
	}

	// ensure addresses are valid addresses
	for _, address := range d.DeviceAddresses {
		if err := validateIPOrCIDR(address); err != nil {
			result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.addresses", d.DeviceInterface, err))
		}
	}

	// check VIP IP is valid
	if d.DeviceVIPConfig != nil {
		if ip := net.ParseIP(d.DeviceVIPConfig.IP()); ip == nil {
			result = multierror.Append(result, fmt.Errorf("[%s] failed to parse %q as IP address", "networking.os.device.vip", d.DeviceVIPConfig.IP()))
		}
	}

	return warnings, result.ErrorOrNil()
}

// CheckDeviceRoutes ensures that the specified routes are valid.
//
//nolint:gocyclo
func CheckDeviceRoutes(d *Device, _ map[string]string) ([]string, error) {
	var result *multierror.Error

	if d == nil {
		return nil, errors.New("empty device")
	}

	if len(d.DeviceRoutes) == 0 {
		return nil, result.ErrorOrNil()
	}

	for idx, route := range d.DeviceRoutes {
		if route.Network() != "" {
			if _, _, err := net.ParseCIDR(route.Network()); err != nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].network", route.Network(), ErrInvalidAddress))
			}
		}

		if route.Gateway() != "" {
			if ip := net.ParseIP(route.Gateway()); ip == nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].gateway", route.Gateway(), ErrInvalidAddress))
			}
		}

		if route.Gateway() == "" && route.Network() == "" {
			result = multierror.Append(result, fmt.Errorf("[%s]: %s", "networking.os.device.route["+strconv.Itoa(idx)+"]", "either network or gateway should be set"))
		}

		if route.Source() != "" {
			if ip := net.ParseIP(route.Source()); ip == nil {
				result = multierror.Append(result, fmt.Errorf("[%s] %q: %w", "networking.os.device.route["+strconv.Itoa(idx)+"].source", route.Source(), ErrInvalidAddress))
			}
		}
	}

	return nil, result.ErrorOrNil()
}

// Validate kubelet configuration.
func (k *KubeletConfig) Validate() ([]string, error) {
	var result *multierror.Error

	if k.KubeletNodeIP != nil {
		for _, cidr := range k.KubeletNodeIP.KubeletNodeIPValidSubnets {
			cidr = strings.TrimPrefix(cidr, "!")

			if _, err := sideronet.ParseSubnetOrAddress(cidr); err != nil {
				result = multierror.Append(result, fmt.Errorf("kubelet nodeIP subnet is not valid: %q", cidr))
			}
		}
	}

	for _, field := range kubelet.ProtectedConfigurationFields {
		if _, exists := k.KubeletExtraConfig.Object[field]; exists {
			result = multierror.Append(result, fmt.Errorf("kubelet configuration field %q can't be overridden", field))
		}
	}

	return nil, result.ErrorOrNil()
}

// Validate etcd configuration.
func (e *EtcdConfig) Validate() error {
	var result *multierror.Error

	if e.CA() == nil {
		result = multierror.Append(result, ErrEmptyKeyCert)
	}

	if e.EtcdSubnet != "" && len(e.EtcdAdvertisedSubnets) > 0 {
		result = multierror.Append(result, errors.New("etcd subnet can't be set when advertised subnets are set"))
	}

	for _, cidr := range e.AdvertisedSubnets() {
		cidr = strings.TrimPrefix(cidr, "!")

		if _, err := sideronet.ParseSubnetOrAddress(cidr); err != nil {
			result = multierror.Append(result, fmt.Errorf("etcd advertised subnet is not valid: %q", cidr))
		}
	}

	for _, cidr := range e.ListenSubnets() {
		cidr = strings.TrimPrefix(cidr, "!")

		if _, err := sideronet.ParseSubnetOrAddress(cidr); err != nil {
			result = multierror.Append(result, fmt.Errorf("etcd listen subnet is not valid: %q", cidr))
		}
	}

	return result.ErrorOrNil()
}

// RuntimeValidate validates the config in runtime context.
//
// In runtime context, resource state is available.
//
//nolint:gocyclo
func (c *Config) RuntimeValidate(ctx context.Context, st state.State, mode validation.RuntimeMode, opt ...validation.Option) ([]string, error) {
	var (
		warnings []string
		result   *multierror.Error
	)

	if c.MachineConfig != nil {
		if mode.RequiresInstall() && c.MachineConfig.MachineInstall != nil {
			diskExpr, err := c.MachineConfig.MachineInstall.DiskMatchExpression()
			if err != nil {
				result = multierror.Append(result, fmt.Errorf("install disk selector is invalid: %w", err))
			} else if diskExpr != nil {
				matchedDisks, err := blockhelpers.MatchDisks(ctx, st, diskExpr)
				if err != nil {
					result = multierror.Append(result, err)
				}

				if len(matchedDisks) == 0 {
					result = multierror.Append(result, fmt.Errorf("no disks matched the expression: %s", diskExpr))
				}
			}
		}

		// if booted using sd-boot, extra kernel arguments are not supported
		if _, err := os.Stat("/sys/firmware/efi/efivars/StubInfo-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f"); err == nil {
			if len(c.MachineConfig.Install().ExtraKernelArgs()) > 0 {
				warnings = append(warnings, "extra kernel arguments are not supported when booting using SDBoot")
			}
		}

		if len(c.MachineConfig.Install().Extensions()) > 0 {
			warnings = append(warnings, ".machine.install.extensions is deprecated, please see https://www.talos.dev/latest/talos-guides/install/boot-assets/")
		}

		if err := ValidateKubernetesImageTag(c.Machine().Kubelet().Image()); err != nil {
			result = multierror.Append(result, fmt.Errorf("kubelet image is not valid: %w", err))
		}
	}

	if c.ClusterConfig != nil && c.MachineConfig != nil {
		if c.Machine().Type().IsControlPlane() {
			for _, spec := range []struct {
				name     string
				imageRef string
			}{
				{
					name:     "kube-apiserver",
					imageRef: c.Cluster().APIServer().Image(),
				},
				{
					name:     "kube-controller-manager",
					imageRef: c.Cluster().ControllerManager().Image(),
				},
				{
					name:     "kube-scheduler",
					imageRef: c.Cluster().Scheduler().Image(),
				},
			} {
				if err := ValidateKubernetesImageTag(spec.imageRef); err != nil {
					result = multierror.Append(result, fmt.Errorf("%s image is not valid: %w", spec.name, err))
				}
			}
		}
	}

	return warnings, result.ErrorOrNil()
}

// ValidateKubernetesImageTag validates the Kubernetes image tag format.
func ValidateKubernetesImageTag(imageRef string) error {
	// this method is called from RuntimeValidate, so we are inside running Talos,
	// so the version of Talos is available, and we can check compatibility
	currentTalosVersion, err := compatibility.ParseTalosVersion(version.NewVersion())
	if err != nil {
		return fmt.Errorf("failed to parse Talos version: %w", err)
	}

	k8sVersion, err := KubernetesVersionFromImageRef(imageRef)
	if err != nil {
		return fmt.Errorf("failed to parse Kubernetes version from image reference %q: %w", imageRef, err)
	}

	return k8sVersion.SupportedWith(currentTalosVersion)
}

// KubernetesVersionFromImageRef parses the Kubernetes version from the image reference.
func KubernetesVersionFromImageRef(ref string) (*compatibility.KubernetesVersion, error) {
	idx := strings.LastIndex(ref, ":v")
	if idx == -1 {
		return nil, fmt.Errorf("invalid image reference: %q", ref)
	}

	versionPart := ref[idx+2:]

	if shaIndex := strings.Index(versionPart, "@"); shaIndex != -1 {
		versionPart = versionPart[:shaIndex]
	}

	return compatibility.ParseKubernetesVersion(versionPart)
}
