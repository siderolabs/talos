// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestCPUPartitionKubeletValidation(t *testing.T) {
	t.Parallel()

	for _, legacy := range []bool{false, true} {
		for _, managed := range []bool{false, true} {
			for _, cli := range []bool{false, true} {
				name := fmt.Sprintf("legacy=%t/pods=%t/cli=%t", legacy, managed, cli)

				t.Run(name, func(t *testing.T) {
					t.Parallel()

					policy := runtime.NewCPUPartitionConfigV1Alpha1()
					if managed {
						policy.KubepodsConfig = &runtime.CPUPartitionRoot{RootCPUs: "1-3"}
					} else {
						policy.SystemConfig = &runtime.CPUPartitionRoot{RootCPUs: "0"}
					}

					extra := meta.Unstructured{Object: map[string]any{}}

					args := meta.Args{}
					if cli {
						args["reserved-cpus"] = meta.NewArgValue("0", nil)
					} else {
						extra.Object["reservedSystemCPUs"] = "0"
					}

					var doc config.Document
					if legacy {
						doc = &v1alpha1.Config{ //nolint:staticcheck // exercise legacy provider bridge
							MachineConfig: &v1alpha1.MachineConfig{
								MachineType:    "worker",
								MachineCA:      &x509.PEMEncodedCertificateAndKey{Crt: []byte("cert")},
								MachineKubelet: &v1alpha1.KubeletConfig{KubeletExtraConfig: extra, KubeletExtraArgs: args}, //nolint:staticcheck // legacy bridge
							},
							ClusterConfig: &v1alpha1.ClusterConfig{
								ControlPlane: &v1alpha1.ControlPlaneConfig{Endpoint: &v1alpha1.Endpoint{URL: &url.URL{Scheme: "https", Host: "localhost:6443"}}}, //nolint:staticcheck // legacy bridge
							},
						}
					} else {
						kubelet := k8s.NewKubeletConfigV1Alpha1()
						kubelet.KubeletImage = "kubelet:v1.38.0-alpha.1"
						kubelet.KubeletConfig, kubelet.KubeletArgs = extra, args
						doc = kubelet
					}

					cfg, err := container.New(policy, doc)
					require.NoError(t, err)

					_, err = cfg.ValidateAsClient(validationMode{})
					if managed {
						require.ErrorContains(t, err, "owned by CPUPartitionConfig")

						_, containerErr := cfg.ValidateAsClient(validationMode{inContainer: true})
						if containerErr != nil {
							require.NotContains(t, containerErr.Error(), "owned by CPUPartitionConfig", "an inert container-mode policy does not own kubelet CPUs")
						}
					} else {
						require.NoError(t, err)
					}
				})
			}
		}
	}
}
