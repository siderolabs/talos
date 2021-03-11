// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package syslinux

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"github.com/talos-systems/go-cmd/pkg/cmd"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	advcommon "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv/syslinux"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const syslinuxCfgTpl = `DEFAULT {{ .Default }}
PROMPT 1
TIMEOUT 50

{{- range .Labels }}
INCLUDE /{{ .Root }}/include.cfg
{{- end }}`

const syslinuxLabelTpl = `LABEL {{ .Root }}
  KERNEL {{ .Kernel }}
  INITRD {{ .Initrd }}
  APPEND {{ .Append }}
`

// Cfg reprsents the cfg file.
type Cfg struct {
	Default string
	Labels  []*Label
}

// Label reprsents a label in the cfg file.
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

	f, err := os.OpenFile(dev, os.O_WRONLY|unix.O_CLOEXEC, os.ModeDevice)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer f.Close()

	if _, err := f.Write(b); err != nil {
		return err
	}

	return nil
}

// Install implements the Bootloader interface. It sets up syslinux with the
// specified kernel parameters.
func Install(fallback string, config interface{}, sequence runtime.Sequence, bootPartitionFound bool) (err error) {
	syslinuxcfg, ok := config.(*Cfg)
	if !ok {
		return errors.New("expected a syslinux config")
	}

	if err = writeCfg(constants.BootMountPoint, SyslinuxConfig, syslinuxcfg); err != nil {
		return err
	}

	if sequence == runtime.SequenceUpgrade && bootPartitionFound {
		log.Println("updating syslinux")

		if err = update(); err != nil {
			return err
		}
	} else {
		log.Println("installing syslinux")

		if err = install(); err != nil {
			return err
		}
	}

	if err = writeUEFIFiles(); err != nil {
		return err
	}

	if sequence == runtime.SequenceUpgrade {
		if err = setADV(SyslinuxLdlinux, fallback); err != nil {
			return err
		}
	}

	return nil
}

// Labels parses the syslinux config and returns the current active label, and
// what should be the next label.
func Labels() (current, next string, err error) {
	var b []byte

	if b, err = ioutil.ReadFile(SyslinuxConfig); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			current = BootA

			return current, "", nil
		}

		return "", "", err
	}

	re := regexp.MustCompile(`^DEFAULT\s(.*)`)
	matches := re.FindSubmatch(b)

	if len(matches) != 2 {
		return "", "", fmt.Errorf("expected 2 matches, got %d", len(matches))
	}

	current = string(matches[1])
	switch current {
	case BootA:
		next = BootB
	case BootB:
		next = BootA
	default:
		return "", "", fmt.Errorf("unknown syslinux label: %q", current)
	}

	return current, next, err
}

// Default sets the default syslinx label.
func Default(label string) (err error) {
	log.Printf("setting default label to %q", label)

	var b []byte

	if b, err = ioutil.ReadFile(SyslinuxConfig); err != nil {
		return err
	}

	re := regexp.MustCompile(`^DEFAULT\s(.*)`)
	matches := re.FindSubmatch(b)

	if len(matches) != 2 {
		return fmt.Errorf("expected 2 matches, got %d", len(matches))
	}

	b = re.ReplaceAll(b, []byte(fmt.Sprintf("DEFAULT %s", label)))

	return ioutil.WriteFile(SyslinuxConfig, b, 0o600)
}

func writeCfg(base, path string, syslinuxcfg *Cfg) (err error) {
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

	if err = ioutil.WriteFile(path, wr.Bytes(), 0o600); err != nil {
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

		log.Printf("writing syslinux label %q to disk", label.Root)

		if err = ioutil.WriteFile(filepath.Join(dir, "include.cfg"), wr.Bytes(), 0o600); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", src)
	}

	s, err := os.Open(src)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer d.Close()

	_, err = io.Copy(d, s)

	return err
}

func writeUEFIFiles() (err error) {
	dir := filepath.Join(constants.BootMountPoint, "EFI", "BOOT")

	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	for src, dest := range map[string]string{syslinuxefi: filepath.Join(dir, "BOOTX64.EFI"), ldlinuxe64: filepath.Join(dir, "ldlinux.e64")} {
		if err = copyFile(src, dest); err != nil {
			return err
		}
	}

	return nil
}

func install() (err error) {
	_, err = cmd.Run("extlinux", "--install", filepath.Dir(SyslinuxConfig))
	if err != nil {
		return fmt.Errorf("failed to install syslinux: %w", err)
	}

	return nil
}

func update() (err error) {
	if _, err = cmd.Run("extlinux", "--update", filepath.Dir(SyslinuxConfig)); err != nil {
		return fmt.Errorf("failed to update syslinux: %w", err)
	}

	return nil
}

func setADV(ldlinux, fallback string) (err error) {
	var f *os.File

	if f, err = os.OpenFile(ldlinux, os.O_RDWR, 0o700); err != nil {
		return err
	}

	//nolint:errcheck
	defer f.Close()

	var adv syslinux.ADV

	if adv, err = syslinux.NewADV(f); err != nil {
		return err
	}

	if ok := adv.SetTag(advcommon.Upgrade, fallback); !ok {
		return fmt.Errorf("failed to set upgrade tag: %q", fallback)
	}

	if _, err = f.Write(adv); err != nil {
		return err
	}

	log.Printf("set fallback to %q", fallback)

	return nil
}
