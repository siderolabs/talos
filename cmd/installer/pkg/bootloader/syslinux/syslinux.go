// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package syslinux

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/cmd"
	"github.com/talos-systems/talos/pkg/constants"

	"golang.org/x/sys/unix"
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

// Labels parses the syslinux config and returns the current active label, and
// what should be the next label.
func Labels() (current string, next string, err error) {
	var b []byte

	if b, err = ioutil.ReadFile(constants.SyslinuxConfig); err != nil {
		return "", "", err
	}

	re := regexp.MustCompile(`^DEFAULT\s(.*)`)
	matches := re.FindSubmatch(b)

	if len(matches) != 2 {
		return "", "", fmt.Errorf("expected 2 matches, got %d", len(matches))
	}

	current = string(matches[1])
	switch current {
	case constants.BootA:
		next = constants.BootB
	case constants.BootB:
		next = constants.BootA
	default:
		return "", "", fmt.Errorf("unknown syslinux label: %q", current)
	}

	return current, next, err
}

// Prepare implements the Bootloader interface. It works by writing
// gptmbr.bin to a block device.
func Prepare(dev string) (err error) {
	b, err := ioutil.ReadFile(gptmbrbin)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(dev, os.O_WRONLY|unix.O_CLOEXEC, os.ModeDevice)
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
//
// nolint: gocyclo
func Install(base, label string, config interface{}, sequence runtime.Sequence, bootPartitionFound bool) (err error) {
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

	err = WriteAllSyslinuxCfgs(base, syslinuxcfg)
	if err != nil {
		return err
	}

	dir := filepath.Dir(filepath.Join(base, "syslinux", "syslinux.cfg"))

	if sequence == runtime.Upgrade && bootPartitionFound {
		if _, err = cmd.Run("extlinux", "--update", dir); err != nil {
			return fmt.Errorf("failed to update extlinux: %w", err)
		}

		if _, err = cmd.Run("extlinux", "--once="+label, dir); err != nil {
			return fmt.Errorf("failed to set label for next boot: %w", err)
		}
	} else {
		_, err = cmd.Run("extlinux", "--install", dir)
		if err != nil {
			return fmt.Errorf("failed to install extlinux: %w", err)
		}
	}

	if sequence == runtime.Upgrade {
		var f *os.File

		if f, err = os.OpenFile(constants.SyslinuxLdlinux, os.O_RDWR, 0700); err != nil {
			return err
		}

		// nolint: errcheck
		defer f.Close()

		var adv ADV

		if adv, err = NewADV(f); err != nil {
			return err
		}

		if ok := adv.SetTag(AdvUpgrade, label); !ok {
			return fmt.Errorf("failed to set upgrade tag: %q", label)
		}

		if _, err = f.Write(adv); err != nil {
			return err
		}

		log.Println("set upgrade tag in ADV")
	}

	return nil
}

// WriteAllSyslinuxCfgs writes legacy and EFI syslinux configs to disk.
func WriteAllSyslinuxCfgs(base string, syslinuxcfg *Cfg) (err error) {
	paths := []string{filepath.Join(base, "syslinux", "syslinux.cfg"), filepath.Join(base, "EFI", "syslinux", "syslinux.cfg")}
	for _, path := range paths {
		if err = WriteSyslinuxCfg(base, path, syslinuxcfg); err != nil {
			return err
		}
	}

	return nil
}

// WriteSyslinuxCfg writes a syslinux config to disk.
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

	log.Printf("writing %s to disk", path)

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
