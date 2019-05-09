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

const extlinuxConf = `DEFAULT {{ .Default }}
  SAY Talos
{{- range .Labels }}
INCLUDE /{{ .Root }}/include.conf
{{ end }}`

const extlinuxConfLabel = `LABEL {{ .Root }}
  KERNEL {{ .Kernel }}
  INITRD {{ .Initrd }}
  APPEND {{ .Append }}
`

const gptmbrbin = "/usr/lib/syslinux/gptmbr.bin"

// ExtlinuxConf reprsents the syslinux extlinux.conf file.
type ExtlinuxConf struct {
	Default string
	Labels  []*ExtlinuxConfLabel
}

// ExtlinuxConfLabel reprsents a label in the syslinux extlinux.conf file.
type ExtlinuxConfLabel struct {
	Root   string
	Kernel string
	Initrd string
	Append string
}

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
func Install(base string, config interface{}) (err error) {
	extlinuxconf, ok := config.(*ExtlinuxConf)
	if !ok {
		return errors.New("expected extlinux")
	}

	path := filepath.Join(base, "extlinux", "extlinux.conf")
	if err = WriteExtlinuxConf(base, path, extlinuxconf); err != nil {
		return err
	}

	if err = cmd("extlinux", "--install", filepath.Dir(path)); err != nil {
		return err
	}

	return nil
}

// WriteExtlinuxConf write extlinux.conf to disk.
func WriteExtlinuxConf(base, path string, extlinuxconf *ExtlinuxConf) (err error) {
	b := []byte{}
	wr := bytes.NewBuffer(b)
	t := template.Must(template.New("extlinux").Parse(extlinuxConf))
	if err = t.Execute(wr, extlinuxconf); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, os.ModeDir); err != nil {
		return err
	}

	log.Println("writing extlinux.conf to disk")
	if err = ioutil.WriteFile(path, wr.Bytes(), 0600); err != nil {
		return err
	}

	for _, label := range extlinuxconf.Labels {
		b = []byte{}
		wr = bytes.NewBuffer(b)
		t = template.Must(template.New("extlinux").Parse(extlinuxConfLabel))
		if err = t.Execute(wr, label); err != nil {
			return err
		}

		dir = filepath.Join(base, label.Root)
		if err = os.MkdirAll(dir, os.ModeDir); err != nil {
			return err
		}

		log.Printf("writing extlinux label %s to disk", label.Root)
		if err = ioutil.WriteFile(filepath.Join(dir, "include.conf"), wr.Bytes(), 0600); err != nil {
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
