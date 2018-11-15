package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/src/initramfs/cmd/osctl/pkg/client"
	"github.com/spf13/cobra"
)

// dmesgCmd represents the dmesg command
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			if err := cmd.Usage(); err != nil {
				// TODO: How should we handle this?
				os.Exit(1)
			}
			os.Exit(1)
		}
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
		if err := c.Dmesg(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(dmesgCmd)
}
