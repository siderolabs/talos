// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
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

// WithNetworkConfig allows to pass network config to be used.
func WithNetworkConfig(network *v1alpha1.NetworkConfig) GenOption {
	return func(o *GenOptions) error {
		o.NetworkConfig = network

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

// WithArchitecture specifies architecture of the Talos cluster.
func WithArchitecture(arch string) GenOption {
	return func(o *GenOptions) error {
		o.Architecture = arch

		return nil
	}
}

// GenOptions describes generate parameters.
type GenOptions struct {
	EndpointList              []string
	InstallDisk               string
	InstallImage              string
	InstallExtraKernelArgs    []string
	AdditionalSubjectAltNames []string
	NetworkConfig             *v1alpha1.NetworkConfig
	CNIConfig                 *v1alpha1.CNIConfig
	RegistryMirrors           map[string]*v1alpha1.RegistryMirrorConfig
	DNSDomain                 string
	Architecture              string
	Debug                     bool
	Persist                   bool
}

// DefaultGenOptions returns default options.
func DefaultGenOptions() GenOptions {
	return GenOptions{
		Persist:      true,
		Architecture: "amd64",
	}
}
