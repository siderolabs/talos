package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	address      string
	ca           string
	crt          string
	key          string
	isContainer  bool
	organization string
	rsa          bool
	name         string
	csr          string
	ip           string
	port         int
	hours        int
	identity     string
	hash         string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "osctl",
	Short: "A CLI for out-of-band management of Kubernetes nodes created by Dianmeo",
	Long:  ``,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&address, "address", "192.168.1.200", "the address of the node")
	rootCmd.PersistentFlags().IntVar(&port, "port", 50000, "the port of the gRPC server")
}
