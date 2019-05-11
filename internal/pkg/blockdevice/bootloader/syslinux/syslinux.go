/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package syslinux

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const syslinuxCfgTpl = `DEFAULT {{ .Default }}
  SAY Talos
{{- range .Labels }}
INCLUDE /{{ .Root }}/include.cfg
{{- end }}`

const syslinuxLabelTpl = `LABEL {{ .Root }}
  KERNEL {{ .Kernel }}
  INITRD {{ .Initrd }}
  APPEND {{ .Append }}
`

const (
	gptmbrbin   = "/usr/lib/syslinux/gptmbr.bin"
	syslinuxefi = "/usr/lib/syslinux/syslinux.efi"
	ldlinuxe64  = "/usr/lib/syslinux/ldlinux.e64"
)

// Cfg reprsents the syslinux.cfg file.
type Cfg struct {
	Default string
	Labels  []*Label
}

// Label reprsents a label in the syslinux.cfg file.
type Label struct {
	Root   string
	Kernel string
	Initrd string
	Append string
}

// Syslinux represents the syslinux bootloader.
type Syslinux struct{}

// Prepare implements the Bootloader interface. It works by writing
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

// Install implements the Bootloader interface. It sets up syslinux with the
// specified kernel parameters.
func Install(base string, config interface{}) (err error) {
	syslinuxcfg, ok := config.(*Cfg)
	if !ok {
		return errors.New("expected a syslinux config")
	}

	efiDir := filepath.Join(base, "EFI", "BOOT")
	if err = os.MkdirAll(efiDir, 0700); err != nil {
		return err
	}

	input, err := ioutil.ReadFile(syslinuxefi)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(efiDir+"/BOOTX64.EFI", input, 0600)
	if err != nil {
		return err
	}

	input, err = ioutil.ReadFile(ldlinuxe64)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(efiDir+"/ldlinux.e64", input, 0600)
	if err != nil {
		return err
	}

	paths := []string{filepath.Join(base, "syslinux", "syslinux.cfg"), filepath.Join(base, "EFI", "syslinux", "syslinux.cfg")}
	for _, path := range paths {
		if err = WriteSyslinuxCfg(base, path, syslinuxcfg); err != nil {
			return err
		}
	}

	if err = cmd("extlinux", "--install", filepath.Dir(paths[0])); err != nil {
		return err
	}

	return nil
}

// WriteSyslinuxCfg write syslinux.cfg to disk.
func WriteSyslinuxCfg(base, path string, syslinuxcfg *Cfg) (err error) {
	b := []byte{}
	wr := bytes.NewBuffer(b)
	t := template.Must(template.New("syslinux").Parse(syslinuxCfgTpl))
	if err = t.Execute(wr, syslinuxcfg); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, os.ModeDir); err != nil {
		return err
	}

	log.Println("writing syslinux.cfg to disk")
	if err = ioutil.WriteFile(path, wr.Bytes(), 0600); err != nil {
		return err
	}

	for _, label := range syslinuxcfg.Labels {
		b = []byte{}
		wr = bytes.NewBuffer(b)
		t = template.Must(template.New("syslinux").Parse(syslinuxLabelTpl))
		if err = t.Execute(wr, label); err != nil {
			return err
		}

		dir = filepath.Join(base, label.Root)
		if err = os.MkdirAll(dir, os.ModeDir); err != nil {
			return err
		}

		log.Printf("writing syslinux label %s to disk", label.Root)
		if err = ioutil.WriteFile(filepath.Join(dir, "include.cfg"), wr.Bytes(), 0600); err != nil {
			return err
		}
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
