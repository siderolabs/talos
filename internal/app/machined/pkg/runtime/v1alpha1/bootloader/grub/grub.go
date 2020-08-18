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
	"github.com/talos-systems/talos/pkg/cmd"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

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

const grubCfgTpl = `{{ range $label := .Labels -}}
menuentry "{{ $label.Root }}" {
  linux {{ $label.Kernel }} {{ $label.Append }}
  initrd {{ $label.Initrd }}
}
{{- end }}
`

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

	re := regexp.MustCompile(`^menuentry\s"(.*)"`)
	matches := re.FindAllSubmatch(b, 2)

	if len(matches) == 0 {
		return "", "", fmt.Errorf("expected at least one menuentry, got 0")
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

// Prepare implements the Bootloader interface. It works by writing
// gptmbr.bin to a block device.
func (g *Grub) Prepare(dev string) (err error) {
	return nil
}

// Install implements the Bootloader interface. It sets up syslinux with the
// specified kernel parameters.
//
// nolint: gocyclo
func (g *Grub) Install(fallback string, config interface{}, sequence runtime.Sequence, bootPartitionFound bool) (err error) {
	grubcfg, ok := config.(*Cfg)
	if !ok {
		return errors.New("expected a grub config")
	}

	if err = writeCfg(constants.BootMountPoint, GrubConfig, grubcfg); err != nil {
		return err
	}

	dev, err := probe.GetDevWithFileSystemLabel(constants.BootPartitionLabel)
	if err != nil {
		return fmt.Errorf("failed to probe boot partition: %w", err)
	}

	// nolint: errcheck
	defer dev.Close()

	log.Printf("installing grub to %s", dev.Path)

	if _, err = cmd.Run("grub-install", "--force", "--boot-directory="+constants.BootMountPoint, dev.Path); err != nil {
		return fmt.Errorf("failed to install grub: %w", err)
	}

	return nil
}

func writeCfg(base, path string, grubcfg *Cfg) (err error) {
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
