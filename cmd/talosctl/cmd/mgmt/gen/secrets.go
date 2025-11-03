// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
)

var genSecretsCmdFlags struct {
	outputFile               string
	talosVersion             string
	fromKubernetesPki        string
	fromControlplaneConfig   string
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
			secretsBundle   *secrets.Bundle
			versionContract *config.VersionContract
			err             error
		)

		if genSecretsCmdFlags.talosVersion != "" {
			versionContract, err = config.ParseContractFromVersion(genSecretsCmdFlags.talosVersion)
			if err != nil {
				return fmt.Errorf("invalid talos-version: %w", err)
			}
		}

		switch {
		case genSecretsCmdFlags.fromKubernetesPki != "":
			secretsBundle, err = secrets.NewBundleFromKubernetesPKI(genSecretsCmdFlags.fromKubernetesPki,
				genSecretsCmdFlags.kubernetesBootstrapToken, versionContract)
		case genSecretsCmdFlags.fromControlplaneConfig != "":
			var cfg config.Provider

			cfg, err = configloader.NewFromFile(genSecretsCmdFlags.fromControlplaneConfig)
			if err != nil {
				return fmt.Errorf("failed to load controlplane config: %w", err)
			}

			secretsBundle = secrets.NewBundleFromConfig(secrets.NewFixedClock(time.Now()), cfg)
		default:
			secretsBundle, err = secrets.NewBundle(secrets.NewFixedClock(time.Now()),
				versionContract,
			)
		}

		if err != nil {
			return fmt.Errorf("failed to create secrets bundle: %w", err)
		}

		return writeSecretsBundleToFile(secretsBundle)
	},
}

func writeSecretsBundleToFile(bundle *secrets.Bundle) error {
	bundleBytes, err := yaml.Marshal(bundle)
	if err != nil {
		return err
	}

	if err = validateFileExists(genSecretsCmdFlags.outputFile); err != nil {
		return err
	}

	return os.WriteFile(genSecretsCmdFlags.outputFile, bundleBytes, 0o600)
}

func init() {
	genSecretsCmd.Flags().StringVarP(&genSecretsCmdFlags.outputFile, "output-file", "o", "secrets.yaml", "path of the output file")
	genSecretsCmd.Flags().StringVar(&genSecretsCmdFlags.talosVersion, "talos-version", "", "the desired Talos version to generate secrets bundle for (backwards compatibility, e.g. v0.8)")
	genSecretsCmd.Flags().StringVar(&genSecretsCmdFlags.fromControlplaneConfig, "from-controlplane-config", "", "use the provided controlplane Talos machine configuration as input")
	genSecretsCmd.Flags().StringVarP(&genSecretsCmdFlags.fromKubernetesPki, "from-kubernetes-pki", "p", "", "use a Kubernetes PKI directory (e.g. /etc/kubernetes/pki) as input")
	genSecretsCmd.Flags().StringVarP(&genSecretsCmdFlags.kubernetesBootstrapToken, "kubernetes-bootstrap-token", "t", "", "use the provided bootstrap token as input")

	Cmd.AddCommand(genSecretsCmd)
}
