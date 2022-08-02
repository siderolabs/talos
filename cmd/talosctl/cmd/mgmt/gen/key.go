// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var genKeyCmdFlags struct {
	name string
}

// genKeyCmd represents the `gen key` command.
var genKeyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generates an Ed25519 private key",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := x509.NewEd25519Key()
		if err != nil {
			return fmt.Errorf("error generating key: %w", err)
		}

		if err := os.WriteFile(genKeyCmdFlags.name+".key", key.PrivateKeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

func init() {
	genKeyCmd.Flags().StringVar(&genKeyCmdFlags.name, "name", "", "the basename of the generated file")
	cli.Should(cobra.MarkFlagRequired(genKeyCmd.Flags(), "name"))

	Cmd.AddCommand(genKeyCmd)
}
