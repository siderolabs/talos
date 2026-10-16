// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/pkg/cluster"
)

// kubernetesEndpointValue is a string flag value which is validated as a Kubernetes API endpoint override.
type kubernetesEndpointValue struct {
	value *string
}

// String implements pflag.Value interface.
func (v *kubernetesEndpointValue) String() string {
	return *v.value
}

// Set implements pflag.Value interface.
func (v *kubernetesEndpointValue) Set(value string) error {
	if value != "" {
		if err := cluster.ValidateKubernetesEndpoint(value); err != nil {
			return err
		}
	}

	*v.value = value

	return nil
}

// Type implements pflag.Value interface.
func (v *kubernetesEndpointValue) Type() string {
	return "string"
}

// KubernetesEndpointVar defines a Kubernetes API endpoint override flag, which is validated when the flags are parsed.
func KubernetesEndpointVar(flags *pflag.FlagSet, p *string, name, value, usage string) {
	*p = value

	flags.Var(&kubernetesEndpointValue{value: p}, name, usage)
}
