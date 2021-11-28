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
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/client"
	clusterresource "github.com/talos-systems/talos/pkg/machinery/resources/cluster"
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
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(Nodes) == 0 {
			return fmt.Errorf("please provide at least a single node to gather the debug information from")
		}

		f, err := openArchive()
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		archive := &cluster.BundleArchive{
			Archive: zip.NewWriter(f),
		}

		progress := make(chan cluster.BundleProgress)

		var eg errgroup.Group

		eg.Go(func() error {
			if supportCmdFlags.verbose {
				for range progress {
				}
			} else {
				showProgress(progress)
			}

			return nil
		})

		err = collectData(archive, progress)

		close(progress)

		if e := eg.Wait(); e != nil {
			return e
		}

		if err != nil {
			if err = printErrors(err); err != nil {
				return err
			}
		}

		fmt.Printf("Support bundle is written to %s\n", supportCmdFlags.output)

		return archive.Archive.Close()
	},
}

func collectData(archive *cluster.BundleArchive, progress chan cluster.BundleProgress) error {
	return WithClient(func(ctx context.Context, c *client.Client) error {
		sources := append([]string{}, Nodes...)
		sources = append(sources, "cluster")

		var (
			errsMu sync.Mutex
			errs   error
		)

		var eg errgroup.Group

		for _, source := range sources {
			opts := &cluster.BundleOptions{
				Archive:    archive,
				NumWorkers: supportCmdFlags.numWorkers,
				Progress:   progress,
				Source:     source,
				Client:     c,
			}

			if !supportCmdFlags.verbose {
				opts.LogOutput = io.Discard
			}

			source := source

			eg.Go(func() error {
				var err error

				if source == "cluster" {
					err = cluster.GetKubernetesSupportBundle(ctx, opts)
				} else {
					err = cluster.GetNodeSupportBundle(client.WithNodes(ctx, source), opts)
				}

				if err == nil {
					return nil
				}

				errsMu.Lock()
				defer errsMu.Unlock()

				errs = multierror.Append(errs, err)

				return err
			})
		}

		// errors are gathered separately as eg.Wait returns only a single error
		// while we want to gather all of them
		eg.Wait() //nolint:errcheck

		return errs
	})
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

func openArchive() (*os.File, error) {
	if supportCmdFlags.output == "" {
		supportCmdFlags.output = "support"

		if config, err := getDiscoveryConfig(); err == nil && config.TypedSpec().DiscoveryEnabled {
			supportCmdFlags.output += "-" + config.TypedSpec().ServiceClusterID
		}

		supportCmdFlags.output += ".zip"
	}

	if _, err := os.Stat(supportCmdFlags.output); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		buf := bufio.NewReader(os.Stdin)
		fmt.Printf("%s already exists, overwrite? [y/N]: ", supportCmdFlags.output)
		choice, err := buf.ReadString('\n')
		if err != nil {
			return nil, err
		}

		if strings.TrimSpace(strings.ToLower(choice)) != "y" {
			return nil, nil
		}
	}

	return os.OpenFile(supportCmdFlags.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
}

func printErrors(err error) error {
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 3, ' ', 0)

	wroteHeader := false

	var errs *multierror.Error

	if !errors.As(err, &errs) {
		fmt.Printf("Processed with errors:\n%s\n", color.RedString(err.Error()))

		return nil
	}

	for _, err := range errs.Errors {
		if !wroteHeader {
			wroteHeader = true

			fmt.Println("Processed with errors:")
			fmt.Fprintln(w, "\tSOURCE\tERROR")
		}

		var (
			bundleErr *cluster.BundleError
			source    string
		)

		if errors.As(err, &bundleErr) {
			source = bundleErr.Source
		}

		details := strings.Split(err.Error(), "\n")
		for i, d := range details {
			details[i] = strings.TrimSpace(d)
		}

		fmt.Fprintf(w, "\t%s\t%s\n", source, color.RedString(details[0]))

		if len(details) > 1 {
			for _, line := range details[1:] {
				fmt.Fprintf(w, "\t\t%s\n", color.RedString(line))
			}
		}
	}

	return w.Flush()
}

func showProgress(progress <-chan cluster.BundleProgress) {
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
}

func init() {
	addCommand(supportCmd)
	supportCmd.Flags().StringVarP(&supportCmdFlags.output, "output", "O", "", "output file to write support archive to")
	supportCmd.Flags().IntVarP(&supportCmdFlags.numWorkers, "num-workers", "w", 1, "number of workers per node")
	supportCmd.Flags().BoolVarP(&supportCmdFlags.verbose, "verbose", "v", false, "verbose output")
}
