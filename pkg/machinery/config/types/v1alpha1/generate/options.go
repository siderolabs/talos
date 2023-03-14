// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"os"

	"github.com/siderolabs/go-pointer"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/config"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// GenOption controls generate options specific to input generation.
type GenOption func(o *GenOptions) error

// WithEndpointList specifies endpoints to use when accessing Talos cluster.
func WithEndpointList(endpoints []string) GenOption {
	return func(o *GenOptions) error {
		o.EndpointList = endpoints

		return nil
	}
}

// WithLocalAPIServerPort specifies the local API server port for the cluster.
func WithLocalAPIServerPort(port int) GenOption {
	return func(o *GenOptions) error {
		o.LocalAPIServerPort = port

		return nil
	}
}

// WithInstallDisk specifies install disk to use in Talos cluster.
func WithInstallDisk(disk string) GenOption {
	return func(o *GenOptions) error {
		o.InstallDisk = disk

		return nil
	}
}

// WithAdditionalSubjectAltNames specifies additional SANs.
func WithAdditionalSubjectAltNames(sans []string) GenOption {
	return func(o *GenOptions) error {
		o.AdditionalSubjectAltNames = sans

		return nil
	}
}

// WithInstallImage specifies install container image to use in Talos cluster.
func WithInstallImage(imageRef string) GenOption {
	return func(o *GenOptions) error {
		o.InstallImage = imageRef

		return nil
	}
}

// WithInstallExtraKernelArgs specifies extra kernel arguments to pass to the installer.
func WithInstallExtraKernelArgs(args []string) GenOption {
	return func(o *GenOptions) error {
		o.InstallExtraKernelArgs = append(o.InstallExtraKernelArgs, args...)

		return nil
	}
}

// WithInstallEphemeralSize specifies the ephemeral size to use
func WithInstallEphemeralSize(size string) GenOption {
	return func(o *GenOptions) error {
		o.InstallEphemeralSize = size

		return nil
	}
}

// WithNetworkOptions adds network config generation option.
func WithNetworkOptions(opts ...v1alpha1.NetworkConfigOption) GenOption {
	return func(o *GenOptions) error {
		o.NetworkConfigOptions = append(o.NetworkConfigOptions, opts...)

		return nil
	}
}

// WithRegistryMirror configures registry mirror endpoint(s).
func WithRegistryMirror(host string, endpoints ...string) GenOption {
	return func(o *GenOptions) error {
		if o.RegistryMirrors == nil {
			o.RegistryMirrors = make(map[string]*v1alpha1.RegistryMirrorConfig)
		}

		o.RegistryMirrors[host] = &v1alpha1.RegistryMirrorConfig{MirrorEndpoints: endpoints}

		return nil
	}
}

// WithRegistryCACert specifies the certificate of the certificate authority which signed certificate of the registry.
func WithRegistryCACert(host, cacert string) GenOption {
	return func(o *GenOptions) error {
		if o.RegistryConfig == nil {
			o.RegistryConfig = make(map[string]*v1alpha1.RegistryConfig)
		}

		if _, ok := o.RegistryConfig[host]; !ok {
			o.RegistryConfig[host] = &v1alpha1.RegistryConfig{}
		}

		if o.RegistryConfig[host].RegistryTLS == nil {
			o.RegistryConfig[host].RegistryTLS = &v1alpha1.RegistryTLSConfig{}
		}

		o.RegistryConfig[host].RegistryTLS.TLSCA = v1alpha1.Base64Bytes(cacert)

		return nil
	}
}

// WithRegistryInsecureSkipVerify marks registry host to skip TLS verification.
func WithRegistryInsecureSkipVerify(host string) GenOption {
	return func(o *GenOptions) error {
		if o.RegistryConfig == nil {
			o.RegistryConfig = make(map[string]*v1alpha1.RegistryConfig)
		}

		if _, ok := o.RegistryConfig[host]; !ok {
			o.RegistryConfig[host] = &v1alpha1.RegistryConfig{}
		}

		if o.RegistryConfig[host].RegistryTLS == nil {
			o.RegistryConfig[host].RegistryTLS = &v1alpha1.RegistryTLSConfig{}
		}

		o.RegistryConfig[host].RegistryTLS.TLSInsecureSkipVerify = pointer.To(true)

		return nil
	}
}

