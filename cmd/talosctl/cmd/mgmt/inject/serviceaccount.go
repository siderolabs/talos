// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inject

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/kubernetes/inject"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

var serviceAccountCmdFlags struct {
	file  string
	roles []string
}

var ServiceAccountCmd = &cobra.Command{
	Use:     fmt.Sprintf("%s [--roles='<ROLE_1>,<ROLE_2>'] -f <manifest.yaml>", constants.ServiceAccountResourceSingular),
	Aliases: []string{constants.ServiceAccountResourceShortName},
	Short:   "Inject Talos API ServiceAccount into Kubernetes manifests",
	Example: fmt.Sprintf(
		`talosctl inject %[1]s --roles="os:admin" -f deployment.yaml > deployment-injected.yaml

Alternatively, stdin can be piped to the command:
cat deployment.yaml | talosctl inject %[1]s --roles="os:admin" -f - > deployment-injected.yaml
`,
		constants.ServiceAccountResourceSingular,
	),
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		var err error

		if serviceAccountCmdFlags.file == "" {
			return cmd.Help()
		}

		reader := os.Stdin

		if serviceAccountCmdFlags.file != "-" {
			reader, err = os.Open(serviceAccountCmdFlags.file)
			if err != nil {
				return err
			}
		}

		injectedYaml, err := inject.ServiceAccount(reader, serviceAccountCmdFlags.roles)
		if err != nil {
			return err
		}

		fmt.Println(string(injectedYaml))

		return nil
	},
}

func init() {
	ServiceAccountCmd.Flags().StringVarP(&serviceAccountCmdFlags.file, "file", "f", "",
		fmt.Sprintf("file with Kubernetes manifests to be injected with %s", constants.ServiceAccountResourceKind))
	ServiceAccountCmd.Flags().StringSliceVarP(&serviceAccountCmdFlags.roles, "roles", "r", []string{string(role.Reader)},
		fmt.Sprintf("roles to add to the generated %s manifests", constants.ServiceAccountResourceKind))
	Cmd.AddCommand(ServiceAccountCmd)
}
