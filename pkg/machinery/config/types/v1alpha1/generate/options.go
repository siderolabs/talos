// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/talos-systems/talos/pkg/machinery/config"
	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// GenOption controls generate options specific to input generation.
type GenOption func(o *GenOptions) error

// WithEndpointList specifies endpoints to use when acessing Talos cluster.
func WithEndpointList(endpoints []string) GenOption {
	return func(o *GenOptions) error {
		o.EndpointList = endpoints

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
		o.InstallExtraKernelArgs = args

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

		o.RegistryConfig[host].RegistryTLS.TLSInsecureSkipVerify = true

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

// WithAllowSchedulingOnMasters specifies AllowSchedulingOnMasters flag.
func WithAllowSchedulingOnMasters(enabled bool) GenOption {
	return func(o *GenOptions) error {
		o.AllowSchedulingOnMasters = enabled

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

// GenOptions describes generate parameters.
type GenOptions struct {
	EndpointList               []string
	InstallDisk                string
	InstallImage               string
	InstallExtraKernelArgs     []string
	AdditionalSubjectAltNames  []string
	NetworkConfigOptions       []v1alpha1.NetworkConfigOption
	CNIConfig                  *v1alpha1.CNIConfig
	RegistryMirrors            map[string]*v1alpha1.RegistryMirrorConfig
	RegistryConfig             map[string]*v1alpha1.RegistryConfig
	DNSDomain                  string
	Debug                      bool
	Persist                    bool
	AllowSchedulingOnMasters   bool
	MachineDisks               []*v1alpha1.MachineDisk
	VersionContract            *config.VersionContract
	SystemDiskEncryptionConfig *v1alpha1.SystemDiskEncryptionConfig
}

// DefaultGenOptions returns default options.
func DefaultGenOptions() GenOptions {
	return GenOptions{
		Persist: true,
	}
}
