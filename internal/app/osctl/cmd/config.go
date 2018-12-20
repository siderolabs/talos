package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/autonomy/talos/internal/app/osctl/internal/client/config"
	"github.com/spf13/cobra"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration",
	Long:  ``,
}

// configTargetCmd represents the config target command.
var configTargetCmd = &cobra.Command{
	Use:   "target <target>",
	Short: "Set the target for the current context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			if err := cmd.Usage(); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
		target := args[0]
		c, err := config.Open()
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		if c.Context == "" {
			fmt.Println("no context is set")
			os.Exit(1)
		}
		c.Contexts[c.Context].Target = target
		if err := c.Save(); err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
	},
}

// configContextCmd represents the configc context command.
var configContextCmd = &cobra.Command{
	Use:   "context <context>",
	Short: "Set the current context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			if err := cmd.Usage(); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
		context := args[0]
		c, err := config.Open()
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		c.Context = context
		if err := c.Save(); err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
	},
}

// configAddCmd represents the config add command.
var configAddCmd = &cobra.Command{
	Use:   "add <context>",
	Short: "Add a new context",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			if err := cmd.Usage(); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
		context := args[0]
		c, err := config.Open()
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		caBytes, err := ioutil.ReadFile(ca)
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		crtBytes, err := ioutil.ReadFile(crt)
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		keyBytes, err := ioutil.ReadFile(key)
		if err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
		newContext := &config.Context{
			CA:  base64.StdEncoding.EncodeToString(caBytes),
			Crt: base64.StdEncoding.EncodeToString(crtBytes),
			Key: base64.StdEncoding.EncodeToString(keyBytes),
		}
		if c.Contexts == nil {
			c.Contexts = map[string]*config.Context{}
		}
		c.Contexts[context] = newContext
		if err := c.Save(); err != nil {
			fmt.Printf("%v", err)
			os.Exit(1)
		}
	},
}

func init() {
	configAddCmd.Flags().StringVar(&ca, "ca", "", "the path to the CA certificate")
	if err := configAddCmd.MarkFlagRequired("ca"); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
	configAddCmd.Flags().StringVar(&crt, "crt", "", "the path to the certificate")
	if err := configAddCmd.MarkFlagRequired("crt"); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
	configAddCmd.Flags().StringVar(&key, "key", "", "the path to the key")
	if err := configAddCmd.MarkFlagRequired("key"); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
	configCmd.AddCommand(configContextCmd, configTargetCmd, configAddCmd)
	rootCmd.AddCommand(configCmd)
}
