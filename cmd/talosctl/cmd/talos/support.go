// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/gosuri/uiprogress"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/client"
	clusterresource "github.com/talos-systems/talos/pkg/resources/cluster"
)

var supportCmdFlags struct {
	output     string
	numWorkers int
	verbose    bool
}

// supportCmd represents the support command.
var supportCmd = &cobra.Command{
	Use:   "support",
	Short: "Dump debug information about the cluster",
	Long: `Generated bundle contains the following debug information:

- For each node:

	- Kernel logs.
	- All Talos internal services logs.
	- All kube-system pods logs.
	- Talos COSI resources without secrets.
	- COSI runtime state graph.
	- Processes snapshot.
	- IO pressure snapshot.
	- Mounts list.
	- PCI devices info.
	- Talos version.

- For the cluster:

	- Kubernetes nodes and kube-system pods manifests.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(Nodes) == 0 {
			return fmt.Errorf("please provide at least a single node to gather the debug information from")
		}

		if supportCmdFlags.output == "" {
			supportCmdFlags.output = "support"

			if config, err := getDiscoveryConfig(); err == nil && config.TypedSpec().DiscoveryEnabled {
				supportCmdFlags.output += "-" + config.TypedSpec().ServiceClusterID
			}

			supportCmdFlags.output += ".zip"
		}

		if _, err := os.Stat(supportCmdFlags.output); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else {
			buf := bufio.NewReader(os.Stdin)
			fmt.Printf("%s already exists, overwrite? [y/N]: ", supportCmdFlags.output)
			choice, err := buf.ReadString('\n')
			if err != nil {
				return err
			}

			if strings.TrimSpace(strings.ToLower(choice)) != "y" {
				return nil
			}
		}

		f, err := os.OpenFile(supportCmdFlags.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		archive := &cluster.BundleArchive{
			Archive: zip.NewWriter(f),
		}

		var nodesWg sync.WaitGroup

		progress := make(chan cluster.BundleProgress)

		options := []*cluster.BundleOptions{}

		getOpts := func() *cluster.BundleOptions {
			res := &cluster.BundleOptions{
				Archive:    archive,
				NumWorkers: supportCmdFlags.numWorkers,
				Progress:   progress,
			}

			if !supportCmdFlags.verbose {
				res.LogOutput = io.Discard
			}

			options = append(options, res)

			return res
		}

		{
			opts := getOpts()

			nodesWg.Add(1)

			go func() {
				defer nodesWg.Done()

				if e := WithClient(func(ctx context.Context, c *client.Client) error {
					opts.Client = c

					return cluster.GetKubernetesSupportBundle(ctx, opts)
				}); e != nil {
					opts.Error(e)
				}
			}()
		}

		for _, node := range Nodes {
			node := node

			opts := getOpts()

			opts.Source = node

			nodesWg.Add(1)

			go func() {
				defer nodesWg.Done()

				if e := WithClient(func(ctx context.Context, c *client.Client) error {
					opts.Client = c

					return cluster.GetNodeSupportBundle(client.WithNodes(ctx, node), opts)
				}); e != nil {
					opts.Error(e)
				}
			}()

			if err != nil {
				fmt.Printf("failed to gather node %s support bundle %s", node, err)
			}
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()

			if supportCmdFlags.verbose {
				for range progress {
				}
			} else {
				showProgress(progress)
			}
		}()

		nodesWg.Wait()

		close(progress)

		wg.Wait()

		w := tabwriter.NewWriter(os.Stderr, 0, 0, 3, ' ', 0)

		wroteHeader := false

		for _, opt := range options {
			for _, err := range opt.Errors {
				if !wroteHeader {
					wroteHeader = true

					fmt.Println("Processed with errors:")
					fmt.Fprintln(w, "\tSOURCE\tERROR")
				}

				details := strings.Split(err.Error(), "\n")
				for i, d := range details {
					details[i] = strings.TrimSpace(d)
				}

				fmt.Fprintf(w, "\t%s\t%s\n", opt.Source, color.RedString(details[0]))

				if len(details) > 1 {
					for _, line := range details[1:] {
						fmt.Fprintf(w, "\t\t%s\n", color.RedString(line))
					}
				}
			}
		}

		if err = w.Flush(); err != nil {
			return err
		}

		fmt.Printf("Support bundle is written to %s\n", supportCmdFlags.output)

		return archive.Archive.Close()
	},
}

func getDiscoveryConfig() (*clusterresource.Config, error) {
	var config *clusterresource.Config

	if e := WithClient(func(ctx context.Context, c *client.Client) error {
		list, err := c.Resources.Get(ctx, clusterresource.NamespaceName, clusterresource.IdentityType, clusterresource.LocalIdentity)
		if err != nil {
			return err
		}

		resp := list[0]
		b, err := yaml.Marshal(resp.Resource.Spec())
		if err != nil {
			return err
		}

		config = clusterresource.NewConfig(resp.Resource.Metadata().Namespace(), resp.Resource.Metadata().ID())

		return yaml.Unmarshal(b, config.TypedSpec())
	}); e != nil {
		return nil, e
	}

	return config, nil
}

func showProgress(progress <-chan cluster.BundleProgress) {
	func() {
		uiprogress.Start()

		type nodeProgress struct {
			state string
			bar   *uiprogress.Bar
		}

		nodes := map[string]*nodeProgress{}

		for p := range progress {
			var (
				np *nodeProgress
				ok bool
			)

			if np, ok = nodes[p.Source]; !ok {
				bar := uiprogress.AddBar(p.Total)
				bar = bar.AppendCompleted().PrependElapsed()

				src := p.Source

				np = &nodeProgress{
					state: "initializing...",
					bar:   bar,
				}

				bar.AppendFunc(func(b *uiprogress.Bar) string {
					return fmt.Sprintf("%s: %s", src, np.state)
				})

				bar.Width = 20

				nodes[src] = np
			} else {
				np = nodes[p.Source]
			}

			np.state = p.State
			np.bar.Incr()
		}

		uiprogress.Stop()
	}()
}

func init() {
	addCommand(supportCmd)
	supportCmd.Flags().StringVarP(&supportCmdFlags.output, "output", "O", "", "output file to write support archive to")
	supportCmd.Flags().IntVarP(&supportCmdFlags.numWorkers, "num-workers", "w", 1, "number of workers per node")
	supportCmd.Flags().BoolVarP(&supportCmdFlags.verbose, "verbose", "v", false, "verbose output")
}
