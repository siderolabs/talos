/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package syslinux

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"text/template"

	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/version"
)

const extlinuxConfig = `DEFAULT Talos
  SAY Talos ({{ .Version }})
LABEL Talos
  KERNEL /vmlinuz
  INITRD /initramfs.xz
  APPEND {{ .Append }}`

const gptmbrbin = "/usr/share/gptmbr.bin"

// Syslinux represents the syslinux bootloader.
type Syslinux struct{}

// Prepare implements the Bootloader interface. It works by invoking writing
// gptmbr.bin to a block device.
func Prepare(dev string) (err error) {
	b, err := ioutil.ReadFile(gptmbrbin)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(dev, os.O_WRONLY, os.ModeDevice)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		return err
	}

	return nil
}

// Install implements the Bootloader interface. It sets up extlinux with the
// specified kernel parameters.
func Install(args string) (err error) {
	aux := struct {
		Version string
		Append  string
	}{
		Version: version.Tag,
		Append:  args,
	}

	b := []byte{}
	wr := bytes.NewBuffer(b)
	t := template.Must(template.New("extlinux").Parse(extlinuxConfig))
	if err = t.Execute(wr, aux); err != nil {
		return err
	}

	if err = os.MkdirAll(constants.NewRoot+"/boot/extlinux", os.ModeDir); err != nil {
		return err
	}

	log.Println("writing extlinux.conf to disk")
	if err = ioutil.WriteFile(constants.NewRoot+"/boot/extlinux/extlinux.conf", wr.Bytes(), 0600); err != nil {
		return err
	}

	if err = cmd("extlinux", "--install", constants.NewRoot+"/boot/extlinux"); err != nil {
		return err
	}

	return nil
}

func cmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
