// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

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

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/cmd"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Cfg reprsents the cfg file.
type Cfg struct {
	Default  string
	Fallback string
	Labels   []*Label
}

// Label reprsents a label in the cfg file.
type Label struct {
	Root   string
	Kernel string
	Initrd string
	Append string
}

const grubCfgTpl = `set default="{{ .Default }}"
{{ with .Fallback -}}
set fallback="{{ . }}"
{{- end }}
set timeout=0

{{ range $label := .Labels -}}
menuentry "{{ $label.Root }}" {
  linux {{ $label.Kernel }} {{ $label.Append }}
  initrd {{ $label.Initrd }}
}
{{- end }}
`

// const deviceMapTpl = `(hd0)    {{ . }}`

// Grub represents the grub bootloader.
type Grub struct{}

// Labels implements the Bootloader interface. It works by writing
// gptmbr.bin to a block device.
func (g *Grub) Labels() (current, next string, err error) {
	var b []byte

	if b, err = ioutil.ReadFile(GrubConfig); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			current = BootA

			return current, "", nil
		}

		return "", "", err
	}

	re := regexp.MustCompile(`^set default="(.*)"`)
	matches := re.FindAllSubmatch(b, -1)

	if len(matches) != 1 {
		return "", "", fmt.Errorf("failed to find default")
	}

	if len(matches[0]) != 2 {
		log.Printf("%+v", matches[0])
		return "", "", fmt.Errorf("expected 2 matches, got %d", len(matches[0]))
	}

	current = string(matches[0][1])
	switch current {
	case BootA:
		next = BootB
	case BootB:
		next = BootA
	default:
		return "", "", fmt.Errorf("unknown grub menuentry: %q", current)
	}

	return current, next, err
}

// Install implements the Bootloader interface. It sets up grub with the
// specified kernel parameters.
//
// nolint: gocyclo
func (g *Grub) Install(fallback string, config interface{}, sequence runtime.Sequence, bootPartitionFound bool) (err error) {
	grubcfg, ok := config.(*Cfg)
	if !ok {
		return errors.New("expected a grub config")
	}

	if err = writeCfg(GrubConfig, grubcfg); err != nil {
		return err
	}

	dev, err := probe.GetDevWithFileSystemLabel(constants.BootPartitionLabel)
	if err != nil {
		return fmt.Errorf("failed to probe boot partition: %w", err)
	}

	// nolint: errcheck
	defer dev.Close()

	blk, err := util.DevnameFromPartname(dev.Path)
	if err != nil {
		return err
	}

	blk = fmt.Sprintf("/dev/%s", blk)

	log.Printf("installing grub to %s", blk)

	// if strings.HasPrefix(blk, "/dev/loop") {
	// 	if err = os.Link(blk, "/dev/sdx"); err != nil {
	// 		return err
	// 	}

	// 	defer os.Remove("/dev/sdx")

	// 	if err = writeDeviceMap(GrubDeviceMap, blk); err != nil {
	// 		return err
	// 	}

	// 	defer os.Remove(GrubDeviceMap)
	// }

	if _, err = cmd.Run("grub-install", "--boot-directory="+constants.BootMountPoint, "--efi-directory="+constants.EFIMountPoint, blk); err != nil {
		return fmt.Errorf("failed to install grub: %w", err)
	}

	return nil
}

// Revert implements the bootloader interface.
func (g *Grub) Revert() error {
	return nil
}

// RevertTo implements the bootloader interface.
func (g *Grub) RevertTo(label string) error {
	return nil
}

func writeCfg(path string, grubcfg *Cfg) (err error) {
	b := []byte{}
	wr := bytes.NewBuffer(b)
	t := template.Must(template.New("grub").Parse(grubCfgTpl))

	if err = t.Execute(wr, grubcfg); err != nil {
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

	return nil
}

// func writeDeviceMap(path, loop string) (err error) {
// 	b := []byte{}
// 	wr := bytes.NewBuffer(b)
// 	t := template.Must(template.New("devicemap").Parse(deviceMapTpl))

// 	if err = t.Execute(wr, loop); err != nil {
// 		return err
// 	}

// 	dir := filepath.Dir(path)
// 	if err = os.MkdirAll(dir, os.ModeDir); err != nil {
// 		return err
// 	}

// 	log.Printf("writing %s to disk", path)

// 	log.Println(wr.String())

// 	if err = ioutil.WriteFile(path, wr.Bytes(), 0o600); err != nil {
// 		return err
// 	}

// 	return nil
// }
