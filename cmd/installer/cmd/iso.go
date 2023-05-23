// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/siderolabs/go-procfs/procfs"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/installer/pkg"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

var (
	//go:embed grub.iso.cfg
	isoGrubCfg []byte
	secureBoot bool
)

// isoCmd represents the iso command.
var isoCmd = &cobra.Command{
	Use:   "iso",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runISOCmd(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	isoCmd.Flags().BoolVar(&secureBoot, "secureboot", false, "Create SecureBoot ISO")
	isoCmd.Flags().StringVar(&outputArg, "output", "/out", "The output path")
	isoCmd.Flags().BoolVar(&tarToStdout, "tar-to-stdout", false, "Tar output and send to stdout")
	rootCmd.AddCommand(isoCmd)
}

//nolint:gocyclo,cyclop
func runISOCmd() error {
	if err := os.MkdirAll(outputArg, 0o777); err != nil {
		return err
	}

	out := fmt.Sprintf("/tmp/talos-%s.iso", options.Arch)

	if secureBoot {
		out = fmt.Sprintf("/tmp/talos-secureboot-%s.iso", options.Arch)

		files := map[string]string{
			fmt.Sprintf("/usr/install/%s/systemd-boot.signed.efi", options.Arch): "/mnt/systemd-boot.signed.efi",
			fmt.Sprintf("/usr/install/%s/vmlinuz.signed.efi", options.Arch):      "/mnt/vmlinuz.signed.efi",
		}

		if err := copyFiles(files); err != nil {
			return err
		}

		log.Println("creating SecureBoot ISO")

		if err := pkg.CreateSecureBootISO(out, "/mnt", options.Arch); err != nil {
			return err
		}

	} else {
		files := map[string]string{
			fmt.Sprintf("/usr/install/%s/vmlinuz", options.Arch):      "/mnt/boot/vmlinuz",
			fmt.Sprintf("/usr/install/%s/initramfs.xz", options.Arch): "/mnt/boot/initramfs.xz",
		}

		if err := copyFiles(files); err != nil {
			return err
		}

		log.Println("creating grub.cfg")

		// ISO is always using platform "metal".
		p := &metal.Metal{}

		cmdline := procfs.NewCmdline("")
		cmdline.Append(constants.KernelParamPlatform, p.Name())
		cmdline.Append("earlyprintk", "ttyS0")

		cmdline.SetAll(p.KernelArgs().Strings())

		if err := cmdline.AppendAll(kernel.DefaultArgs); err != nil {
			return err
		}

		if err := cmdline.AppendAll(options.ExtraKernelArgs, procfs.WithOverwriteArgs("console")); err != nil {
			return err
		}

		if metaValues := options.MetaValues.GetSlice(); len(metaValues) > 0 {
			// pass META values as kernel talos.environment args which will be passed via the environment to the installer
			metaBase64 := base64.StdEncoding.EncodeToString([]byte(strings.Join(metaValues, ";")))
			cmdline.Append(constants.KernelParamEnvironment, metaValueEnvVariable+"="+metaBase64)
		}

		var grubCfg bytes.Buffer

		tmpl, err := template.New("grub.cfg").
			Funcs(template.FuncMap{
				"quote": grub.Quote,
			}).
			Parse(string(isoGrubCfg))
		if err != nil {
			return err
		}

		if err = tmpl.Execute(&grubCfg, struct {
			Cmdline string
		}{
			Cmdline: cmdline.String(),
		}); err != nil {
			return err
		}

		cfgPath := "/mnt/boot/grub/grub.cfg"

		if err = os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return err
		}

		if err = os.WriteFile(cfgPath, grubCfg.Bytes(), 0o666); err != nil {
			return err
		}

		if err = pkg.TouchFiles("/mnt"); err != nil {
			return err
		}

		log.Println("creating ISO")

		if err = pkg.CreateISO(out, "/mnt"); err != nil {
			return err
		}
	}

	from, err := os.Open(out)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer from.Close()

	to, err := os.OpenFile(filepath.Join(outputArg, filepath.Base(out)), os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}

	if tarToStdout {
		if err := tarOutput(); err != nil {
			return err
		}
	}

	return nil
}

func copyFiles(files map[string]string) error {
	for src, dest := range files {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		log.Printf("copying %s to %s", src, dest)

		from, err := os.Open(src)
		if err != nil {
			return err
		}
		//nolint:errcheck
		defer from.Close()

		to, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE, 0o666)
		if err != nil {
			return err
		}
		//nolint:errcheck
		defer to.Close()

		_, err = io.Copy(to, from)
		if err != nil {
			return err
		}
	}

	return nil
}
