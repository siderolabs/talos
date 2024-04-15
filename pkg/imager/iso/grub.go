// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// GRUBOptions described the input for the CreateGRUB function.
type GRUBOptions struct {
	KernelPath    string
	InitramfsPath string
	Cmdline       string
	Version       string

	ScratchDir string

	OutPath string
}

//go:embed grub.cfg
var grubCfgTemplate string

// CreateGRUB creates a GRUB-based ISO image.
//
// This iso supports both BIOS and UEFI booting.
func CreateGRUB(printf func(string, ...any), options GRUBOptions) error {
	if err := utils.CopyFiles(
		printf,
		utils.SourceDestination(options.KernelPath, filepath.Join(options.ScratchDir, "boot", "vmlinuz")),
		utils.SourceDestination(options.InitramfsPath, filepath.Join(options.ScratchDir, "boot", "initramfs.xz")),
	); err != nil {
		return err
	}

	printf("creating grub.cfg")

	var grubCfg bytes.Buffer

	tmpl, err := template.New("grub.cfg").
		Funcs(template.FuncMap{
			"quote": grub.Quote,
		}).
		Parse(grubCfgTemplate)
	if err != nil {
		return err
	}

	if err = tmpl.Execute(&grubCfg, struct {
		Cmdline        string
		AddResetOption bool
	}{
		Cmdline:        options.Cmdline,
		AddResetOption: quirks.New(options.Version).SupportsResetGRUBOption(),
	}); err != nil {
		return err
	}

	cfgPath := filepath.Join(options.ScratchDir, "boot/grub/grub.cfg")

	if err = os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}

	if err = os.WriteFile(cfgPath, grubCfg.Bytes(), 0o666); err != nil {
		return err
	}

	if err = utils.TouchFiles(printf, options.ScratchDir); err != nil {
		return err
	}

	printf("creating ISO image")

	return grubMkrescue(options.OutPath, options.ScratchDir)
}

func grubMkrescue(isoPath, scratchPath string) error {
	args := []string{
		"--compress=xz",
		"--output=" + isoPath,
		scratchPath,
	}

	if epoch, ok, err := utils.SourceDateEpoch(); err != nil {
		return err
	} else if ok {
		// set EFI FAT image serial number
		if err := os.Setenv("GRUB_FAT_SERIAL_NUMBER", fmt.Sprintf("%x", uint32(epoch))); err != nil {
			return err
		}

		args = append(args,
			"--",
			"-volume_date", "all_file_dates", fmt.Sprintf("=%d", epoch),
			"-volume_date", "uuid", time.Unix(epoch, 0).Format("2006010215040500"),
		)
	}

	_, err := cmd.Run("grub-mkrescue", args...)
	if err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	return nil
}
