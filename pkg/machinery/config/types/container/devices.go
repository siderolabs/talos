// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// devicePathPrefix is the directory every device destination is under.
const devicePathPrefix = "/dev/"

// reservedDeviceNames are the entries of the container's /dev the runtime populates itself.
var reservedDeviceNames = []string{
	"console",
	"core",
	"fd",
	"full",
	"mqueue",
	"null",
	"ptmx",
	"pts",
	"random",
	"shm",
	"stderr",
	"stdin",
	"stdout",
	"tty",
	"urandom",
	"zero",
}

// ContainerDevice describes a single device to hand to the container.
//
// Exactly one source must be set.
type ContainerDevice struct {
	//   description: |
	//     Hand a raw volume to the container as a block device.
	//
	//     The raw volume is referenced by the name of its `RawVolumeConfig` document.
	//     The container gets a device node at the destination and is allowed to read and write
	//     that device, and no other block device. This works with the `restricted` profile,
	//     no capabilities are added. For an encrypted raw volume the node is the unlocked device.
	//
	//     Declaring this device makes the container wait for the raw volume to be ready, and the
	//     raw volume is not closed while the container is running. A raw volume can be handed to
	//     a single container only.
	RawVolumeDevice *RawVolumeDevice `yaml:"rawVolume,omitempty"`
}

// RawVolumeDevice hands a raw volume to the container by name.
type RawVolumeDevice struct {
	//   description: |
	//     Name of the `RawVolumeConfig` document.
	VolumeName string `yaml:"name"`
	//   description: |
	//     Path of the device node inside the container.
	//
	//     Must be under `/dev/`, and not one of the entries the container runtime creates there
	//     itself (e.g. `/dev/null`, `/dev/pts`). The node keeps the owner and mode of the device
	//     on the host.
	//   examples:
	//     - value: '"/dev/data"'
	DeviceDestination string `yaml:"destination"`
}

// Validate checks the device and returns its destination.
func (d *ContainerDevice) Validate() (string, error) {
	if d.RawVolumeDevice == nil {
		return "", errors.New("exactly one of rawVolume must be set")
	}

	return d.RawVolumeDevice.DeviceDestination, d.RawVolumeDevice.Validate()
}

// Validate checks the raw volume device.
func (d *RawVolumeDevice) Validate() error {
	var validationErrors error

	if d.VolumeName == "" {
		validationErrors = errors.Join(validationErrors, errors.New("rawVolume.name is required"))
	}

	return errors.Join(
		validationErrors,
		ValidateDeviceDestination("rawVolume.destination", d.DeviceDestination),
	)
}

// ValidateDeviceDestination checks the path of a device node inside the container.
func ValidateDeviceDestination(kind, path string) error {
	if err := ValidateAbsPath(kind, path); err != nil {
		return err
	}

	if filepath.Clean(path) != path {
		return fmt.Errorf("%s %q must be a clean path, i.e. %q", kind, path, filepath.Clean(path))
	}

	name, isDevice := strings.CutPrefix(path, devicePathPrefix)
	if !isDevice {
		return fmt.Errorf("%s %q must be under %q", kind, path, devicePathPrefix)
	}

	if first, _, _ := strings.Cut(name, "/"); slices.Contains(reservedDeviceNames, first) {
		return fmt.Errorf("%s %q is reserved by the container runtime", kind, path)
	}

	return nil
}

// pathsOverlap reports whether one of the clean absolute paths is the other one, or is under it.
func pathsOverlap(a, b string) bool {
	return a == b || isUnder(a, b) || isUnder(b, a)
}

// isUnder reports whether the clean absolute path is strictly under the clean absolute dir.
func isUnder(path, dir string) bool {
	if dir == "/" {
		return path != "/"
	}

	return strings.HasPrefix(path, dir+"/")
}

// Check interfaces.
var (
	_ config.ContainerDeviceConfig          = &ContainerDevice{}
	_ config.ContainerRawVolumeDeviceConfig = &RawVolumeDevice{}
)

// RawVolume implements config.ContainerDeviceConfig interface.
func (d *ContainerDevice) RawVolume() optional.Optional[config.ContainerRawVolumeDeviceConfig] {
	if d.RawVolumeDevice == nil {
		return optional.None[config.ContainerRawVolumeDeviceConfig]()
	}

	return optional.Some[config.ContainerRawVolumeDeviceConfig](d.RawVolumeDevice)
}

// Name implements config.ContainerRawVolumeDeviceConfig interface.
func (d *RawVolumeDevice) Name() string { return d.VolumeName }

// Destination implements config.ContainerRawVolumeDeviceConfig interface.
func (d *RawVolumeDevice) Destination() string { return d.DeviceDestination }
