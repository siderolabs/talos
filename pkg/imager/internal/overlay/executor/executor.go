// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package executor implements overlay.Installer
package executor

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"gopkg.in/yaml.v2"

	"github.com/siderolabs/talos/pkg/machinery/overlay"
)

var _ overlay.Installer = (*Options)(nil)

// Options executor options.
type Options struct {
	commandPath string
}

// New returns a new overlay installer executor.
func New(commandPath string) *Options {
	return &Options{
		commandPath: commandPath,
	}
}

// GetOptions returns the options for the overlay installer.
func (o *Options) GetOptions(extra overlay.InstallExtraOptions) (overlay.Options, error) {
	// parse extra as yaml
	extraYAML, err := yaml.Marshal(extra)
	if err != nil {
		return overlay.Options{}, fmt.Errorf("failed to marshal extra: %w", err)
	}

	out, err := o.execute(bytes.NewReader(extraYAML), "get-options")
	if err != nil {
		return overlay.Options{}, fmt.Errorf("failed to run overlay installer: %w", err)
	}

	var options overlay.Options

	if err := yaml.Unmarshal(out, &options); err != nil {
		return overlay.Options{}, fmt.Errorf("failed to unmarshal overlay options: %w", err)
	}

	return options, nil
}

// Install installs the overlay.
func (o *Options) Install(options overlay.InstallOptions) error {
	optionsBytes, err := yaml.Marshal(&options)
	if err != nil {
		return fmt.Errorf("failed to marshal options: %w", err)
	}

	if _, err := o.execute(bytes.NewReader(optionsBytes), "install"); err != nil {
		return fmt.Errorf("failed to run overlay installer: %w", err)
	}

	return nil
}

func (o *Options) execute(stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command(o.commandPath, args...)
	cmd.Stdin = stdin

	var stdOut, stdErr bytes.Buffer

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run overlay installer: %w, stdErr: %s", err, stdErr.Bytes())
	}

	return stdOut.Bytes(), nil
}
