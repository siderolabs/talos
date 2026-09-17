// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var contentLibraryUploadCmdFlags struct {
	name      string
	overwrite bool
}

// hypervisorCmd represents the hypervisor command.
var hypervisorCmd = &cobra.Command{
	Use:     "hypervisor",
	Aliases: []string{"hv"},
	Short:   "Manage the Talos hypervisor",
	Long:    ``,
	Args:    cobra.NoArgs,
}

// contentLibraryCmd represents the hypervisor content-library command.
var contentLibraryCmd = &cobra.Command{
	Use:     "content-library",
	Aliases: []string{"content-libraries"},
	Short:   "Manage the contents of content libraries",
	Long: `Content libraries store virtual machine images. They are declared with the
ContentLibraryConfig document; this command manages what is stored in them.`,
	Args: cobra.NoArgs,
}

// contentLibraryListCmd represents the hypervisor content-library list command.
var contentLibraryListCmd = &cobra.Command{
	Use:     "list <library_id>",
	Aliases: []string{"l", "ls"},
	Short:   "List the files stored in a content library",
	Long:    ``,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return contentLibraryList(cmd.Context(), args[0])
	},
}

func contentLibraryList(ctx context.Context, libraryID string) error {
	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	responseChan := multiplex.StreamingViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (grpc.ServerStreamingClient[machine.ContentLibraryServiceListResponse], error) {
			return c.ContentLibraryClient.List(ctx, &machine.ContentLibraryServiceListRequest{
				LibraryId: libraryID,
			})
		},
	)

	w := tabwriter.NewWriter(safeout.Stdout(), 0, 0, 3, ' ', 0)
	headerWritten := false

	writeHeader := func() {
		if headerWritten {
			return
		}

		headerWritten = true

		fmt.Fprintln(w, "NODE\tNAME\tSIZE\tMODIFIED")
	}

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		writeHeader()

		safeout.Fprintf(
			w, "%s\t%s\t%s\t%s\n",
			resp.Node,
			resp.Payload.GetName(),
			humanize.Bytes(resp.Payload.GetSize()),
			resp.Payload.GetModifiedAt().AsTime().Format(time.RFC3339),
		)
	}

	// An existing library with no files still deserves a header: empty output would be
	// indistinguishable from the command doing nothing.
	if errs == nil {
		writeHeader()
	}

	return errors.Join(errs, w.Flush())
}

// contentLibraryUploadCmd represents the hypervisor content-library upload command.
var contentLibraryUploadCmd = &cobra.Command{
	Use:     "upload <library_id> <filename>",
	Aliases: []string{"u"},
	Short:   "Upload a file to a content library",
	Long: `Uploads a local file to a content library, under its own name.

If '-' is given for <filename>, the contents are read from stdin, and --name is required
to say what the file should be called within the library.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return contentLibraryUpload(cmd.Context(), args[0], args[1])
	},
}

func contentLibraryUpload(ctx context.Context, libraryID, filename string) error {
	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	// Uploading the same image to every node of a cluster from one invocation is not what anyone
	// means by this command, and it would read the file once per node.
	ctx, c, node, err := clientFactory.BuildClientEnforceSingleNode(ctx, "talosctl hypervisor content-library upload")
	if err != nil {
		return err
	}

	var (
		contents io.Reader
		name     = contentLibraryUploadCmdFlags.name
	)

	if filename == "-" {
		if name == "" {
			return errors.New("--name is required when reading from stdin")
		}

		contents = os.Stdin
	} else {
		if name == "" {
			name = filepath.Base(filename)
		}

		f, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("error opening %q: %w", filename, err)
		}

		defer f.Close() //nolint:errcheck

		contents = f
	}

	resp, err := c.ContentLibraryUpload(ctx, libraryID, name, contentLibraryUploadCmdFlags.overwrite, contents)
	if err != nil {
		return fmt.Errorf("error uploading to node %s: %w", node, err)
	}

	safeout.Printf("uploaded %s (%s) to the content library %q on node %s\n",
		resp.GetName(), humanize.Bytes(resp.GetSize()), libraryID, node)

	return nil
}

// contentLibraryDeleteCmd represents the hypervisor content-library delete command.
var contentLibraryDeleteCmd = &cobra.Command{
	Use:     "delete <library_id> <filename>",
	Aliases: []string{"d", "rm"},
	Short:   "Delete a file from a content library",
	Long:    ``,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return contentLibraryDelete(cmd.Context(), args[0], args[1])
	},
}

func contentLibraryDelete(ctx context.Context, libraryID, name string) error {
	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	respCh := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.ContentLibraryServiceDeleteResponse, error) {
			return c.ContentLibraryClient.Delete(ctx, &machine.ContentLibraryServiceDeleteRequest{
				LibraryId: libraryID,
				Name:      name,
			})
		},
	)

	var errs error

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
		}
	}

	return errs
}

func init() {
	contentLibraryUploadCmd.Flags().StringVar(&contentLibraryUploadCmdFlags.name, "name", "", "name of the file within the library (defaults to the base name of <filename>)")
	contentLibraryUploadCmd.Flags().BoolVar(&contentLibraryUploadCmdFlags.overwrite, "overwrite", false, "replace a file of the same name if it already exists")

	contentLibraryCmd.AddCommand(contentLibraryListCmd)
	contentLibraryCmd.AddCommand(contentLibraryUploadCmd)
	contentLibraryCmd.AddCommand(contentLibraryDeleteCmd)

	hypervisorCmd.AddCommand(contentLibraryCmd)
	addCommand(hypervisorCmd)
}
