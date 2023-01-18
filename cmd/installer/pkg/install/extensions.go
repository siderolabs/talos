// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package install provides the installation routine.
package install

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"github.com/siderolabs/talos/internal/pkg/extensions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	extinterface "github.com/siderolabs/talos/pkg/machinery/extensions"
)

// nolint:gocyclo
func (i *Installer) installExtensions() error {
	extensionsList, err := extensions.List(constants.SystemExtensionsPath)
	if err != nil {
		return fmt.Errorf("error listing extensions: %w", err)
	}

	if len(extensionsList) == 0 {
		return nil
	}

	if err = printExtensions(extensionsList); err != nil {
		return err
	}

	if err = validateExtensions(extensionsList); err != nil {
		return err
	}

	extensionsPathWithKernelModules := findExtensionsWithKernelModules(extensionsList)

	if len(extensionsPathWithKernelModules) > 0 {
		kernelModuleDepExtension, genErr := extensions.GenerateKernelModuleDependencyTreeExtension(extensionsPathWithKernelModules, i.options.Arch)
		if genErr != nil {
			return genErr
		}

		extensionsList = append(extensionsList, kernelModuleDepExtension)
	}

	tempDir, err := os.MkdirTemp("", "ext")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir) //nolint:errcheck

	var cfg *extinterface.Config

	if cfg, err = compressExtensions(extensionsList, tempDir); err != nil {
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
		path, err := ext.Compress(tempDir, tempDir)
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

func findExtensionsWithKernelModules(extensions []*extensions.Extension) []string {
	var modulesPath []string

	for _, ext := range extensions {
		if ext.ProvidesKernelModules() {
			modulesPath = append(modulesPath, ext.KernelModuleDirectory())
		}
	}

	return modulesPath
}

func buildContents(path string) (io.Reader, error) {
	var listing bytes.Buffer

	if err := buildContentsRecursive(path, "", &listing); err != nil {
		return nil, err
	}

	return &listing, nil
}

func buildContentsRecursive(basePath, path string, w io.Writer) error {
	if path != "" {
		fmt.Fprintf(w, "%s\n", path)
	}

	st, err := os.Stat(filepath.Join(basePath, path))
	if err != nil {
		return err
	}

	if !st.IsDir() {
		return nil
	}

	contents, err := os.ReadDir(filepath.Join(basePath, path))
	if err != nil {
		return err
	}

	for _, item := range contents {
		if err = buildContentsRecursive(basePath, filepath.Join(path, item.Name()), w); err != nil {
			return err
		}
	}

	return nil
}

func (i *Installer) rebuildInitramfs(tempDir string) error {
	initramfsAsset := fmt.Sprintf(constants.InitramfsAssetPath, i.options.Arch)

	log.Printf("creating system extensions initramfs archive and compressing it")

	// the code below runs the equivalent of:
	//   find $tempDir -print | cpio -H newc --create --reproducible | xz -v -C crc32 -0 -e -T 0 -z

	listing, err := buildContents(tempDir)
	if err != nil {
		return err
	}

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}

	defer pipeR.Close() //nolint:errcheck
	defer pipeW.Close() //nolint:errcheck

	// build cpio image which contains .sqsh images and extensions.yaml
	cmd1 := exec.Command("cpio", "-H", "newc", "--create", "--reproducible")
	cmd1.Dir = tempDir
	cmd1.Stdin = listing
	cmd1.Stdout = pipeW
	cmd1.Stderr = os.Stderr

	if err = cmd1.Start(); err != nil {
		return err
	}

	if err = pipeW.Close(); err != nil {
		return err
	}

	destination, err := os.OpenFile(initramfsAsset, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	defer destination.Close() //nolint:errcheck

	// append compressed initramfs.sysext to the original initramfs.xz, kernel can read such format
	cmd2 := exec.Command("xz", "-v", "-C", "crc32", "-0", "-e", "-T", "0", "-z")
	cmd2.Dir = tempDir
	cmd2.Stdin = pipeR
	cmd2.Stdout = destination
	cmd2.Stderr = os.Stderr

	if err = cmd2.Start(); err != nil {
		return err
	}

	if err = pipeR.Close(); err != nil {
		return err
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- cmd1.Wait()
	}()

	go func() {
		errCh <- cmd2.Wait()
	}()

	for i := 0; i < 2; i++ {
		if err = <-errCh; err != nil {
			return err
		}
	}

	return destination.Sync()
}
