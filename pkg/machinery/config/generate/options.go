// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"maps"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Option controls generate options specific to input generation.
type Option func(o *Options) error

// WithEndpointList specifies endpoints to use when accessing Talos cluster.
func WithEndpointList(endpoints []string) Option {
	return func(o *Options) error {
		o.EndpointList = endpoints

		return nil
	}
}

// WithLocalAPIServerPort specifies the local API server port for the cluster.
func WithLocalAPIServerPort(port int) Option {
	return func(o *Options) error {
		o.LocalAPIServerPort = port

		return nil
	}
}

// WithKubePrismPort specifies the KubePrism port.
//
// If 0, load balancer is disabled.
// If not set, defaults to enabled with Talos 1.6+.
func WithKubePrismPort(port int) Option {
	return func(o *Options) error {
		o.KubePrismPort = optional.Some(port)

		return nil
	}
}

// WithInstallDisk specifies install disk to use in Talos cluster.
func WithInstallDisk(disk string) Option {
	return func(o *Options) error {
		o.InstallDisk = disk

		return nil
	}
}

// WithAdditionalSubjectAltNames specifies additional SANs.
func WithAdditionalSubjectAltNames(sans []string) Option {
	return func(o *Options) error {
		o.AdditionalSubjectAltNames = append(o.AdditionalSubjectAltNames, sans...)

		return nil
	}
}

// WithInstallImage specifies install container image to use in Talos cluster.
func WithInstallImage(imageRef string) Option {
	return func(o *Options) error {
		o.InstallImage = imageRef

		return nil
	}
}

// WithInstallExtraKernelArgs specifies extra kernel arguments to pass to the installer.
func WithInstallExtraKernelArgs(args []string) Option {
	return func(o *Options) error {
		o.InstallExtraKernelArgs = append(o.InstallExtraKernelArgs, args...)

		return nil
	}
}

// WithNetworkOptions adds network config generation option.
func WithNetworkOptions(opts ...v1alpha1.NetworkConfigOption) Option {
	return func(o *Options) error {
		o.NetworkConfigOptions = append(o.NetworkConfigOptions, opts...)

		return nil
	}
}

// WithRegistryMirror configures registry mirror endpoint(s).
func WithRegistryMirror(host string, endpoints ...string) Option {
	return func(o *Options) error {
		if o.RegistryEndpoints == nil {
			o.RegistryEndpoints = make(map[string][]string)
		}

		o.RegistryEndpoints[host] = append(o.RegistryEndpoints[host], endpoints...)

		return nil
	}
}

// WithRegistryCACert specifies the certificate of the certificate authority which signed certificate of the registry.
func WithRegistryCACert(host, cacert string) Option {
	return func(o *Options) error {
		if o.RegistryCACerts == nil {
			o.RegistryCACerts = make(map[string]string)
		}

		o.RegistryCACerts[host] = cacert

		return nil
	}
}

// WithRegistryInsecureSkipVerify marks registry host to skip TLS verification.
func WithRegistryInsecureSkipVerify(host string) Option {
	return func(o *Options) error {
		if o.RegistryInsecure == nil {
			o.RegistryInsecure = make(map[string]bool)
		}

		o.RegistryInsecure[host] = true

		return nil
	}
}

// WithDNSDomain specifies domain name to use in Talos cluster.
func WithDNSDomain(dnsDomain string) Option {
	return func(o *Options) error {
		o.DNSDomain = dnsDomain

		return nil
	}
}

// WithDebug enables verbose logging to console for all services.
func WithDebug(enable bool) Option {
	return func(o *Options) error {
		o.Debug = enable

		return nil
	}
}

// WithPersist enables persistence of machine config across reboots.
func WithPersist(enable bool) Option {
	return func(o *Options) error {
		o.Persist = enable

		return nil
	}
}

// WithClusterCNIConfig specifies custom cluster CNI config.
func WithClusterCNIConfig(config *v1alpha1.CNIConfig) Option {
	return func(o *Options) error {
		o.CNIConfig = config

		return nil
	}
}

// WithUserDisks generates user partitions config.
//
// Deprecated: use block.UserVolumeConfig instead.
func WithUserDisks(disks []*v1alpha1.MachineDisk) Option {
	return func(o *Options) error {
		o.MachineDisks = disks

		return nil
	}
}

// WithAllowSchedulingOnControlPlanes specifies AllowSchedulingOnControlPlane flag.
func WithAllowSchedulingOnControlPlanes(enabled bool) Option {
	return func(o *Options) error {
		o.AllowSchedulingOnControlPlanes = enabled

		return nil
	}
}

// WithVersionContract specifies version contract to use when generating.
func WithVersionContract(versionContract *config.VersionContract) Option {
	return func(o *Options) error {
		o.VersionContract = versionContract

		return nil
	}
}

// WithRoles specifies user roles.
func WithRoles(roles role.Set) Option {
	return func(o *Options) error {
		o.Roles = roles

		return nil
	}
}

// WithClusterDiscovery enables cluster discovery feature.
func WithClusterDiscovery(enabled bool) Option {
	return func(o *Options) error {
		o.DiscoveryEnabled = pointer.To(enabled)

		return nil
	}
}

// WithSysctls merges list of sysctls with new values.
func WithSysctls(params map[string]string) Option {
	return func(o *Options) error {
		if o.Sysctls == nil {
			o.Sysctls = make(map[string]string)
		}

		maps.Copy(o.Sysctls, params)

		return nil
	}
}

// WithSecretsBundle specifies custom secrets bundle.
func WithSecretsBundle(bundle *secrets.Bundle) Option {
	return func(o *Options) error {
		o.SecretsBundle = bundle

		return nil
	}
}

// WithHostDNSForwardKubeDNSToHost specifies whether to forward kube-dns to host.
func WithHostDNSForwardKubeDNSToHost(forward bool) Option {
	return func(o *Options) error {
		o.HostDNSForwardKubeDNSToHost = optional.Some(forward)

		return nil
	}
}

// Options describes generate parameters.
type Options struct {
	VersionContract *config.VersionContract

	// Custom secrets bundle.
	SecretsBundle *secrets.Bundle

	// Base settings.
	Debug   bool
	Persist bool

	// Machine settings: install.
	InstallDisk            string
	InstallImage           string
	InstallExtraKernelArgs []string

	// Machine disks.
	MachineDisks []*v1alpha1.MachineDisk

	// Machine network settings.
	NetworkConfigOptions []v1alpha1.NetworkConfigOption

	// Machine sysctls.
	Sysctls map[string]string

	// Machine registries.
	RegistryEndpoints map[string][]string
	RegistryCACerts   map[string]string
	RegistryInsecure  map[string]bool

	// Cluster settings.
	DNSDomain                      string
	CNIConfig                      *v1alpha1.CNIConfig
	AllowSchedulingOnControlPlanes bool
	LocalAPIServerPort             int
	AdditionalSubjectAltNames      []string
	DiscoveryEnabled               *bool

	KubePrismPort optional.Optional[int]

	HostDNSForwardKubeDNSToHost optional.Optional[bool]

	// Client options.
	Roles        role.Set
	EndpointList []string
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{
		DNSDomain: "cluster.local",
		Persist:   true,
		Roles:     role.MakeSet(role.Admin),
	}
}
