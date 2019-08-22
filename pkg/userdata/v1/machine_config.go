/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package v1

// MachineConfig reperesents the machine-specific config values
type MachineConfig struct {
	Type    string           `yaml:"type"`
	Token   string           `yaml:"token"`
	CA      *MachineCAConfig `yaml:"ca,omitempty"`
	Kubelet *KubeletConfig   `yaml:"kubelet,omitempty"`
	Network *NetworkConfig   `yaml:"network,omitempty"`
	Install *Install         `yaml:"install,omitempty"`
}

// KubeletConfig reperesents the kubelet config values
type KubeletConfig struct {
	Image     string            `yaml:"image,omitempty"`
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// NetworkConfig reperesents the machine's networking config values
type NetworkConfig struct {
	Hostname   string    `yaml:"hostname,omitempty"`
	Interfaces []*Device `yaml:"interfaces,omitempty"`
}

// MachineCAConfig reperesents the machine's talos cert config values
type MachineCAConfig struct {
	Crt string `yaml:"crt,omitempty"`
	Key string `yaml:"key,omitempty"`
}
