// nolint: dupl,golint
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/autonomy/talos/internal/app/osctl/internal/client"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/spf13/cobra"
)

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Download the admin.conf from the node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := client.NewDefaultClientCredentials()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c, err := client.NewClient(constants.OsdPort, creds)
		if err != nil {
			log.Fatal(err)
		}
		if err := c.Kubeconfig(); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(kubeconfigCmd)
}
