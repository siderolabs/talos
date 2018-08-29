// nolint: dupl,golint
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/osctl/pkg/client"
	"github.com/spf13/cobra"
)

// psCmd represents the processes command
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List processes",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := client.NewDefaultClientCredentials()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c, err := client.NewClient(port, creds)
		if err != nil {
			log.Fatal(err)
		}
		if err := c.Processes(); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(psCmd)
}
