// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/installer/pkg"
)

var cfg = []byte(`set default=0
set timeout=3

insmod all_video

terminal_input console
terminal_output console

menuentry "Talos ISO" {
	set gfxmode=auto
	set gfxpayload=text
	linux /boot/vmlinuz init_on_alloc=1 slab_nomerge pti=on panic=0 consoleblank=0 printk.devkmsg=on earlyprintk=ttyS0 console=tty0 console=ttyS0 talos.platform=metal
	initrd /boot/initramfs.xz
}`)

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
	isoCmd.Flags().StringVar(&outputArg, "output", "/out", "The output path")
	isoCmd.Flags().BoolVar(&tarToStdout, "tar-to-stdout", false, "Tar output and send to stdout")
	rootCmd.AddCommand(isoCmd)
}

//nolint:gocyclo
func runISOCmd() error {
	if err := os.MkdirAll(outputArg, 0o777); err != nil {
		return err
	}

	files := map[string]string{
		fmt.Sprintf("/usr/install/%s/vmlinuz", options.Arch):      "/mnt/boot/vmlinuz",
		fmt.Sprintf("/usr/install/%s/initramfs.xz", options.Arch): "/mnt/boot/initramfs.xz",
	}

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

	log.Println("creating grub.cfg")

	cfgPath := "/mnt/boot/grub/grub.cfg"

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(cfgPath, cfg, 0o666); err != nil {
		return err
	}

	log.Println("creating ISO")

	out := fmt.Sprintf("/tmp/talos-%s.iso", options.Arch)

	if err := pkg.CreateISO(out, "/mnt"); err != nil {
		return err
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
