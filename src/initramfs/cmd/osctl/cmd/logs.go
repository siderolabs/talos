package cmd

import (
	"fmt"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/osctl/pkg/client"
	"github.com/autonomy/dianemo/src/initramfs/cmd/osd/proto"
	"github.com/spf13/cobra"
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs <id>",
	Short: "Retrieve logs for a process or container",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			if err := cmd.Usage(); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
		process := args[0]
		creds, err := client.NewDefaultClientCredentials()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		c, err := client.NewClient(port, creds)
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}
		r := &proto.LogsRequest{
			Process:   process,
			Container: isContainer,
		}
		if err := c.Logs(r); err != nil {
			fmt.Print(err)
			os.Exit(1)
		}
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&isContainer, "container", "c", false, "treat <id> as a container ID")
	rootCmd.AddCommand(logsCmd)
}
