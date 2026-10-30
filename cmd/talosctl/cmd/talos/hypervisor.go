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
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var contentLibraryUploadCmdFlags struct {
	name   string
	digest string
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
	Long: `Content libraries store immutable SHA-256 objects under objects/sha256/<hex>.
Names at the library root are create-only symlinks. Identical uploads share one object.
Libraries are declared with ContentLibraryConfig.

Deleting a name retains its object, including retired cloud-init seed contents.
There is no automatic garbage collection.`,
	Args: cobra.NoArgs,
}

// contentLibraryListCmd represents the hypervisor content-library list command.
var contentLibraryListCmd = &cobra.Command{
	Use:     "list <library_id>",
	Aliases: []string{"l", "ls"},
	Short:   "List SHA256 objects and their attached names",
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
			return c.ContentLibraryService().List(ctx, &machine.ContentLibraryServiceListRequest{
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

		fmt.Fprintln(w, "NODE	SHA256	NAMES	SIZE")
	}

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		writeHeader()

		safeout.Fprintf(
			w, "%s	%s	%s	%s\n",
			resp.Node,
			resp.Payload.GetSha256(),
			strings.Join(resp.Payload.GetNames(), ","),
			humanize.Bytes(resp.Payload.GetSize()),
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
	Long: `Uploads a local file as an immutable SHA-256 object and creates a name for it.
An existing name is never overwritten, even when its contents are identical.
Choose a new name, or explicitly delete an unused name before reusing it.

If '-' is given for <filename>, the contents are read from stdin, and --name is required
to say what the file should be called within the library.

With --digest, the node verifies what it received before storing it, and refuses the
upload unless the contents hash to the digest given.`,
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

	resp, err := c.ContentLibraryUpload(ctx, libraryID, name, contentLibraryUploadCmdFlags.digest, contents)

	// Hashes computed on the node
	if digests := client.ContentLibraryUploadDigests(resp, err); digests != nil {
		safeout.Printf("Uploaded content's digest: %s\n", digests.GetSha256())
	}

	if err != nil {
		safeout.Flush() //nolint:errcheck

		fmt.Fprintln(os.Stderr, color.RedString("error uploading to node %s: %s", safeout.Cell(node), safeout.String(err.Error()))) //nolint:forbidigo // colored by hand, see above

		// Already reported, and reporting it again would print it a second time without the color.
		common.SuppressErrors = true

		return err
	}

	safeout.Printf("uploaded %s (%s) to the content library %q on node %s\n",
		resp.GetName(), humanize.Bytes(resp.GetSize()), libraryID, node)

	if contentLibraryUploadCmdFlags.digest != "" {
		safeout.Flush() //nolint:errcheck

		fmt.Fprintln(os.Stdout, color.GreenString( //nolint:forbidigo // colored by hand, see above
			"integrity verified: uploaded content's digest matches the provided one (%s)",
			safeout.Cell(contentLibraryUploadCmdFlags.digest),
		))
	}

	return nil
}

// contentLibraryDeleteCmd represents the hypervisor content-library delete command.
var contentLibraryDeleteCmd = &cobra.Command{
	Use:     "delete <library_id> <name|sha256:digest>",
	Aliases: []string{"d", "rm"},
	Short:   "Delete an alias or a SHA-256 object from a content library",
	Long: `A filename removes only that alias and retains the immutable object.
A sha256:<hex> target explicitly removes the stored object. If any symlinks refer
to it, deletion fails without changing them. Use --force to remove all of those
symlinks before removing the object. Pool-owned copies are not affected.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}

		return contentLibraryDelete(cmd.Context(), args[0], args[1], force)
	},
}

func contentLibraryDeleteRequest(libraryID, target string, force bool) (*machine.ContentLibraryServiceDeleteRequest, error) {
	req := &machine.ContentLibraryServiceDeleteRequest{LibraryId: libraryID}

	if strings.HasPrefix(target, "sha256:") {
		d, err := digest.Parse(target)
		if err != nil || d.Algorithm() != digest.SHA256 || strings.ToLower(target) != target {
			return nil, fmt.Errorf("invalid SHA-256 object identity %q: use sha256:<64 lowercase hexadecimal characters>", target)
		}

		req.Target = &machine.ContentLibraryServiceDeleteRequest_Sha256{Sha256: target}
		req.Force = force
	} else {
		if force {
			return nil, errors.New("--force requires a sha256:<hex> object target; deleting a name only removes its alias")
		}

		req.Target = &machine.ContentLibraryServiceDeleteRequest_Name{Name: target}
	}

	return req, nil
}

func contentLibraryDelete(ctx context.Context, libraryID, target string, force bool) error {
	req, err := contentLibraryDeleteRequest(libraryID, target, force)
	if err != nil {
		return err
	}

	clientFactory, err := NewClientFactory(ctx, nil)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	respCh := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.ContentLibraryServiceDeleteResponse, error) {
			return c.ContentLibraryService().Delete(ctx, req)
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
	contentLibraryDeleteCmd.Flags().Bool("force", false, "remove all symlinks to the selected SHA-256 object before deleting it")

	contentLibraryUploadCmd.Flags().StringVar(&contentLibraryUploadCmdFlags.name, "name", "", "new name within the library (defaults to the base name of <filename>)")
	contentLibraryUploadCmd.Flags().StringVar(&contentLibraryUploadCmdFlags.digest, "digest", "",
		"expected digest of the contents as sha256:<hex>; the upload is refused unless the contents match it")

	contentLibraryCmd.AddCommand(contentLibraryListCmd)
	contentLibraryCmd.AddCommand(contentLibraryUploadCmd)
	contentLibraryCmd.AddCommand(contentLibraryDeleteCmd)

	hypervisorCmd.AddCommand(contentLibraryCmd)
	addCommand(hypervisorCmd)
}
