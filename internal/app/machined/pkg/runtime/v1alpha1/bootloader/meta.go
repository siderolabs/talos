// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootloader

import (
	"fmt"
	"io"
	"os"

	"github.com/talos-systems/go-blockdevice/blockdevice/probe"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv/talos"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Meta represents the meta reader.
type Meta struct {
	*os.File
	LegacyADV adv.ADV
	ADV       adv.ADV
}

// NewMeta initializes and returns a `Meta`.
func NewMeta() (meta *Meta, err error) {
	var (
		f   *os.File
		dev *probe.ProbedBlockDevice
	)

	dev, err = probe.GetDevWithPartitionName(constants.MetaPartitionLabel)
	if err != nil {
		return nil, err
	}

	part, err := dev.OpenPartition(constants.MetaPartitionLabel)
	if err != nil {
		return nil, err
	}

	f = part.Device()

	adv, err := talos.NewADV(f)
	if adv == nil && err != nil {
		// if adv is not nil, but err is nil, it might be missing ADV, ignore it
		return nil, err
	}

	legacyAdv, err := syslinux.NewADV(f)
	if err != nil {
		return nil, err
	}

	return &Meta{
		File:      f,
		LegacyADV: legacyAdv,
		ADV:       adv,
	}, nil
}

func (m *Meta) Write() error {
	serialized, err := m.ADV.Bytes()
	if err != nil {
		return err
	}

	n, err := m.File.WriteAt(serialized, 0)
	if err != nil {
		return err
	}

	if n != len(serialized) {
		return fmt.Errorf("expected to write %d bytes, wrote %d", len(serialized), n)
	}

	serialized, err = m.LegacyADV.Bytes()
	if err != nil {
		return err
	}

	offset, err := m.File.Seek(-int64(len(serialized)), io.SeekEnd)
	if err != nil {
		return err
	}

	n, err = m.File.WriteAt(serialized, offset)
	if err != nil {
		return err
	}

	if n != len(serialized) {
		return fmt.Errorf("expected to write %d bytes, wrote %d", len(serialized), n)
	}

	return m.File.Sync()
}

// Revert reverts the default bootloader label to the previous installation.
func (m *Meta) Revert() (err error) {
	label, ok := m.LegacyADV.ReadTag(adv.Upgrade)
	if !ok {
		return nil
	}

	if label == "" {
		m.LegacyADV.DeleteTag(adv.Upgrade)

		return m.Write()
	}

	g := &grub.Grub{}

	if err = g.Default(label); err != nil {
		return err
	}

	m.LegacyADV.DeleteTag(adv.Upgrade)

	return m.Write()
}
