// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"text/template"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

//go:embed grub.cfg
var grubCfgTemplate string

// CreateGRUB creates a GRUB-based ISO image.
//
// This iso supports both BIOS and UEFI booting.
func (options Options) CreateGRUB(printf func(string, ...any)) (Generator, error) {
	if err := utils.CopyFiles(
		printf,
		utils.SourceDestination(options.KernelPath, filepath.Join(options.ScratchDir, "boot", "vmlinuz")),
		utils.SourceDestination(options.InitramfsPath, filepath.Join(options.ScratchDir, "boot", "initramfs.xz")),
	); err != nil {
		return nil, err
	}

	printf("creating grub.cfg")

	var grubCfg bytes.Buffer

	tmpl, err := template.New("grub.cfg").
		Funcs(template.FuncMap{
			"quote": grub.Quote,
		}).
		Parse(grubCfgTemplate)
	if err != nil {
		return nil, err
	}

	if err = tmpl.Execute(&grubCfg, struct {
		Cmdline        string
		AddResetOption bool
	}{
		Cmdline:        options.Cmdline,
		AddResetOption: quirks.New(options.Version).SupportsResetGRUBOption(),
	}); err != nil {
		return nil, err
	}

	cfgPath := filepath.Join(options.ScratchDir, "boot/grub/grub.cfg")

	if err = os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return nil, err
	}

	if err = os.WriteFile(cfgPath, grubCfg.Bytes(), 0o666); err != nil {
		return nil, err
	}

	if err = utils.TouchFiles(printf, options.ScratchDir); err != nil {
		return nil, err
	}

	printf("creating ISO image")

	return &ExecutorOptions{
		Command: "grub-mkrescue",
		Version: options.Version,
		Arguments: []string{
			"--compress=xz",
			"--output=" + options.OutPath,
			"--verbose",
			options.ScratchDir,
			"-iso-level", "3",
			"--",
		},
	}, nil
}
