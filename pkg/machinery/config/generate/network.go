// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//nolint:gocyclo
func (in *Input) generateNetworkConfigs(machine *v1alpha1.MachineConfig) ([]config.Document, error) {
	var documents []config.Document

	if len(in.Options.NetworkConfigOptions) > 0 {
		networkConfig := &v1alpha1.NetworkConfig{} //nolint:staticcheck // using legacy NetworkConfig for older Talos versions

		for _, opt := range in.Options.NetworkConfigOptions {
			if err := opt(machine.Type(), networkConfig); err != nil {
				return nil, err
			}
		}

		machine.MachineNetwork = networkConfig //nolint:staticcheck // using legacy NetworkConfig for older Talos versions
	}

	// generate empty machine.network for backwards compatibility with older Talos versions
	if machine.MachineNetwork == nil && !in.Options.VersionContract.KubeSpanMultidocConfig() { //nolint:staticcheck // using legacy NetworkConfig for older Talos versions
		machine.MachineNetwork = &v1alpha1.NetworkConfig{} //nolint:staticcheck // using legacy NetworkConfig for older Talos versions
	}

	if in.Options.VersionContract.StableHostnameEnabled() && !in.Options.VersionContract.MultidocNetworkConfigSupported() {
		machine.MachineFeatures.StableHostname = pointer.To(true) //nolint:staticcheck // using legacy field for older Talos versions
	}

	if in.Options.VersionContract.MultidocNetworkConfigSupported() {
		hostnameConfig := network.NewHostnameConfigV1Alpha1()
		hostnameConfig.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindStable)

		documents = append(documents, hostnameConfig)
	}

	if kubeSpanEnabled, isSet := in.Options.KubeSpanEnabled.Get(); isSet {
		if in.Options.VersionContract.KubeSpanMultidocConfig() {
			kubeSpanConfig := network.NewKubeSpanV1Alpha1()
			kubeSpanConfig.ConfigEnabled = pointer.To(kubeSpanEnabled)

			documents = append(documents, kubeSpanConfig)
		} else {
			// for older Talos versions, set KubeSpan config in machine.network.kubespan
			machine.MachineNetwork.NetworkKubeSpan = &v1alpha1.NetworkKubeSpan{ //nolint:staticcheck // using legacy NetworkKubeSpan for older Talos versions
				KubeSpanEnabled: pointer.To(kubeSpanEnabled),
			}
		}
	}

	return documents, nil
}
