// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// BootLabel represents a boot label, e.g. A or B.
type BootLabel string

const (
	amd64 = "amd64"
	arm64 = "arm64"

	confTemplate = `set default="{{ (index .Entries .Default).Name }}"
{{ with (index .Entries .Fallback).Name -}}
set fallback="{{ . }}"
{{- end }}
set timeout=3

insmod all_video

terminal_input console
terminal_output console

{{ range $key, $entry := .Entries -}}
menuentry "{{ $entry.Name }}" {
  set gfxmode=auto
  set gfxpayload=text
  linux {{ $entry.Linux }} {{ $entry.Cmdline }}
  initrd {{ $entry.Initrd }}
}
{{ end -}}
`
)

var (
	defaultEntryRegex  = regexp.MustCompile(`(?m)^\s*set default="(.*)"\s*$`)
	fallbackEntryRegex = regexp.MustCompile(`(?m)^\s*set fallback="(.*)"\s*$`)
	menuEntryRegex     = regexp.MustCompile(`(?m)^menuentry "(.+)" {([^}]+)}`)
	linuxRegex         = regexp.MustCompile(`(?m)^\s*linux\s+(.+?)\s+(.*)$`)
	initrdRegex        = regexp.MustCompile(`(?m)^\s*initrd\s+(.+)$`)
)

// Config represents a grub configuration file (grub.cfg).
type Config struct {
	Default  BootLabel
	Fallback BootLabel
	Entries  map[BootLabel]MenuEntry
}

// MenuEntry represents a grub menu entry in the grub config file.
type MenuEntry struct {
	Name    string
	Linux   string
	Cmdline string
	Initrd  string
}

// NewConfig creates a new grub configuration (nothing is written to disk).
func NewConfig(cmdline string) *Config {
	return &Config{
		Default: BootA,
		Entries: map[BootLabel]MenuEntry{
			BootA: *buildMenuEntry(BootA, cmdline),
		},
	}
}

// Put puts a new menu entry to the grub config (nothing is written to disk).
func (c *Config) Put(entry BootLabel, cmdline string) error {
	c.Entries[entry] = *buildMenuEntry(entry, cmdline)

	return nil
}

// ReadFromDisk reads the grub configuration from the disk.
func ReadFromDisk() (*Config, error) {
	c, err := ioutil.ReadFile(GrubConfig)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return ParseBytes(c)
}

// ParseBytes parses the grub configuration from the given bytes.
func ParseBytes(c []byte) (*Config, error) {
	defaultEntryMatches := defaultEntryRegex.FindAllSubmatch(c, -1)
	if len(defaultEntryMatches) != 1 {
		return nil, fmt.Errorf("failed to find default")
	}

	fallbackEntryMatches := fallbackEntryRegex.FindAllSubmatch(c, -1)
	if len(fallbackEntryMatches) > 1 {
		return nil, fmt.Errorf("found multiple fallback entries")
	}

	var fallbackEntry BootLabel

	if len(fallbackEntryMatches) == 1 {
		if len(fallbackEntryMatches[0]) != 2 {
			return nil, fmt.Errorf("failed to parse fallback entry")
		}

		entry, err := ParseBootLabel(string(fallbackEntryMatches[0][1]))
		if err != nil {
			return nil, err
		}

		fallbackEntry = entry
	}

	if len(defaultEntryMatches[0]) != 2 {
		return nil, fmt.Errorf("expected 2 matches, got %d", len(defaultEntryMatches[0]))
	}

	defaultEntry, err := ParseBootLabel(string(defaultEntryMatches[0][1]))
	if err != nil {
		return nil, err
	}

	entries, err := parseEntries(c)
	if err != nil {
		return nil, err
	}

	conf := Config{
		Default:  defaultEntry,
		Fallback: fallbackEntry,
		Entries:  entries,
	}

	return &conf, nil
}

func parseEntries(conf []byte) (map[BootLabel]MenuEntry, error) {
	entries := make(map[BootLabel]MenuEntry)

	matches := menuEntryRegex.FindAllSubmatch(conf, -1)
	for _, m := range matches {
		if len(m) != 3 {
			return nil, fmt.Errorf("expected 3 matches, got %d", len(m))
		}

		confBlock := m[2]

		linux, cmdline, initrd, err := parseConfBlock(confBlock)
		if err != nil {
			return nil, err
		}

		name := string(m[1])

		bootEntry, err := ParseBootLabel(name)
		if err != nil {
			return nil, err
		}

		entries[bootEntry] = MenuEntry{
			Name:    name,
			Linux:   linux,
			Cmdline: cmdline,
			Initrd:  initrd,
		}
	}

	return entries, nil
}

