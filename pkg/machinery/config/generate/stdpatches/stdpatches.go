// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package stdpatches contains standard patches applied to Talos machine configurations.
package stdpatches

import (
	"github.com/siderolabs/go-pointer"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// WithStaticHostname returns a patch that sets a static hostname in the machine configuration.
func WithStaticHostname(versionContract *config.VersionContract, hostname string) ([]byte, error) {
	if versionContract.MultidocNetworkConfigSupported() {
		hostnameConfig := network.NewHostnameConfigV1Alpha1()
		hostnameConfig.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindOff)
		hostnameConfig.ConfigHostname = hostname

		return patchFromDocument(hostnameConfig)
	}

	return patchFromV1Alpha1(map[string]any{
		"machine": map[string]any{
			"network": map[string]any{
				"hostname": hostname,
			},
		},
	})
}

func patchFromDocument(doc configconfig.Document) ([]byte, error) {
	ctr, err := container.New(doc)
	if err != nil {
		return nil, err
	}

	return ctr.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
}

func patchFromV1Alpha1(doc any) ([]byte, error) {
	return yaml.Marshal(doc)
}
