// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
)

var genSecretsCmdFlags struct {
	outputFile               string
	talosVersion             string
	fromKubernetesPki        string
	kubernetesBootstrapToken string
}

// genSecretsCmd represents the `gen secrets` command.
var genSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Generates a secrets bundle file which can later be used to generate a config",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			secretsBundle   *generate.SecretsBundle
			versionContract *config.VersionContract
			err             error
		)

		if genSecretsCmdFlags.talosVersion != "" {
			versionContract, err = config.ParseContractFromVersion(genSecretsCmdFlags.talosVersion)
			if err != nil {
				return fmt.Errorf("invalid talos-version: %w", err)
			}
		}

		if genSecretsCmdFlags.fromKubernetesPki != "" {
			secretsBundle, err = generate.NewSecretsBundleFromKubernetesPKI(genSecretsCmdFlags.fromKubernetesPki,
				genSecretsCmdFlags.kubernetesBootstrapToken, versionContract)
			if err != nil {
				return fmt.Errorf("failed to create secrets bundle: %w", err)
			}

			return writeSecretsBundleToFile(secretsBundle)
		}

		secretsBundle, err = generate.NewSecretsBundle(generate.NewClock(),
			generate.WithVersionContract(versionContract),
		)
		if err != nil {
			return fmt.Errorf("failed to create secrets bundle: %w", err)
		}

		return writeSecretsBundleToFile(secretsBundle)
	},
}

func writeSecretsBundleToFile(bundle *generate.SecretsBundle) error {
	bundleBytes, err := yaml.Marshal(bundle)
	if err != nil {
		return err
	}

	return os.WriteFile(genSecretsCmdFlags.outputFile, bundleBytes, 0o600)
}

func init() {
	genSecretsCmd.Flags().StringVarP(&genSecretsCmdFlags.outputFile, "output-file", "o", "secrets.yaml", "path of the output file")
	genSecretsCmd.Flags().StringVar(&genSecretsCmdFlags.talosVersion, "talos-version", "", "the desired Talos version to generate secrets bundle for (backwards compatibility, e.g. v0.8)")
	genSecretsCmd.Flags().StringVarP(&genSecretsCmdFlags.fromKubernetesPki, "from-kubernetes-pki", "p", "", "use a Kubernetes PKI directory (e.g. /etc/kubernetes/pki) as input")
	genSecretsCmd.Flags().StringVarP(&genSecretsCmdFlags.kubernetesBootstrapToken, "kubernetes-bootstrap-token", "t", "", "use the provided bootstrap token as input")

	Cmd.AddCommand(genSecretsCmd)
}
