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

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/fatih/color"
	"github.com/gosuri/uiprogress"
	"github.com/siderolabs/go-talos-support/support"
	"github.com/siderolabs/go-talos-support/support/bundle"
	"github.com/siderolabs/go-talos-support/support/bundle/encryption"
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
	output                        string
	numWorkers                    int
	verbose                       bool
	noEncryption                  bool
	encryptionRecipients          []string
	encryptionNoDefaultRecipients bool
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

By default, the generated bundle is encrypted using age encryption to the list of recipients
set by the members of the 'siderolabs' GitHub organization. The encrypted bundle by default will
only be decryptable by the Sidero Labs team, but you can also specify additional recipients using the
--encryption-recipients flag, or disable encryption completely using the --no-encryption flag.
Default encryption recipients can be removed by setting --encryption-no-default-recipients flag.
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(GlobalArgs.Nodes) == 0 {
			return errors.New("please provide at least a single node to gather the debug information from")
		}

		if supportCmdFlags.noEncryption && (len(supportCmdFlags.encryptionRecipients) > 0 || supportCmdFlags.encryptionNoDefaultRecipients) {
			return errors.New("--encryption-recipients and --encryption-no-default-recipients cannot be used with --no-encryption")
		}

		var encryptionOpts []encryption.Option

		if !supportCmdFlags.noEncryption {
			opts, recipients, err := buildEncryptionOptions()
			if err != nil {
				return err
			}

			encryptionOpts = opts

			fmt.Fprintln(os.Stderr, "Encrypting support bundle to the following recipients:")

			for _, r := range recipients {
				fmt.Fprintf(os.Stderr, "  - %s\n", r)
			}
		}

		f, err := openArchive(cmd.Context())
		if err != nil {
			return err
		}

		defer f.Close() //nolint:errcheck

		var (
			archiveOutput io.Writer = f
			encWriter     io.WriteCloser
		)

		if !supportCmdFlags.noEncryption {
			encWriter, err = encryption.Encrypt(f, encryptionOpts...)
			if err != nil {
				return err
			}

			archiveOutput = encWriter
		}

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

		collectErr := collectData(cmd.Context(), archiveOutput, progress)

		close(progress)

		if e := eg.Wait(); e != nil {
			return e
		}

		// flush the age encryption layer before reporting success.
		if encWriter != nil {
			if err = encWriter.Close(); err != nil {
				return err
			}
		}

		if err = errors.print(); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Support bundle is written to %s\n", supportCmdFlags.output)

		return collectErr
	},
}

func collectData(ctx context.Context, dest io.Writer, progress chan bundle.Progress) error {
	return WithClientNoNodes(ctx, func(ctx context.Context, c *client.Client) error {
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

func getClusterInfo(ctx context.Context) (*clusterresource.Info, error) {
	var info *clusterresource.Info

	if e := WithClientNoNodes(ctx, func(ctx context.Context, c *client.Client) error {
		var err error

		info, err = safe.StateGetByID[*clusterresource.Info](
			ctx,
			c.COSI,
			clusterresource.InfoID,
		)

		return err
	}); e != nil {
		return nil, e
	}

	return info, nil
}

// buildEncryptionOptions builds the age encryption options based on the command
// flags and returns the list of recipients (for display) the bundle is encrypted to.
func buildEncryptionOptions() ([]encryption.Option, []string, error) {
	var (
		opts       []encryption.Option
		recipients []string
	)

	if supportCmdFlags.encryptionNoDefaultRecipients {
		if len(supportCmdFlags.encryptionRecipients) == 0 {
			return nil, nil, errors.New("no recipients to encrypt to: --encryption-no-default-recipients is set but no --encryption-recipients provided")
		}

		opts = append(opts, encryption.WithRecipients(supportCmdFlags.encryptionRecipients...))
		recipients = append(recipients, supportCmdFlags.encryptionRecipients...)

		return opts, recipients, nil
	}

	defaults, err := encryption.DefaultRecipients()
	if err != nil {
		return nil, nil, err
	}

	for _, r := range defaults {
		recipients = append(recipients, r.String())
	}

	if len(supportCmdFlags.encryptionRecipients) > 0 {
		opts = append(opts, encryption.WithAdditionalRecipients(supportCmdFlags.encryptionRecipients...))
		recipients = append(recipients, supportCmdFlags.encryptionRecipients...)
	}

	return opts, recipients, nil
}

func openArchive(ctx context.Context) (*os.File, error) {
	if supportCmdFlags.output == "" {
		supportCmdFlags.output = "support"

		if info, err := getClusterInfo(ctx); err == nil && info.TypedSpec().ClusterName != "" {
			supportCmdFlags.output += "-" + info.TypedSpec().ClusterName
		}

		supportCmdFlags.output += ".zip"

		if !supportCmdFlags.noEncryption {
			supportCmdFlags.output += ".age"
		}
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
	supportCmd.Flags().BoolVar(
		&supportCmdFlags.noEncryption, "no-encryption", false,
		"do not encrypt the support bundle (output is written as-is)",
	)
	supportCmd.Flags().StringArrayVar(
		&supportCmdFlags.encryptionRecipients, "encryption-recipients", nil,
		"additional age recipients (SSH or age public keys) to encrypt the support bundle to (can be specified multiple times)",
	)
	supportCmd.Flags().BoolVar(
		&supportCmdFlags.encryptionNoDefaultRecipients, "encryption-no-default-recipients", false,
		"do not encrypt to the default recipients, only to the ones provided via --encryption-recipients",
	)
}
