// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
)

var (
	ca  string
	crt string
	key string
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration",
	Long:  ``,
}

// configEndpointCmd represents the config endpoint command.
var configEndpointCmd = &cobra.Command{
	Use:     "endpoint <endpoint>...",
	Aliases: []string{"endpoints"},
	Short:   "Set the endpoint(s) for the current context",
	Long:    ``,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := openConfigAndContext("")
		if err != nil {
			return err
		}

		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}

		c.Contexts[c.Context].Endpoints = args
		if err := c.Save(Talosconfig); err != nil {
			return fmt.Errorf("error writing config: %w", err)
		}

		return nil
	},
}

// configNodeCmd represents the config node command.
var configNodeCmd = &cobra.Command{
	Use:     "node <endpoint>...",
	Aliases: []string{"nodes"},
	Short:   "Set the node(s) for the current context",
	Long:    ``,
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := openConfigAndContext("")
		if err != nil {
			return err
		}

		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}

		c.Contexts[c.Context].Nodes = args
		if err := c.Save(Talosconfig); err != nil {
			return fmt.Errorf("error writing config: %w", err)
		}

		return nil
	},
}

// configContextCmd represents the config context command.
var configContextCmd = &cobra.Command{
	Use:     "context <context>",
	Short:   "Set the current context",
	Aliases: []string{"use-context"},
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		context := args[0]

		c, err := openConfigAndContext(context)
		if err != nil {
			return err
		}

		c.Context = context

		if err := c.Save(Talosconfig); err != nil {
			return fmt.Errorf("error writing config: %s", err)
		}

		return nil
	},
}

// configAddCmd represents the config add command.
var configAddCmd = &cobra.Command{
	Use:   "add <context>",
	Short: "Add a new context",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		context := args[0]
		c, err := clientconfig.Open(Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		caBytes, err := ioutil.ReadFile(ca)
		if err != nil {
			return fmt.Errorf("error reading CA: %w", err)
		}

		crtBytes, err := ioutil.ReadFile(crt)
		if err != nil {
			return fmt.Errorf("error reading certificate: %w", err)
		}

		keyBytes, err := ioutil.ReadFile(key)
		if err != nil {
			return fmt.Errorf("error reading key: %w", err)
		}

		newContext := &clientconfig.Context{
			CA:  base64.StdEncoding.EncodeToString(caBytes),
			Crt: base64.StdEncoding.EncodeToString(crtBytes),
			Key: base64.StdEncoding.EncodeToString(keyBytes),
		}

		if c.Contexts == nil {
			c.Contexts = map[string]*clientconfig.Context{}
		}

		c.Contexts[context] = newContext
		if err := c.Save(Talosconfig); err != nil {
			return fmt.Errorf("error writing config: %w", err)
		}

		return nil
	},
}

// configGenerateCmd represents the config generate stub command.
var configGenerateCmd = &cobra.Command{
	Use:    "generate",
	Short:  "Generate Talos config",
	Long:   ``,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("'talosctl config generate' was renamed to 'talosctl gen config'")
	},
}

func openConfigAndContext(context string) (*clientconfig.Config, error) {
	c, err := clientconfig.Open(Talosconfig)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	if context == "" {
		context = c.Context
	}

	if context == "" {
		return nil, fmt.Errorf("no context is set")
	}

	if _, ok := c.Contexts[context]; !ok {
		return nil, fmt.Errorf("context %q is not defined", context)
	}

	return c, nil
}

// configGetContexts represents config contexts command.
var configGetContexts = &cobra.Command{
	Use:     "contexts",
	Short:   "List contexts defined in Talos config",
	Aliases: []string{"get-contexts"},
	Long:    ``,
	Hidden:  false,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := clientconfig.Open(Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		keys := make([]string, len(c.Contexts))
		i := 0
		for key := range c.Contexts {
			keys[i] = key
			i++
		}
		sort.Strings(keys)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "CURRENT\tNAME\tENDPOINTS\tNODES")
		for _, name := range keys {
			context := c.Contexts[name]

			var (
				current   string
				endpoints string
				nodes     string
			)

			if name == c.Context {
				current = "*"
			}

			endpoints = strings.Join(context.Endpoints, ",")
			if len(context.Nodes) > 3 {
				nodes = strings.Join(context.Nodes[:3], ",")
				nodes += "..."
			} else {
				nodes = strings.Join(context.Nodes, ",")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", current, name, endpoints, nodes)
		}

		return w.Flush()
	},
}

// configMergeCmd represents the config merge command.
var configMergeCmd = &cobra.Command{
	Use:    "merge <from>",
	Short:  "Merge additional contexts from another Talos config into the default config",
	Long:   "Contexts with the same name are renamed while merging configs.",
	Hidden: false,
	Args:   cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		from := args[0]
		c, err := clientconfig.Open(Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		secondConfig, err := clientconfig.Open(from)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		renames := c.Merge(secondConfig)
		for _, rename := range renames {
			fmt.Printf("renamed talosconfig context %s\n", rename.String())
		}

		if err := c.Save(Talosconfig); err != nil {
			return fmt.Errorf("error writing config: %s", err)
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configContextCmd, configEndpointCmd, configNodeCmd, configAddCmd, configGenerateCmd, configMergeCmd, configGetContexts)
	configAddCmd.Flags().StringVar(&ca, "ca", "", "the path to the CA certificate")
	configAddCmd.Flags().StringVar(&crt, "crt", "", "the path to the certificate")
	configAddCmd.Flags().StringVar(&key, "key", "", "the path to the key")
	cli.Should(configAddCmd.MarkFlagRequired("ca"))
	cli.Should(configAddCmd.MarkFlagRequired("crt"))
	cli.Should(configAddCmd.MarkFlagRequired("key"))
	addCommand(configCmd)
}