// WithDNSDomain specifies domain name to use in Talos cluster.
func WithDNSDomain(dnsDomain string) GenOption {
	return func(o *GenOptions) error {
		o.DNSDomain = dnsDomain

		return nil
	}
}

// WithDebug enables verbose logging to console for all services.
func WithDebug(enable bool) GenOption {
	return func(o *GenOptions) error {
		o.Debug = enable

		return nil
	}
}

// WithPersist enables persistence of machine config across reboots.
func WithPersist(enable bool) GenOption {
	return func(o *GenOptions) error {
		o.Persist = enable

		return nil
	}
}

// WithClusterCNIConfig specifies custom cluster CNI config.
func WithClusterCNIConfig(config *v1alpha1.CNIConfig) GenOption {
	return func(o *GenOptions) error {
		o.CNIConfig = config

		return nil
	}
}

// WithUserDisks generates user partitions config.
func WithUserDisks(disks []*v1alpha1.MachineDisk) GenOption {
	return func(o *GenOptions) error {
		o.MachineDisks = disks

		return nil
	}
}

// WithAllowSchedulingOnControlPlanes specifies AllowSchedulingOnControlPlane flag.
func WithAllowSchedulingOnControlPlanes(enabled bool) GenOption {
	return func(o *GenOptions) error {
		o.AllowSchedulingOnControlPlanes = enabled

		return nil
	}
}

// WithVersionContract specifies version contract to use when generating.
func WithVersionContract(versionContract *config.VersionContract) GenOption {
	return func(o *GenOptions) error {
		o.VersionContract = versionContract

		return nil
	}
}

// WithSystemDiskEncryption specifies encryption settings for the system disk partitions.
func WithSystemDiskEncryption(cfg *v1alpha1.SystemDiskEncryptionConfig) GenOption {
	return func(o *GenOptions) error {
		o.SystemDiskEncryptionConfig = cfg

		return nil
	}
}

// WithRoles specifies user roles.
func WithRoles(roles role.Set) GenOption {
	return func(o *GenOptions) error {
		o.Roles = roles

		return nil
	}
}

// WithClusterDiscovery enables cluster discovery feature.
func WithClusterDiscovery(enabled bool) GenOption {
	return func(o *GenOptions) error {
		o.DiscoveryEnabled = pointer.To(enabled)

		return nil
	}
}

// WithSysctls merges list of sysctls with new values.
func WithSysctls(params map[string]string) GenOption {
	return func(o *GenOptions) error {
		if o.Sysctls == nil {
			o.Sysctls = make(map[string]string)
		}

		for k, v := range params {
			o.Sysctls[k] = v
		}

		return nil
	}
}

// WithSecrets reads secrets from a provided file.
func WithSecrets(file string) GenOption {
	return func(o *GenOptions) error {
		yamlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		var secrets SecretsBundle

		err = yaml.Unmarshal(yamlBytes, &secrets)
		if err != nil {
			return err
		}

		secrets.Clock = NewClock()

		o.Secrets = &secrets

		return nil
	}
}

// GenOptions describes generate parameters.
type GenOptions struct {
	EndpointList                   []string
	InstallDisk                    string
	InstallImage                   string
	InstallExtraKernelArgs         []string
	InstallEphemeralSize           string
	AdditionalSubjectAltNames      []string
	NetworkConfigOptions           []v1alpha1.NetworkConfigOption
	CNIConfig                      *v1alpha1.CNIConfig
	RegistryMirrors                map[string]*v1alpha1.RegistryMirrorConfig
	RegistryConfig                 map[string]*v1alpha1.RegistryConfig
	Sysctls                        map[string]string
	DNSDomain                      string
	Debug                          bool
	Persist                        bool
	AllowSchedulingOnControlPlanes bool
	MachineDisks                   []*v1alpha1.MachineDisk
	VersionContract                *config.VersionContract
	SystemDiskEncryptionConfig     *v1alpha1.SystemDiskEncryptionConfig
	Roles                          role.Set
	DiscoveryEnabled               *bool
	LocalAPIServerPort             int
	Secrets                        *SecretsBundle
}

// DefaultGenOptions returns default options.
func DefaultGenOptions() GenOptions {
	return GenOptions{
		DNSDomain: "cluster.local",
		Persist:   true,
		Roles:     role.MakeSet(role.Admin),
	}
}
