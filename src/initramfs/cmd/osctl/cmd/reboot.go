package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/osctl/pkg/client"
	"github.com/spf13/cobra"
)

// rebootCmd represents the reboot command
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot a node",
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
		if err := c.Reboot(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(rebootCmd)
}
