/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

// cpCmd represents the cp command
var cpCmd = &cobra.Command{
	Use:   "cp <src-path> -|<local-path>",
	Short: "Copy data out from the node",
	Long: `Creates an .tar.gz archive at the node starting at <src-path> and
streams it back to the client.

If '-' is given for <local-path>, archive is written to stdout.
Otherwise archive is extracted to <local-path> which should be an empty directory or
osctl creates a directory if <local-path> doesn't exist. Command doesn't preserve
ownership and access mode for the files in extract mode, while  streamed .tar archive
captures ownership and permission bits.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			r, errCh, err := c.CopyOut(globalCtx, args[0])
			if err != nil {
				helpers.Fatalf("error copying: %s", err)
			}

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				for err := range errCh {
					fmt.Fprintln(os.Stderr, err.Error())
				}
			}()

			defer wg.Wait()

			localPath := args[1]

			if localPath == "-" {
				// nolint: errcheck
				_, err = io.Copy(os.Stdout, r)
				if err != nil {
					helpers.Fatalf("error copying: %s", err)
				}
				return
			}

			localPath = filepath.Clean(localPath)

			fi, err := os.Stat(localPath)
			if err == nil && !fi.IsDir() {
				helpers.Fatalf("local path %q should be a directory", args[1])
			}
			if err != nil {
				if !os.IsNotExist(err) {
					helpers.Fatalf("failed to stat local path: %s", err)
				}
				if err = os.MkdirAll(localPath, 0777); err != nil {
					helpers.Fatalf("error creating local path %q: %s", localPath, err)
				}
			}

			zr, err := gzip.NewReader(r)
			if err != nil {
				helpers.Fatalf("error initializing gzip: %s", err)
			}
			tr := tar.NewReader(zr)

			for {
				hdr, err := tr.Next()
				if err != nil {
					if err == io.EOF {
						break
					}
					helpers.Fatalf("error reading tar header: %s", err)
				}

				path := filepath.Clean(filepath.Join(localPath, hdr.Name))
				// TODO: do we need to clean up any '..' references?

				switch hdr.Typeflag {
				case tar.TypeDir:
					mode := hdr.FileInfo().Mode()
					mode |= 0700 // make rwx for the owner
					if err = os.Mkdir(path, mode); err != nil {
						helpers.Fatalf("error creating directory %q mode %s: %s", path, mode, err)
					}
					if err = os.Chmod(path, mode); err != nil {
						helpers.Fatalf("error updating mode %s for %q: %s", mode, path, err)
					}
				case tar.TypeSymlink:
					if err = os.Symlink(hdr.Linkname, path); err != nil {
						helpers.Fatalf("error creating symlink %q -> %q: %s", path, hdr.Linkname, err)
					}
				default:
					mode := hdr.FileInfo().Mode()
					fp, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
					if err != nil {
						helpers.Fatalf("error creating file %q mode %s: %s", path, mode, err)
					}

					_, err = io.Copy(fp, tr)
					if err != nil {
						helpers.Fatalf("error copying data to %q: %s", path, err)
					}

					if err = fp.Close(); err != nil {
						helpers.Fatalf("error closing %q: %s", path, err)
					}

					if err = os.Chmod(path, mode); err != nil {
						helpers.Fatalf("error updating mode %s for %q: %s", mode, path, err)
					}
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(cpCmd)
}
