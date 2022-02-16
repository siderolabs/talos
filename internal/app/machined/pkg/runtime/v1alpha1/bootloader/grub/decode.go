// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
)

var (
	defaultEntryRegex  = regexp.MustCompile(`(?m)^\s*set default="(.*)"\s*$`)
	fallbackEntryRegex = regexp.MustCompile(`(?m)^\s*set fallback="(.*)"\s*$`)
	menuEntryRegex     = regexp.MustCompile(`(?m)^menuentry "(.+)" {([^}]+)}`)
	linuxRegex         = regexp.MustCompile(`(?m)^\s*linux\s+(.+?)\s+(.*)$`)
	initrdRegex        = regexp.MustCompile(`(?m)^\s*initrd\s+(.+)$`)
)

// Read reads the grub configuration from the disk.
func Read(path string) (*Config, error) {
	c, err := ioutil.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return Decode(c)
}

// Decode parses the grub configuration from the given bytes.
func Decode(c []byte) (*Config, error) {
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
