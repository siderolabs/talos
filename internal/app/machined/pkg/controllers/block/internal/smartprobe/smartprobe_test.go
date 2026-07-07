// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package smartprobe_test

import (
	"errors"
	"testing"

	smart "github.com/anatol/smart.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/smartprobe"
)

// fakeSataDevice implements the sataDevice interface without touching real hardware, so the
// standby spin-up-avoidance gate in smartprobe.Prober.Probe can be exercised directly.
type fakeSataDevice struct {
	powerMode    byte
	powerModeErr error
	smartErr     error

	checkPowerModeCalls int
	readSMARTDataCalls  int
}

func (d *fakeSataDevice) Type() string { return "sata" }

func (d *fakeSataDevice) Close() error { return nil }

func (d *fakeSataDevice) ReadGenericAttributes() (*smart.GenericAttributes, error) {
	return &smart.GenericAttributes{}, nil
}

func (d *fakeSataDevice) CheckPowerMode() (byte, error) {
	d.checkPowerModeCalls++

	return d.powerMode, d.powerModeErr
}

func (d *fakeSataDevice) ReadSMARTData() (*smart.AtaSmartPage, error) {
	d.readSMARTDataCalls++

	if d.smartErr != nil {
		return nil, d.smartErr
	}

	return &smart.AtaSmartPage{}, nil
}

func (d *fakeSataDevice) ReadSMARTThresholds() (*smart.AtaSmartThresholdsPage, error) {
	return &smart.AtaSmartThresholdsPage{}, nil
}

// fakeNVMeDevice implements the nvmeDevice interface without touching real hardware.
type fakeNVMeDevice struct {
	err error

	readSMARTCalls int
}

func (d *fakeNVMeDevice) Type() string { return "nvme" }

func (d *fakeNVMeDevice) Close() error { return nil }

func (d *fakeNVMeDevice) ReadGenericAttributes() (*smart.GenericAttributes, error) {
	return &smart.GenericAttributes{}, nil
}

func (d *fakeNVMeDevice) ReadSMART() (*smart.NvmeSMARTLog, error) {
	d.readSMARTCalls++

	if d.err != nil {
		return nil, d.err
	}

	return &smart.NvmeSMARTLog{}, nil
}

func TestSmartGoProberSATAStandbyNotSpunUp(t *testing.T) {
	t.Parallel()

	dev := &fakeSataDevice{powerMode: smart.PowerModeStandby}

	prober := smartprobe.Prober{Open: func(string) (smart.Device, error) { return dev, nil }}

	spec, standby, err := prober.Probe("/dev/sda", true)
	require.NoError(t, err)

	assert.True(t, standby)
	assert.Equal(t, "standby", spec.PowerState)
	assert.Equal(t, 1, dev.checkPowerModeCalls)
	assert.Equal(t, 0, dev.readSMARTDataCalls, "a standby rotational disk must not be spun up to read SMART data")
}

func TestSmartGoProberSATAActiveIsProbed(t *testing.T) {
	t.Parallel()

	dev := &fakeSataDevice{powerMode: smart.PowerModeActive}

	prober := smartprobe.Prober{Open: func(string) (smart.Device, error) { return dev, nil }}

	spec, standby, err := prober.Probe("/dev/sda", true)
	require.NoError(t, err)

	assert.False(t, standby)
	assert.Equal(t, "active", spec.PowerState)
	assert.Equal(t, 1, dev.checkPowerModeCalls)
	assert.Equal(t, 1, dev.readSMARTDataCalls)
}

func TestSmartGoProberSATAPowerModeErrorFallsThroughToProbe(t *testing.T) {
	t.Parallel()

	dev := &fakeSataDevice{powerModeErr: errors.New("check power mode: not supported")}

	prober := smartprobe.Prober{Open: func(string) (smart.Device, error) { return dev, nil }}

	_, standby, err := prober.Probe("/dev/sda", true)
	require.NoError(t, err)

	assert.False(t, standby)
	assert.Equal(t, 1, dev.checkPowerModeCalls)
	assert.Equal(t, 1, dev.readSMARTDataCalls, "when power mode can't be determined, probing should proceed rather than silently skip")
}

func TestSmartGoProberSATANonRotationalSkipsPowerModeGate(t *testing.T) {
	t.Parallel()

	dev := &fakeSataDevice{powerMode: smart.PowerModeStandby}

	prober := smartprobe.Prober{Open: func(string) (smart.Device, error) { return dev, nil }}

	_, standby, err := prober.Probe("/dev/sda", false)
	require.NoError(t, err)

	assert.False(t, standby)
	assert.Equal(t, 0, dev.checkPowerModeCalls, "non-rotational disks must never be power-mode gated")
	assert.Equal(t, 1, dev.readSMARTDataCalls)
}

func TestSmartGoProberNVMeIgnoresRotationalFlag(t *testing.T) {
	t.Parallel()

	for _, rotational := range []bool{true, false} {
		dev := &fakeNVMeDevice{}

		prober := smartprobe.Prober{Open: func(string) (smart.Device, error) { return dev, nil }}

		_, standby, err := prober.Probe("/dev/nvme0n1", rotational)
		require.NoError(t, err)

		assert.False(t, standby)
		assert.Equal(t, 1, dev.readSMARTCalls)
	}
}

func TestSmartGoProberOpenError(t *testing.T) {
	t.Parallel()

	openErr := errors.New("open: no such device")

	prober := smartprobe.Prober{Open: func(string) (smart.Device, error) { return nil, openErr }}

	_, standby, err := prober.Probe("/dev/sda", true)

	assert.ErrorIs(t, err, openErr)
	assert.False(t, standby)
}