func parseConfBlock(block []byte) (linux, cmdline, initrd string, err error) {
	linuxMatches := linuxRegex.FindAllSubmatch(block, -1)
	if len(linuxMatches) != 1 {
		return "", "", "",
			fmt.Errorf("expected 1 match, got %d", len(linuxMatches))
	}

	if len(linuxMatches[0]) != 3 {
		return "", "", "",
			fmt.Errorf("expected 3 matches, got %d", len(linuxMatches[0]))
	}

	linux = string(linuxMatches[0][1])
	cmdline = string(linuxMatches[0][2])

	initrdMatches := initrdRegex.FindAllSubmatch(block, -1)
	if len(initrdMatches) != 1 {
		return "", "", "",
			fmt.Errorf("expected 1 match, got %d", len(initrdMatches))
	}

	if len(initrdMatches[0]) != 2 {
		return "", "", "",
			fmt.Errorf("expected 2 matches, got %d", len(initrdMatches[0]))
	}

	initrd = string(initrdMatches[0][1])

	return linux, cmdline, initrd, nil
}

func (c *Config) validate() error {
	if _, ok := c.Entries[c.Default]; !ok {
		return fmt.Errorf("invalid default entry: %s", c.Default)
	}

	if c.Fallback != "" {
		if _, ok := c.Entries[c.Fallback]; !ok {
			return fmt.Errorf("invalid fallback entry: %s", c.Fallback)
		}
	}

	if c.Default == c.Fallback {
		return fmt.Errorf("default and fallback entries must not be the same")
	}

	return nil
}

// WriteToFile writes the grub configuration to the given file.
func (c *Config) WriteToFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return err
	}

	wr := new(bytes.Buffer)

	err := c.Write(wr)
	if err != nil {
		return err
	}

	log.Printf("writing %s to disk", path)

	return ioutil.WriteFile(path, wr.Bytes(), 0o600)
}

// Write writes the grub configuration to the given writer.
func (c *Config) Write(wr io.Writer) error {
	if err := c.validate(); err != nil {
		return err
	}

	t := template.Must(template.New("grub").Parse(confTemplate))

	return t.Execute(wr, c)
}

// Install validates the grub configuration and writes it to the disk.
//nolint:gocyclo
func (c *Config) Install(bootDisk, arch string) error {
	if err := c.WriteToFile(GrubConfig); err != nil {
		return err
	}

	blk, err := getBlockDeviceName(bootDisk)
	if err != nil {
		return err
	}

	loopDevice := strings.HasPrefix(blk, "/dev/loop")

	var platforms []string

	switch arch {
	case amd64:
		platforms = []string{"x86_64-efi", "i386-pc"}
	case arm64:
		platforms = []string{"arm64-efi"}
	}

	if goruntime.GOARCH == amd64 && arch == amd64 && !loopDevice {
		// let grub choose the platform automatically if not building an image
		platforms = []string{""}
	}

	for _, platform := range platforms {
		args := []string{"--boot-directory=" + constants.BootMountPoint, "--efi-directory=" +
			constants.EFIMountPoint, "--removable"}

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

func getBlockDeviceName(bootDisk string) (string, error) {
	dev, err := blockdevice.Open(bootDisk)
	if err != nil {
		return "", err
	}

	//nolint:errcheck
	defer dev.Close()

	// verify that BootDisk has boot partition
	_, err = dev.GetPartition(constants.BootPartitionLabel)
	if err != nil {
		return "", err
	}

	blk := dev.Device().Name()

	return blk, nil
}

// FlipBootLabel flips the boot entry, e.g. A -> B, B -> A.
func FlipBootLabel(e BootLabel) (BootLabel, error) {
	switch e {
	case BootA:
		return BootB, nil
	case BootB:
		return BootA, nil
	default:
		return "", fmt.Errorf("invalid entry: %s", e)
	}
}

// ParseBootLabel parses the given human-readable boot label to a grub.BootLabel.
func ParseBootLabel(name string) (BootLabel, error) {
	if strings.HasPrefix(name, string(BootA)) {
		return BootA, nil
	}

	if strings.HasPrefix(name, string(BootB)) {
		return BootB, nil
	}

	return "", fmt.Errorf("could not parse boot entry from name: %s", name)
}

func buildMenuEntry(entry BootLabel, cmdline string) *MenuEntry {
	return &MenuEntry{
		Name:    fmt.Sprintf("%s - %s", entry, version.Short()),
		Linux:   filepath.Join("/", string(BootA), constants.KernelAsset),
		Cmdline: cmdline,
		Initrd:  filepath.Join("/", string(BootA), constants.InitramfsAsset),
	}
}
