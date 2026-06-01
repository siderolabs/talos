// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubelet defines Talos interface for the kubelet.
package kubelet

// ProtectedConfigurationFields is a list of kubelet config fields that can't be overridden
// with the machine configuration.
var ProtectedConfigurationFields = []string{
	"apiVersion",
	"authentication",
	"authorization",
	"cgroupRoot",
	"kind",
	"kubeletCgroups",
	"port",
	"protectKernelDefaults",
	"resolvConf",
	"rotateCertificates",
	"systemCgroups",
	"staticPodPath",
	"seccompDefault",
}
