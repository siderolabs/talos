package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/osctl/pkg/client"
	"github.com/autonomy/dianemo/src/initramfs/pkg/version"
	"github.com/spf13/cobra"
)

var (
	shortVersion bool
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if shortVersion {
			version.PrintShortVersion()
		} else {
			if err := version.PrintLongVersion(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
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
		if err := c.Version(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "Print the short version")
	rootCmd.AddCommand(versionCmd)
}
