// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package iso contains functions for creating ISO images.
package iso

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// VolumeID returns a valid volume ID for the given label.
func VolumeID(label string) string {
	// builds a valid volume ID: 32 chars out of [A-Z0-9_]
	label = strings.ToUpper(label)
	label = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_' || r == '-' || r == '.':
			return '_'
		default:
			return -1
		}
	}, label)

	if len(label) > 32 {
		label = label[:32]
	}

	return label
}

// Label returns an ISO full label for a given version.
func Label(version string, secureboot bool) string {
	label := "Talos-"

	if secureboot {
		label += "SB-"
	}

	return label + version
}

// ExecutorOptions defines the iso generation options.
type ExecutorOptions struct {
	Command   string
	Version   string
	Arguments []string
}

// Generator is an interface for executing the iso generation.
type Generator interface {
	Generate(ctx context.Context) error
}

// Options describe the input generating different types of ISOs.
type Options struct {
	KernelPath    string
	InitramfsPath string
	Cmdline       string

	UKIPath    string
	SDBootPath string

	Arch    string
	Version string

	// A value in loader.conf secure-boot-enroll: off, manual, if-safe, force.
	SDBootSecureBootEnrollKeys string

	// UKISigningCertDer is the DER encoded UKI signing certificate.
	UKISigningCertDerPath string

	// optional, for auto-enrolling secureboot keys
	PlatformKeyPath    string
	KeyExchangeKeyPath string
	SignatureKeyPath   string

	ScratchDir string
	OutPath    string
}

// Generate creates an ISO image.
func (e *ExecutorOptions) Generate(ctx context.Context) error {
	if epoch, ok, err := utils.SourceDateEpoch(); err != nil {
		return err
	} else if ok {
		// set EFI FAT image serial number
		if err := os.Setenv("GRUB_FAT_SERIAL_NUMBER", fmt.Sprintf("%x", uint32(epoch))); err != nil {
			return err
		}

		e.Arguments = append(e.Arguments,
			"-volume_date", "all_file_dates", fmt.Sprintf("=%d", epoch),
			"-volume_date", "uuid", time.Unix(epoch, 0).Format("2006010215040500"),
		)
	}

	if quirks.New(e.Version).SupportsISOLabel() {
		label := Label(e.Version, false)

		e.Arguments = append(e.Arguments,
			"-volid", VolumeID(label),
			"-volset-id", label,
		)
	}

	_, err := cmd.RunWithOptions(ctx, e.Command, e.Arguments)
	if err != nil {
		return fmt.Errorf("failed to create ISO: %w", err)
	}

	return nil
}
