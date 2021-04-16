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
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"text/template"

	"github.com/talos-systems/go-blockdevice/blockdevice"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
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
set timeout=3

insmod all_video

terminal_input console
terminal_output console

{{ range $label := .Labels -}}
menuentry "{{ $label.Root }}" {
  set gfxmode=auto
  set gfxpayload=text
  linux {{ $label.Kernel }} {{ $label.Append }}
  initrd {{ $label.Initrd }}
}
{{ end }}
`

const (
	amd64 = "amd64"
	arm64 = "arm64"
)

// Grub represents the grub bootloader.
type Grub struct {
	BootDisk string
	Arch     string
}

// Labels implements the Bootloader interface.
func (g *Grub) Labels() (current, next string, err error) {
	var b []byte

	if b, err = ioutil.ReadFile(GrubConfig); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			next = BootA

			return current, next, nil
		}

		return "", "", err
	}

	re := regexp.MustCompile(`^set default="(.*)"`)
	matches := re.FindAllSubmatch(b, -1)

	if len(matches) != 1 {
		return "", "", fmt.Errorf("failed to find default")
	}

	if len(matches[0]) != 2 {
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
//nolint:gocyclo
func (g *Grub) Install(fallback string, config interface{}, sequence runtime.Sequence) (err error) {
	grubcfg, ok := config.(*Cfg)
	if !ok {
		return errors.New("expected a grub config")
	}

	if err = writeCfg(GrubConfig, grubcfg); err != nil {
		return err
	}

	dev, err := blockdevice.Open(g.BootDisk)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer dev.Close()

	// verify that BootDisk has boot partition
	_, err = dev.GetPartition(constants.BootPartitionLabel)
	if err != nil {
		return err
	}

	blk := dev.Device().Name()

	loopDevice := strings.HasPrefix(blk, "/dev/loop")

	var platforms []string

	switch g.Arch {
	case amd64:
		platforms = []string{"x86_64-efi", "i386-pc"}
	case arm64:
		platforms = []string{"arm64-efi"}
	}

	if goruntime.GOARCH == amd64 && g.Arch == amd64 && !loopDevice {
		// let grub choose the platform automatically if not building an image
		platforms = []string{""}
	}

	for _, platform := range platforms {
		args := []string{"--boot-directory=" + constants.BootMountPoint, "--efi-directory=" + constants.EFIMountPoint, "--removable"}

		if loopDevice {
			args = append(args, "--no-nvram")
		}

		if platform != "" {
			args = append(args, "--target="+platform)
		}

		args = append(args, blk)

		log.Printf("executing: grub-install %s", strings.Join(args, " "))

		cmd := exec.Command("grub-install", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("failed to install grub: %w", err)
		}
	}

	return nil
}

// Default implements the bootloader interface.
func (g *Grub) Default(label string) (err error) {
	var b []byte

	if b, err = ioutil.ReadFile(GrubConfig); err != nil {
		return err
	}

	re := regexp.MustCompile(`^set default="(.*)"`)
	b = re.ReplaceAll(b, []byte(fmt.Sprintf(`set default="%s"`, label)))

	log.Printf("writing %s to disk", GrubConfig)

	return ioutil.WriteFile(GrubConfig, b, 0o600)
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

	return ioutil.WriteFile(path, wr.Bytes(), 0o600)
}
