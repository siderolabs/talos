// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/cmd/installer/pkg"
)

var isolinuxCfg = []byte(`DEFAULT ISO
  SAY Talos
LABEL ISO
  KERNEL /vmlinuz
  INITRD /initramfs.xz
  APPEND page_poison=1 slab_nomerge slub_debug=P pti=on consoleblank=0 console=tty0 talos.platform=iso`)

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
	rootCmd.AddCommand(isoCmd)
}

// nolint: gocyclo
func runISOCmd() error {
	for _, dir := range []string{"/mnt/isolinux", "/mnt/usr/install"} {
		log.Printf("creating %s", dir)

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	files := map[string][]string{
		"/usr/lib/syslinux/isolinux.bin": {"/mnt/isolinux/isolinux.bin"},
		"/usr/lib/syslinux/ldlinux.c32":  {"/mnt/isolinux/ldlinux.c32"},
		"/usr/install/vmlinuz":           {"/mnt/vmlinuz", "/mnt/usr/install/vmlinuz"},
		"/usr/install/initramfs.xz":      {"/mnt/initramfs.xz", "/mnt/usr/install/initramfs.xz"},
	}

	for src, dest := range files {
		for _, f := range dest {
			log.Printf("copying %s to %s", src, f)

			from, err := os.Open(src)
			if err != nil {
				return err
			}
			// nolint: errcheck
			defer from.Close()

			to, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE, 0666)
			if err != nil {
				return err
			}
			// nolint: errcheck
			defer to.Close()

			_, err = io.Copy(to, from)
			if err != nil {
				return err
			}
		}
	}

	log.Println("creating isolinux.cfg")

	if err := ioutil.WriteFile("/mnt/isolinux/isolinux.cfg", isolinuxCfg, 0666); err != nil {
		return err
	}

	log.Println("creating ISO")

	if err := pkg.Mkisofs("/tmp/talos.iso", "/mnt"); err != nil {
		return err
	}

	log.Println("creating hybrid ISO")

	if err := pkg.Isohybrid("/tmp/talos.iso"); err != nil {
		return err
	}

	from, err := os.Open("/tmp/talos.iso")
	if err != nil {
		log.Fatal(err)
	}
	// nolint: errcheck
	defer from.Close()

	to, err := os.OpenFile(filepath.Join(outputArg, "talos.iso"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	// nolint: errcheck
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
