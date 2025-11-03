// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/cgroupsprinter"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/pkg/cgroups"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var cgroupsCmdFlags struct {
	schemaFile     string
	presetName     string
	skipCRIResolve bool
}

// cgroupsCmd represents the cgroups command.
var cgroupsCmd = &cobra.Command{
	Use:     "cgroups",
	Aliases: []string{"cg"},
	Short:   "Retrieve cgroups usage information",
	Long: `The cgroups command fetches control group v2 (cgroupv2) usage details from the machine.
Several presets are available to focus on specific cgroup subsystems:

* cpu
* cpuset
* io
* memory
* process
* swap

You can specify the preset using the --preset flag.

Alternatively, a custom schema can be provided using the --schema-file flag.
To see schema examples, refer to https://github.com/siderolabs/talos/tree/main/cmd/talosctl/cmd/talos/cgroupsprinter/schemas.
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "cgroups"); err != nil {
				return err
			}

			var schema cgroupsprinter.Schema

			switch {
			case cgroupsCmdFlags.schemaFile != "":
				in, err := os.Open(cgroupsCmdFlags.schemaFile)
				if err != nil {
					return fmt.Errorf("error opening schema file: %w", err)
				}

				defer in.Close() //nolint:errcheck

				if err = yaml.NewDecoder(in).Decode(&schema); err != nil {
					return fmt.Errorf("error decoding schema file: %w", err)
				}
			case cgroupsCmdFlags.presetName != "":
				presetNames := cgroupsprinter.GetPresetNames()

				if slices.Index(presetNames, cgroupsCmdFlags.presetName) == -1 {
					return fmt.Errorf("invalid preset name: %s (valid %v)", cgroupsCmdFlags.presetName, presetNames)
				}

				schema = cgroupsprinter.GetPreset(cgroupsCmdFlags.presetName)
			default:
				return fmt.Errorf("either schema file or preset must be specified")
			}

			if err := schema.Compile(); err != nil {
				return fmt.Errorf("error compiling schema: %w", err)
			}

			processResolveMap := buildProcessResolveMap(ctx, c)
			devicesResolveMap := buildDevicesResolveMap(ctx, c)

			r, err := c.Copy(ctx, constants.CgroupMountPath)
			if err != nil {
				return fmt.Errorf("error copying: %w", err)
			}

			defer r.Close() //nolint:errcheck

			tree, err := cgroups.TreeFromTarGz(r)
			if err != nil {
				return fmt.Errorf("error reading cgroups: %w", err)
			}

			if !cgroupsCmdFlags.skipCRIResolve {
				cgroupNameResolveMap := buildCgroupResolveMap(ctx, c)
				tree.ResolveNames(cgroupNameResolveMap)
			}

			tree.Walk(func(node *cgroups.Node) {
				node.CgroupProcsResolved = xslices.Map(node.CgroupProcs, func(pid cgroups.Value) cgroups.RawValue {
					if name, ok := processResolveMap[pid.String()]; ok {
						return cgroups.RawValue(name)
					}

					return cgroups.RawValue(pid.String())
				})

				for dev := range node.IOStat {
					if name, ok := devicesResolveMap[dev]; ok {
						node.IOStat[name] = node.IOStat[dev]
						delete(node.IOStat, dev)
					}
				}
			})

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			defer w.Flush() //nolint:errcheck

			headerLine := "NAME\t" + schema.HeaderLine() + "\n"

			_, err = w.Write([]byte(headerLine))
			if err != nil {
				return fmt.Errorf("error writing header line: %w", err)
			}

			return cgroupsprinter.PrintNode(".", w, &schema, tree.Root, nil, 0, nil, false, true)
		})
	},
}

func completeCgroupPresetArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return cgroupsprinter.GetPresetNames(), cobra.ShellCompDirectiveNoFileComp
}

func buildCgroupResolveMap(ctx context.Context, c *client.Client) map[string]string {
	cgroupNameResolveMap := map[string]string{}

	containersResp, err := c.Containers(ctx, constants.K8sContainerdNamespace, common.ContainerDriver_CRI)
	if err != nil {
		cli.Warning("error getting containers: %s", err)
	} else {
		for _, ctr := range containersResp.Messages[0].Containers {
			if ctr.Uid != "" && ctr.PodId != "" {
				cgroupNameResolveMap["pod"+ctr.Uid] = ctr.PodId
			}

			if ctr.InternalId != "" {
				if ctr.PodId == ctr.Name {
					cgroupNameResolveMap[ctr.InternalId] = "sandbox"
				} else {
					cgroupNameResolveMap[ctr.InternalId] = ctr.Name
				}
			}
		}
	}

	return cgroupNameResolveMap
}

func buildProcessResolveMap(ctx context.Context, c *client.Client) map[string]string {
	processResolveMap := map[string]string{}

	processesResp, err := c.Processes(ctx)
	if err != nil {
		cli.Warning("error getting processes: %s", err)

		return processResolveMap
	}

	for _, proc := range processesResp.Messages[0].Processes {
		name := proc.Executable

		if name == "" {
			name = proc.Command
		}

		if name == "" {
			args := strings.Fields(proc.Args)

			if len(args) > 0 {
				name = args[0]
			}
		}

		name = filepath.Base(name)

		processResolveMap[strconv.FormatInt(int64(proc.Pid), 10)] = name
	}

	return processResolveMap
}

func buildDevicesResolveMap(ctx context.Context, c *client.Client) map[string]string {
	devicesResolveMap := map[string]string{}

	r, err := c.Copy(ctx, "/sys/dev/block")
	if err != nil {
		cli.Warning("error copying devices: %s", err)

		return devicesResolveMap
	}

	defer r.Close() //nolint:errcheck

	gzR, err := gzip.NewReader(r)
	if err != nil {
		cli.Warning("error reading devices: %s", err)

		return devicesResolveMap
	}

	defer gzR.Close() //nolint:errcheck

	tarR := tar.NewReader(gzR)

	for {
		header, err := tarR.Next()
		if err != nil {
			break
		}

		if header.Typeflag != tar.TypeSymlink {
			continue
		}

		devicesResolveMap[header.Name] = filepath.Base(header.Linkname)
	}

	return devicesResolveMap
}

func init() {
	presetNames := cgroupsprinter.GetPresetNames()

	cgroupsCmd.Flags().StringVar(&cgroupsCmdFlags.schemaFile, "schema-file", "", "path to the columns schema file")
	cgroupsCmd.Flags().StringVar(&cgroupsCmdFlags.presetName, "preset", "", fmt.Sprintf("preset name (one of: %v)", presetNames))
	cgroupsCmd.Flags().BoolVar(&cgroupsCmdFlags.skipCRIResolve, "skip-cri-resolve", false, "do not resolve cgroup names via a request to CRI")
	cgroupsCmd.MarkFlagsMutuallyExclusive("schema-file", "preset")
	cgroupsCmd.RegisterFlagCompletionFunc("preset", completeCgroupPresetArg) //nolint:errcheck

	addCommand(cgroupsCmd)
}
