package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/client"
	"github.com/spf13/cobra"
)

// dmesgCmd represents the dmesg command
var dmesgCmd = &cobra.Command{
	Use:   "dmesg",
	Short: "Retrieve kernel logs",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			cmd.Usage()
			os.Exit(1)
		}
		creds, err := client.NewDefaultClientCredentials()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c, err := client.NewClient(address, port, creds)
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
