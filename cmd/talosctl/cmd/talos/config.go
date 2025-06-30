// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ryanuber/go-glob"
	"github.com/siderolabs/gen/maps"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the client configuration file (talosconfig)",
	Long:  ``,
}

func openConfigAndContext(context string) (*clientconfig.Config, error) {
	c, err := clientconfig.Open(GlobalArgs.Talosconfig)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	if context == "" {
		context = c.Context
	}

	if context == "" {
		return nil, errors.New("no context is set")
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

		ctxData, err := getContextData(c)
		if err != nil {
			return err
		}

		ctxData.Endpoints = args
		if err := c.Save(GlobalArgs.Talosconfig); err != nil {
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

		ctxData, err := getContextData(c)
		if err != nil {
			return err
		}

		ctxData.Nodes = args
		if err := c.Save(GlobalArgs.Talosconfig); err != nil {
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

		if err := c.Save(GlobalArgs.Talosconfig); err != nil {
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
		c, err := clientconfig.Open(GlobalArgs.Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		newContext := &clientconfig.Context{}

		if configAddCmdFlags.ca != "" {
			var caBytes []byte
			caBytes, err = os.ReadFile(configAddCmdFlags.ca)
			if err != nil {
				return fmt.Errorf("error reading CA: %w", err)
			}

			newContext.CA = base64.StdEncoding.EncodeToString(caBytes)
		}

		err = checkAndSetCrtAndKey(newContext)
		if err != nil {
			return err
		}

		if c.Contexts == nil {
			c.Contexts = map[string]*clientconfig.Context{}
		}

		c.Contexts[context] = newContext
		if err := c.Save(GlobalArgs.Talosconfig); err != nil {
			return fmt.Errorf("error writing config: %w", err)
		}

		return nil
	},
}

// configRemoveCmdFlags represents the `config remove` command flags.
var configRemoveCmdFlags struct {
	noconfirm bool
	dry       bool
}

// configRemoveCmd represents the `config remove` command.
var configRemoveCmd = &cobra.Command{
	Use:   "remove <context>",
	Short: "Remove contexts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		if pattern == "" {
			return errors.New("no context specified")
		}

		c, err := clientconfig.Open(GlobalArgs.Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		if len(c.Contexts) == 0 {
			return errors.New("no contexts defined")
		}

		matches := sortInPlace(maps.Keys(
			maps.Filter(c.Contexts, func(context string, _ *clientconfig.Context) bool {
				return glob.Glob(pattern, context)
			}),
		))
		if len(matches) == 0 {
			return fmt.Errorf("no contexts matched %q", pattern)
		}

		// we want to prevent file updates in case there were no changes
		noChanges := true

		for _, match := range matches {
			if match == c.Context {
				fmt.Fprintf(
					os.Stderr,
					"skipping removal of current context %q, please change it to another before removing\n",
					match,
				)

				continue
			}

			if !configRemoveCmdFlags.noconfirm {
				prompt := fmt.Sprintf("remove context %q", match)

				if !helpers.Confirm(prompt + "?") {
					continue
				}
			} else {
				fmt.Fprintf(os.Stderr, "removing context %q\n", match)
			}

			noChanges = false
			delete(c.Contexts, match)
		}

		if configRemoveCmdFlags.dry || noChanges {
			return nil
		}

		err = c.Save(GlobalArgs.Talosconfig)
		if err != nil {
			return fmt.Errorf("error writing config: %w", err)
		}

		return nil
	},
	ValidArgsFunction: CompleteConfigContext,
}

func sortInPlace(slc []string) []string {
	slices.Sort(slc)

	return slc
}

func checkAndSetCrtAndKey(configContext *clientconfig.Context) error {
	crt := configAddCmdFlags.crt
	key := configAddCmdFlags.key

	if crt == "" && key == "" {
		return nil
	}

	if crt == "" || key == "" {
		return errors.New("if either the 'crt' or 'key' flag is specified, both are required")
	}

	crtBytes, err := os.ReadFile(crt)
	if err != nil {
		return fmt.Errorf("error reading certificate: %w", err)
	}

	configContext.Crt = base64.StdEncoding.EncodeToString(crtBytes)

	keyBytes, err := os.ReadFile(key)
	if err != nil {
		return fmt.Errorf("error reading key: %w", err)
	}

	configContext.Key = base64.StdEncoding.EncodeToString(keyBytes)

	return nil
}

// configGetContextsCmd represents the `config contexts` command.
var configGetContextsCmd = &cobra.Command{
	Use:     "contexts",
	Short:   "List defined contexts",
	Aliases: []string{"get-contexts"},
	Long:    ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := clientconfig.Open(GlobalArgs.Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		keys := maps.Keys(c.Contexts)
		slices.Sort(keys)

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
		c, err := clientconfig.Open(GlobalArgs.Talosconfig)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		secondConfig, err := clientconfig.Open(from)
		if err != nil {
			return fmt.Errorf("error reading config: %w", err)
		}

		renames := c.Merge(secondConfig)
		for _, rename := range renames {
			fmt.Fprintf(os.Stderr, "renamed talosconfig context %s\n", rename.String())
		}

		if err := c.Save(GlobalArgs.Talosconfig); err != nil {
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
var configInfoCmdTemplate = template.Must(template.New("configInfoCmdTemplate").
	Funcs(template.FuncMap{"join": strings.Join}).
	Option("missingkey=error").
	Parse(strings.TrimSpace(`
Current context:     {{ .Context }}
Nodes:               {{ if .Nodes }}{{ join .Nodes ", " }}{{ else }}not defined{{ end }}
Endpoints:           {{ if .Endpoints }}{{ join .Endpoints ", " }}{{ else }}not defined{{ end }}
{{- if .Roles }}
Roles:               {{ join .Roles ", " }}{{ end }}
{{- if .CertTTL }}
Certificate expires: {{ .CertTTL }} ({{ .CertNotAfter }}){{ end }}
`)))

type talosconfigInfo struct {
	Context      string   `json:"context" yaml:"context"`
	Nodes        []string `json:"nodes" yaml:"nodes"`
	Endpoints    []string `json:"endpoints" yaml:"endpoints"`
	Roles        []string `json:"roles" yaml:"roles"`
	CertTTL      string   `json:"certTTL" yaml:"certTTL"`
	CertNotAfter string   `json:"certNotAfter" yaml:"certNotAfter"`
}

// configInfo returns talosct config info.
func configInfo(config *clientconfig.Config, now time.Time) (talosconfigInfo, error) {
	cfgContext, err := getContextData(config)
	if err != nil {
		return talosconfigInfo{}, err
	}

	var (
		certTTL, certNotAfter string
		roles                 role.Set
	)

	if cfgContext.Crt != "" {
		var b []byte

		b, err = base64.StdEncoding.DecodeString(cfgContext.Crt)
		if err != nil {
			return talosconfigInfo{}, err
		}

		block, _ := pem.Decode(b)
		if block == nil {
			return talosconfigInfo{}, errors.New("error decoding PEM")
		}

		var crt *x509.Certificate

		crt, err = x509.ParseCertificate(block.Bytes)
		if err != nil {
			return talosconfigInfo{}, err
		}

		roles, _ = role.Parse(crt.Subject.Organization)

		certTTL = humanize.RelTime(crt.NotAfter, now, "ago", "from now")
		certNotAfter = crt.NotAfter.UTC().Format("2006-01-02")
	}

	return talosconfigInfo{
		Context:      config.Context,
		Nodes:        cfgContext.Nodes,
		Endpoints:    cfgContext.Endpoints,
		Roles:        roles.Strings(),
		CertTTL:      certTTL,
		CertNotAfter: certNotAfter,
	}, nil
}

// configInfoCommand implements `config info` command logic.
func configInfoCommand(config *clientconfig.Config, now time.Time) (string, error) {
	info, err := configInfo(config, now)
	if err != nil {
		return "", err
	}

	var res bytes.Buffer

	err = configInfoCmdTemplate.Execute(&res, info)

	return res.String() + "\n", err
}

var configInfoCmdFlags struct {
	output string
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

		switch configInfoCmdFlags.output {
		case "text":
			res, err := configInfoCommand(c, time.Now())
			if err != nil {
				return err
			}

			fmt.Print(res)

			return nil
		case "json":
			info, err := configInfo(c, time.Now())
			if err != nil {
				return err
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			return enc.Encode(&info)
		case "yaml":
			info, err := configInfo(c, time.Now())
			if err != nil {
				return err
			}

			return yaml.NewEncoder(os.Stdout).Encode(&info)
		default:
			return fmt.Errorf("unknown output format: %q", configInfoCmdFlags.output)
		}
	},
}

// CompleteConfigContext represents tab completion for `--context`
// argument and `config [context|remove]` command.
func CompleteConfigContext(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	c, err := clientconfig.Open(GlobalArgs.Talosconfig)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	contextnames := maps.Keys(c.Contexts)
	slices.Sort(contextnames)

	return contextnames, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	configCmd.AddCommand(
		configEndpointCmd,
		configNodeCmd,
		configContextCmd,
		configAddCmd,
		configRemoveCmd,
		configGetContextsCmd,
		configMergeCmd,
		configNewCmd,
		configInfoCmd,
	)

	configAddCmd.Flags().StringVar(&configAddCmdFlags.ca, "ca", "", "the path to the CA certificate")
	configAddCmd.Flags().StringVar(&configAddCmdFlags.crt, "crt", "", "the path to the certificate")
	configAddCmd.Flags().StringVar(&configAddCmdFlags.key, "key", "", "the path to the key")

	configRemoveCmd.Flags().BoolVarP(
		&configRemoveCmdFlags.noconfirm, "noconfirm", "y", false,
		"do not ask for confirmation",
	)
	configRemoveCmd.Flags().BoolVar(
		&configRemoveCmdFlags.dry, "dry-run", false, "dry run",
	)

	configNewCmd.Flags().StringSliceVar(&configNewCmdFlags.roles, "roles", role.MakeSet(role.Admin).Strings(), "roles")
	configNewCmd.Flags().DurationVar(&configNewCmdFlags.crtTTL, "crt-ttl", constants.TalosAPIDefaultCertificateValidityDuration, "certificate TTL")

	configInfoCmd.Flags().StringVarP(&configInfoCmdFlags.output, "output", "o", "text", "output format (json|yaml|text). Default text.")

	addCommand(configCmd)
}

func getContextData(c *clientconfig.Config) (*clientconfig.Context, error) {
	contextName := c.Context

	if GlobalArgs.CmdContext != "" {
		contextName = GlobalArgs.CmdContext
	}

	ctxData, ok := c.Contexts[contextName]
	if !ok {
		return nil, fmt.Errorf("context %q is not defined", contextName)
	}

	return ctxData, nil
}
