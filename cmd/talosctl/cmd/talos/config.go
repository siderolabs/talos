// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/cli"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration file (talosconfig)",
	Long:  ``,
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

// configEndpointCmd represents the `config endpoint` command.
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

// configNodeCmd represents the `config node` command.
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

// configContextCmd represents the `config context` command.
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
	ValidArgsFunction: CompleteConfigContext,
}

// configAddCmdFlags represents the `config add` command flags.
var configAddCmdFlags struct {
	ca  string
	crt string
	key string
}

// configAddCmd represents the `config add` command.
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

		caBytes, err := ioutil.ReadFile(configAddCmdFlags.ca)
		if err != nil {
			return fmt.Errorf("error reading CA: %w", err)
		}

		crtBytes, err := ioutil.ReadFile(configAddCmdFlags.crt)
		if err != nil {
			return fmt.Errorf("error reading certificate: %w", err)
		}

		keyBytes, err := ioutil.ReadFile(configAddCmdFlags.key)
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

// configGetContextsCmd represents the `config contexts` command.
var configGetContextsCmd = &cobra.Command{
	Use:     "contexts",
	Short:   "List defined contexts",
	Aliases: []string{"get-contexts"},
	Long:    ``,
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

// configMergeCmd represents the `config merge` command.
var configMergeCmd = &cobra.Command{
	Use:   "merge <from>",
	Short: "Merge additional contexts from another client configuration file",
	Long:  "Contexts with the same name are renamed while merging configs.",
	Args:  cobra.MinimumNArgs(1),
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

// configNewCmdFlags represents the `config new` command flags.
var configNewCmdFlags struct {
	roles  []string
	crtTTL time.Duration
}

// configNewCmd represents the `config new` command.
var configNewCmd = &cobra.Command{
	Use:   "new [<path>]",
	Short: "Generate a new client configuration file",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			args = []string{"talosconfig"}
		}

		path := args[0]

		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "talosconfig"); err != nil {
				return err
			}

			roles, unknownRoles := role.Parse(configNewCmdFlags.roles)
			if len(unknownRoles) != 0 {
				return fmt.Errorf("unknown roles: %s", strings.Join(unknownRoles, ", "))
			}

			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("talosconfig file already exists: %q", path)
			}

			resp, err := c.GenerateClientConfiguration(ctx, &machineapi.GenerateClientConfigurationRequest{
				Roles:  roles.Strings(),
				CrtTtl: durationpb.New(configNewCmdFlags.crtTTL),
			})
			if err != nil {
				return err
			}

			if l := len(resp.Messages); l != 1 {
				panic(fmt.Sprintf("expected 1 message, got %d", l))
			}

			config, err := clientconfig.FromBytes(resp.Messages[0].Talosconfig)
			if err != nil {
				return err
			}

			// make the new config immediately useful
			config.Contexts[config.Context].Endpoints = c.GetEndpoints()

			return config.Save(path)
		})
	},
}

// configNewCmd represents the `config info` command output template.
var configInfoCmdTemplate = template.Must(template.New("configInfoCmdTemplate").Option("missingkey=error").Parse(strings.TrimSpace(`
Current context:     {{ .Context }}
Nodes:               {{ .Nodes }}
Endpoints:           {{ .Endpoints }}
Roles:               {{ .Roles }}
Certificate expires: {{ .CertTTL }} ({{ .CertNotAfter }})
`)))

// configInfoCommand implements `config info` command logic.
//
//nolint:goconst
func configInfoCommand(config *clientconfig.Config, now time.Time) (string, error) {
	context := config.Contexts[config.Context]

	b, err := base64.StdEncoding.DecodeString(context.Crt)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return "", fmt.Errorf("error decoding PEM")
	}

	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}

	roles, _ := role.Parse(crt.Subject.Organization)

	nodesS := "not defined"
	if len(context.Nodes) > 0 {
		nodesS = strings.Join(context.Nodes, ", ")
	}

	endpointsS := "not defined"
	if len(context.Endpoints) > 0 {
		endpointsS = strings.Join(context.Endpoints, ", ")
	}

	rolesS := "not defined"
	if s := roles.Strings(); len(s) > 0 {
		rolesS = strings.Join(s, ", ")
	}

	var res bytes.Buffer
	err = configInfoCmdTemplate.Execute(&res, map[string]string{
		"Context":      config.Context,
		"Nodes":        nodesS,
		"Endpoints":    endpointsS,
		"Roles":        rolesS,
		"CertTTL":      humanize.RelTime(crt.NotAfter, now, "ago", "from now"),
		"CertNotAfter": crt.NotAfter.UTC().Format("2006-01-02"),
	})

	return res.String() + "\n", err
}

// configInfoCmd represents the `config info` command.
var configInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show information about the current context",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := openConfigAndContext("")
		if err != nil {
			return err
		}

		res, err := configInfoCommand(c, time.Now())
		if err != nil {
			return err
		}

		fmt.Print(res)

		return nil
	},
}

// CompleteConfigContext represents tab completion for `--context` argument and `config context` command.
func CompleteConfigContext(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c, err := clientconfig.Open(Talosconfig)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	contextnames := make([]string, 0, len(c.Contexts))
	for contextname := range c.Contexts {
		contextnames = append(contextnames, contextname)
	}

	sort.Strings(contextnames)

	return contextnames, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	configCmd.AddCommand(
		configEndpointCmd,
		configNodeCmd,
		configContextCmd,
		configAddCmd,
		configGetContextsCmd,
		configMergeCmd,
		configNewCmd,
		configInfoCmd,
	)

	configAddCmd.Flags().StringVar(&configAddCmdFlags.ca, "ca", "", "the path to the CA certificate")
	configAddCmd.Flags().StringVar(&configAddCmdFlags.crt, "crt", "", "the path to the certificate")
	configAddCmd.Flags().StringVar(&configAddCmdFlags.key, "key", "", "the path to the key")
	cli.Should(configAddCmd.MarkFlagRequired("ca"))
	cli.Should(configAddCmd.MarkFlagRequired("crt"))
	cli.Should(configAddCmd.MarkFlagRequired("key"))

	configNewCmd.Flags().StringSliceVar(&configNewCmdFlags.roles, "roles", role.MakeSet(role.Admin).Strings(), "roles")
	configNewCmd.Flags().DurationVar(&configNewCmdFlags.crtTTL, "crt-ttl", 87600*time.Hour, "certificate TTL")

	addCommand(configCmd)
}
