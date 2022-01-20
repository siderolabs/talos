// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"github.com/talos-systems/talos/internal/pkg/extensions"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	extinterface "github.com/talos-systems/talos/pkg/machinery/extensions"
)

func (i *Installer) installExtensions() error {
	extensions, err := extensions.List(constants.SystemExtensionsPath)
	if err != nil {
		return fmt.Errorf("error listing extensions: %w", err)
	}

	if len(extensions) == 0 {
		return nil
	}

	if err = printExtensions(extensions); err != nil {
		return err
	}

	if err = validateExtensions(extensions); err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "ext")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir) //nolint:errcheck

	var cfg *extinterface.Config

	if cfg, err = compressExtensions(extensions, tempDir); err != nil {
		return err
	}

	if err = cfg.Write(filepath.Join(tempDir, constants.ExtensionsConfigFile)); err != nil {
		return err
	}

	if err = i.rebuildInitramfs(tempDir); err != nil {
		return err
	}

	return nil
}

func printExtensions(extensions []*extensions.Extension) error {
	log.Printf("discovered system extensions:")

	var b bytes.Buffer

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)

	fmt.Fprint(w, "NAME\tVERSION\tAUTHOR\n")

	for _, ext := range extensions {
		fmt.Fprintf(w, "%s\t%s\t%s\n", ext.Manifest.Metadata.Name, ext.Manifest.Metadata.Version, ext.Manifest.Metadata.Author)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for {
		line, err := b.ReadString('\n')
		if err != nil {
			break
		}

		log.Printf("%s", line)
	}

	return nil //nolint:nilerr
}

func validateExtensions(extensions []*extensions.Extension) error {
	log.Printf("validating system extensions")

	for _, ext := range extensions {
		if err := ext.Validate(); err != nil {
			return fmt.Errorf("error validating extension %q: %w", ext.Manifest.Metadata.Name, err)
		}
	}

	return nil
}

func compressExtensions(extensions []*extensions.Extension, tempDir string) (*extinterface.Config, error) {
	cfg := &extinterface.Config{}

	log.Printf("compressing system extensions")

	for _, ext := range extensions {
		path, err := ext.Compress(tempDir)
		if err != nil {
			return nil, fmt.Errorf("error compressing extension %q: %w", ext.Manifest.Metadata.Name, err)
		}

		cfg.Layers = append(cfg.Layers, &extinterface.Layer{
			Image:    filepath.Base(path),
			Metadata: ext.Manifest.Metadata,
		})
	}

	return cfg, nil
}

func (i *Installer) rebuildInitramfs(tempDir string) error {
	initramfsAsset := fmt.Sprintf(constants.InitramfsAssetPath, i.options.Arch)

	log.Printf("creating system extensions initramfs archive")

	contents, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}

	var listing bytes.Buffer

	for _, item := range contents {
		fmt.Fprintf(&listing, "%s\n", item.Name())
	}

	// build cpio image which contains .sqsh images and extensions.yaml
	cmd := exec.Command("cpio", "-H", "newc", "--create", "--reproducible", "-F", "initramfs.sysext")
	cmd.Dir = tempDir
	cmd.Stdin = &listing
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return err
	}

	log.Printf("compressing system extensions initramfs archive")

	source, err := os.OpenFile(filepath.Join(tempDir, "initramfs.sysext"), os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	defer source.Close() //nolint:errcheck

	destination, err := os.OpenFile(initramfsAsset, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	defer destination.Close() //nolint:errcheck

	// append compressed initramfs.sysext to the original initramfs.xz, kernel can read such format
	cmd = exec.Command("xz", "-v", "-C", "crc32", "-0", "-e", "-T", "0", "-z")
	cmd.Dir = tempDir
	cmd.Stdin = source
	cmd.Stdout = destination
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return err
	}

	return nil
}
