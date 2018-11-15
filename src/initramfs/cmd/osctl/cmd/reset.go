// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/src/initramfs/cmd/osctl/pkg/client"
	"github.com/spf13/cobra"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a node",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := client.NewDefaultClientCredentials()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c, err := client.NewClient(port, creds)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := c.Reset(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
