// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// GenOption controls generate options specific to input generation.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.GenOption instead.
type GenOption = generate.Option

// WithEndpointList specifies endpoints to use when accessing Talos cluster.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithEndpointList instead.
func WithEndpointList(endpoints []string) GenOption {
	return generate.WithEndpointList(endpoints)
}

// WithLocalAPIServerPort specifies the local API server port for the cluster.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithLocalAPIServerPort instead.
func WithLocalAPIServerPort(port int) GenOption {
	return generate.WithLocalAPIServerPort(port)
}

// WithInstallDisk specifies install disk to use in Talos cluster.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithInstallDisk instead.
func WithInstallDisk(disk string) GenOption {
	return generate.WithInstallDisk(disk)
}

// WithAdditionalSubjectAltNames specifies additional SANs.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithAdditionalSubjectAltNames instead.
func WithAdditionalSubjectAltNames(sans []string) GenOption {
	return generate.WithAdditionalSubjectAltNames(sans)
}

// WithInstallImage specifies install container image to use in Talos cluster.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithInstallImage instead.
func WithInstallImage(imageRef string) GenOption {
	return generate.WithInstallImage(imageRef)
}

// WithInstallExtraKernelArgs specifies extra kernel arguments to pass to the installer.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithInstallExtraKernelArgs instead.
func WithInstallExtraKernelArgs(args []string) GenOption {
	return generate.WithInstallExtraKernelArgs(args)
}

// WithNetworkOptions adds network config generation option.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithNetworkOptions instead.
func WithNetworkOptions(opts ...v1alpha1.NetworkConfigOption) GenOption {
	return generate.WithNetworkOptions(opts...)
}

// WithRegistryMirror configures registry mirror endpoint(s).
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithRegistryMirror instead.
func WithRegistryMirror(host string, endpoints ...string) GenOption {
	return generate.WithRegistryMirror(host, endpoints...)
}

// WithRegistryCACert specifies the certificate of the certificate authority which signed certificate of the registry.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithRegistryCACert instead.
func WithRegistryCACert(host, cacert string) GenOption {
	return generate.WithRegistryCACert(host, cacert)
}

// WithRegistryInsecureSkipVerify marks registry host to skip TLS verification.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithRegistryInsecureSkipVerify instead.
func WithRegistryInsecureSkipVerify(host string) GenOption {
	return generate.WithRegistryInsecureSkipVerify(host)
}

// WithDNSDomain specifies domain name to use in Talos cluster.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithDNSDomain instead.
func WithDNSDomain(dnsDomain string) GenOption {
	return generate.WithDNSDomain(dnsDomain)
}

// WithDebug enables verbose logging to console for all services.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithDebug instead.
func WithDebug(enable bool) GenOption {
	return generate.WithDebug(enable)
}

// WithPersist enables persistence of machine config across reboots.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithPersist instead.
func WithPersist(enable bool) GenOption {
	return generate.WithPersist(enable)
}

// WithClusterCNIConfig specifies custom cluster CNI config.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithClusterCNIConfig instead.
func WithClusterCNIConfig(config *v1alpha1.CNIConfig) GenOption {
	return generate.WithClusterCNIConfig(config)
}

// WithUserDisks generates user partitions config.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithUserDisks instead.
func WithUserDisks(disks []*v1alpha1.MachineDisk) GenOption {
	return generate.WithUserDisks(disks)
}

// WithAllowSchedulingOnControlPlanes specifies AllowSchedulingOnControlPlane flag.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithAllowSchedulingOnControlPlanes instead.
func WithAllowSchedulingOnControlPlanes(enabled bool) GenOption {
	return generate.WithAllowSchedulingOnControlPlanes(enabled)
}

// WithVersionContract specifies version contract to use when generating.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithVersionContract instead.
func WithVersionContract(versionContract *config.VersionContract) GenOption {
	return generate.WithVersionContract(versionContract)
}

// WithSystemDiskEncryption specifies encryption settings for the system disk partitions.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithSystemDiskEncryption instead.
func WithSystemDiskEncryption(cfg *v1alpha1.SystemDiskEncryptionConfig) GenOption {
	return generate.WithSystemDiskEncryption(cfg)
}

// WithRoles specifies user roles.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithRoles instead.
func WithRoles(roles role.Set) GenOption {
	return generate.WithRoles(roles)
}

// WithClusterDiscovery enables cluster discovery feature.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithClusterDiscovery instead.
func WithClusterDiscovery(enabled bool) GenOption {
	return generate.WithClusterDiscovery(enabled)
}

// WithSysctls merges list of sysctls with new values.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithSysctls instead.
func WithSysctls(params map[string]string) GenOption {
	return generate.WithSysctls(params)
}

// WithSecrets reads secrets from a provided file.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.WithSecrets instead.
func WithSecrets(file string) GenOption {
	return func(o *generate.Options) error {
		bundle, err := secrets.LoadBundle(file)
		if err != nil {
			return err
		}

		return generate.WithSecretsBundle(bundle)(o)
	}
}

// GenOptions describes generate parameters.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.GenOptions instead.
type GenOptions = generate.Options

// DefaultGenOptions returns default options.
//
// Deprecated: use github.com/siderolabs/talos/pkg/machinery/config/generate.DefaultGenOptions instead.
func DefaultGenOptions() GenOptions {
	return generate.DefaultOptions()
}
