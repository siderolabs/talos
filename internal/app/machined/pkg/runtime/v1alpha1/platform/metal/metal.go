// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-smbios/smbios"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const (
	mnt = "/mnt"
)

// Metal is a discoverer for non-cloud environments.
type Metal struct{}

// Name implements the platform.Platform interface.
func (m *Metal) Name() string {
	return "metal"
}

// Configuration implements the platform.Platform interface.
//
// nolint: gocyclo
func (m *Metal) Configuration() ([]byte, error) {
	var option *string
	if option = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, fmt.Errorf("no config option was found")
	}

	log.Printf("fetching machine config from: %q", *option)

	u, err := url.Parse(*option)
	if err != nil {
		return nil, err
	}

	values := u.Query()

	if len(values) > 0 {
		for key := range values {
			switch key {
			case "uuid":
				s, err := smbios.New()
				if err != nil {
					return nil, err
				}

				uuid, err := s.SystemInformation().UUID()
				if err != nil {
					return nil, err
				}

				values.Set("uuid", uuid.String())
			default:
				log.Printf("unsupported query parameter: %q", key)
			}
		}

		u.RawQuery = values.Encode()

		*option = u.String()
	}

	switch *option {
	case constants.MetalConfigISOLabel:
		return readConfigFromISO()
	default:
		return download.Download(*option)
	}
}

// Hostname implements the platform.Platform interface.
func (m *Metal) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// Mode implements the platform.Platform interface.
func (m *Metal) Mode() runtime.Mode {
	return runtime.ModeMetal
}

// ExternalIPs implements the platform.Platform interface.
func (m *Metal) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}

func readConfigFromISO() (b []byte, err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(constants.MetalConfigISOLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to find %s iso: %w", constants.MetalConfigISOLabel, err)
	}

	// nolint: errcheck
	defer dev.Close()

	if err = unix.Mount(dev.Path, mnt, dev.SuperBlock.Type(), unix.MS_RDONLY, ""); err != nil {
		return nil, fmt.Errorf("failed to mount iso: %w", err)
	}

	b, err = ioutil.ReadFile(filepath.Join(mnt, filepath.Base(constants.ConfigPath)))
	if err != nil {
		return nil, fmt.Errorf("read config: %s", err.Error())
	}

	if err = unix.Unmount(mnt, 0); err != nil {
		return nil, fmt.Errorf("failed to unmount: %w", err)
	}

	return b, nil
}

// KernelArgs implements the runtime.Platform interface.
func (m *Metal) KernelArgs() procfs.Parameters {
	return nil
}
