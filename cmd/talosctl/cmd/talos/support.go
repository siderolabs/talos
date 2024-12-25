// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/fatih/color"
	"github.com/gosuri/uiprogress"
	"github.com/siderolabs/go-talos-support/support"
	"github.com/siderolabs/go-talos-support/support/bundle"
	"github.com/siderolabs/go-talos-support/support/collectors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/machinery/client"
	clusterresource "github.com/siderolabs/talos/pkg/machinery/resources/cluster"
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
		if len(GlobalArgs.Nodes) == 0 {
			return errors.New("please provide at least a single node to gather the debug information from")
		}

		f, err := openArchive()
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		progress := make(chan bundle.Progress)

		var (
			eg     errgroup.Group
			errors supportBundleErrors
		)

		eg.Go(func() error {
			if supportCmdFlags.verbose {
				for p := range progress {
					errors.handleProgress(p)
				}
			} else {
				showProgress(progress, &errors)
			}

			return nil
		})

		collectErr := collectData(f, progress)

		close(progress)

		if e := eg.Wait(); e != nil {
			return e
		}

		if err = errors.print(); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Support bundle is written to %s\n", supportCmdFlags.output)

		return collectErr
	},
}

func collectData(dest *os.File, progress chan bundle.Progress) error {
	return WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		clientset, err := getKubernetesClient(ctx, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create kubernetes client %s\n", err)
		}

		opts := []bundle.Option{
			bundle.WithArchiveOutput(dest),
			bundle.WithKubernetesClient(clientset),
			bundle.WithTalosClient(c),
			bundle.WithNodes(GlobalArgs.Nodes...),
			bundle.WithNumWorkers(supportCmdFlags.numWorkers),
			bundle.WithProgressChan(progress),
		}

		if !supportCmdFlags.verbose {
			opts = append(opts, bundle.WithLogOutput(io.Discard))
		}

		options := bundle.NewOptions(opts...)

		collectors, err := collectors.GetForOptions(ctx, options)
		if err != nil {
			return err
		}

		return support.CreateSupportBundle(ctx, options, collectors...)
	})
}

func getKubernetesClient(ctx context.Context, c *client.Client) (*k8s.Clientset, error) {
	kubeconfig, err := c.Kubeconfig(ctx)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}

	restconfig, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := k8s.NewForConfig(restconfig)
	if err != nil {
		return nil, err
	}

	// just checking that k8s responds
	_, err = clientset.CoreV1().Namespaces().Get(ctx, "kube-system", v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func getDiscoveryConfig() (*clusterresource.Config, error) {
	var config *clusterresource.Config

	if e := WithClient(func(ctx context.Context, c *client.Client) error {
		var err error

		config, err = safe.StateGet[*clusterresource.Config](
			ctx,
			c.COSI,
			resource.NewMetadata(clusterresource.NamespaceName, clusterresource.IdentityType, clusterresource.LocalIdentity, resource.VersionUndefined),
		)

		return err
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
			return nil, fmt.Errorf("operation aborted")
		}
	}

	return os.OpenFile(supportCmdFlags.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
}

type supportBundleError struct {
	source string
	value  string
}

type supportBundleErrors struct {
	errors []supportBundleError
}

func (sbe *supportBundleErrors) handleProgress(p bundle.Progress) {
	if p.Error != nil {
		sbe.errors = append(sbe.errors, supportBundleError{
			source: p.Source,
			value:  p.Error.Error(),
		})
	}
}

func (sbe *supportBundleErrors) print() error {
	if sbe.errors == nil {
		return nil
	}

	var wroteHeader bool

	w := tabwriter.NewWriter(os.Stderr, 0, 0, 3, ' ', 0)

	for _, err := range sbe.errors {
		if !wroteHeader {
			wroteHeader = true

			fmt.Fprintln(os.Stderr, "Processed with errors:")
			fmt.Fprintln(w, "\tSOURCE\tERROR")
		}

		details := strings.Split(err.value, "\n")
		for i, d := range details {
			details[i] = strings.TrimSpace(d)
		}

		fmt.Fprintf(w, "\t%s\t%s\n", err.source, color.RedString(details[0]))

		if len(details) > 1 {
			for _, line := range details[1:] {
				fmt.Fprintf(w, "\t\t%s\n", color.RedString(line))
			}
		}
	}

	return w.Flush()
}

func showProgress(progress <-chan bundle.Progress, errors *supportBundleErrors) {
	uiprogress.Start()

	type nodeProgress struct {
		mu    sync.Mutex
		state string
		bar   *uiprogress.Bar
	}

	nodes := map[string]*nodeProgress{}

	for p := range progress {
		errors.handleProgress(p)

		var (
			np *nodeProgress
			ok bool
		)

		src := p.Source

		if _, ok = nodes[p.Source]; !ok {
			bar := uiprogress.AddBar(p.Total)
			bar = bar.AppendCompleted().PrependElapsed()

			np = &nodeProgress{
				state: "initializing...",
				bar:   bar,
			}

			bar.AppendFunc(
				func(src string, np *nodeProgress) func(b *uiprogress.Bar) string {
					return func(b *uiprogress.Bar) string {
						np.mu.Lock()
						defer np.mu.Unlock()

						return fmt.Sprintf("%s: %s", src, np.state)
					}
				}(src, np),
			)

			bar.Width = 20

			nodes[src] = np
		} else {
			np = nodes[src]
		}

		np.mu.Lock()
		np.state = p.State
		np.mu.Unlock()

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
