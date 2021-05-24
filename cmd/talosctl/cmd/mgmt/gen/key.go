// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gen

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/cli"
)

var keyName string

// keyCmd represents the gen key command.
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Generates an Ed25519 private key",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := x509.NewEd25519Key()
		if err != nil {
			return fmt.Errorf("error generating key: %w", err)
		}

		if err := ioutil.WriteFile(keyName+".key", key.PrivateKeyPEM, 0o600); err != nil {
			return fmt.Errorf("error writing key: %w", err)
		}

		return nil
	},
}

func init() {
	keyCmd.Flags().StringVar(&keyName, "name", "", "the basename of the generated file")
	cli.Should(cobra.MarkFlagRequired(keyCmd.Flags(), "name"))

	Cmd.AddCommand(keyCmd)
}
