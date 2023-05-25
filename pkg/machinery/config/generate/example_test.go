// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate_test

import (
	"log"
	"os"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:wsl,testableexamples
func Example() {
	// This is an example of generating a set of machine configuration files for multiple
	// nodes of the cluster from a single cluster-specific cluster.

	// Input values for the config generation:

	// * cluster name and Kubernetes control plane endpoint
	clusterName := "test-cluster"
	controlPlaneEndpoint := "https://kubernetes.example.com:6443"

	// * Kubernetes version to install, using the latest here
	kubernetesVersion := constants.DefaultKubernetesVersion

	// * version contract defines the version of the Talos cluster configuration is generated for
	//   generate package can generate machine configuration compatible with current and previous versions of Talos
	targetVersion := "v1.0"

	// parse the version contract
	var (
		versionContract = config.TalosVersionCurrent //nolint:wastedassign,ineffassign // version of the Talos machinery package
		err             error
	)

	versionContract, err = config.ParseContractFromVersion(targetVersion)
	if err != nil {
		log.Fatalf("failed to parse version contract: %s", err)
	}

	// generate the cluster-wide secrets once and use it for every node machine configuration
	// secrets can be stashed for future use by marshaling the structure to YAML or JSON
	secretsBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), versionContract)
	if err != nil {
		log.Fatalf("failed to generate secrets bundle: %s", err)
	}

	input, err := generate.NewInput(clusterName, controlPlaneEndpoint, kubernetesVersion,
		generate.WithVersionContract(versionContract),
		generate.WithSecretsBundle(secretsBundle),
		generate.WithEndpointList(
			[]string{"172.0.0.1", "172.0.0.2", "172.20.0.3"}, // list of control plane node IP addresses
		),
		// there are many more generate options available which allow to tweak generated config programmatically
	)
	if err != nil {
		log.Fatalf("failed to generate input: %s", err)
	}

	// generate the machine config for each node of the cluster using the secrets
	for _, node := range []string{"machine1", "machine2"} {
		var cfg config.Provider

		// generate the machine config for the node, using the right machine type:
		// * machine.TypeConrolPlane for control plane nodes
		// * machine.TypeWorker for worker nodes
		cfg, err = input.Config(machine.TypeControlPlane)
		if err != nil {
			log.Fatalf("failed to generate config for node %q: %s", node, err)
		}

		// config can be tweaked at this point to add machine-specific configuration, e.g.:
		cfg.RawV1Alpha1().MachineConfig.MachineInstall.InstallDisk = "/dev/sdb"

		// marshal the config to YAML
		var marshaledCfg []byte

		marshaledCfg, err = cfg.Bytes()
		if err != nil {
			log.Fatalf("failed to generate config for node %q: %s", node, err)
		}

		// write the config to a file
		if err = os.WriteFile(clusterName+"-"+node+".yaml", marshaledCfg, 0o600); err != nil {
			log.Fatalf("failed to write config for node %q: %s", node, err)
		}
	}

	// generate the client Talos configuration (for API access, e.g. talosctl)
	clientCfg, err := input.Talosconfig()
	if err != nil {
		log.Fatalf("failed to generate client config: %s", err)
	}

	if err = clientCfg.Save(clusterName + "-talosconfig"); err != nil {
		log.Fatalf("failed to save client config: %s", err)
	}
}
