package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/osctl/pkg/client"
	"github.com/autonomy/dianemo/src/initramfs/cmd/osd/proto"
	"github.com/spf13/cobra"
)

var (
	timeout int32
)

// restartCmd represents the processes command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart a process",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			if err := cmd.Usage(); err != nil {
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
			log.Fatal(err)
		}
		r := &proto.RestartRequest{
			Id:      args[0],
			Timeout: timeout,
		}
		if err := c.Restart(r); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	restartCmd.Flags().Int32VarP(&timeout, "timeout", "t", 60, "the timeout duration in seconds")
	rootCmd.AddCommand(restartCmd)
}
