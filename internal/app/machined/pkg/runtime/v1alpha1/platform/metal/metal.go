// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-smbios/smbios"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
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
func (m *Metal) Configuration(ctx context.Context) ([]byte, error) {
	var option *string
	if option = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, errors.ErrNoConfigSource
	}

	if *option == constants.ConfigNone {
		return nil, errors.ErrNoConfigSource
	}

	log.Printf("fetching machine config from: %q", *option)

	downloadURL, err := PopulateURLParameters(*option, getSystemUUID)
	if err != nil {
		return nil, err
	}

	switch downloadURL {
	case constants.MetalConfigISOLabel:
		return readConfigFromISO()
	default:
		return download.Download(ctx, downloadURL)
	}
}

type MachineConfigUrlTemplate struct {
	UUID     string
	MAC      string
	Hostname string
}

// PopulateURLParameters fills in empty parameters in the download URL.
func PopulateURLParameters(downloadURL string, getSystemUUID func() (string, error)) (string, error) {
	// first, do a templating of the downloadURL.  Then finally add a uuid if not set
	tmpl, templateErr := template.New("config-url-template").Parse(downloadURL)
	uid, uuidError := getSystemUUID()
	if templateErr != nil {
		log.Printf("failed to parse downloadURL: #{templateErr}")
	} else if uuidError != nil {
		log.Printf("failed to generate system uuid: #{uuidError}")
	} else {
		var urlTemplateResult bytes.Buffer

		data := getMachineConfigSubstitutions(uid)
		if err := tmpl.Execute(&urlTemplateResult, data); err != nil {
			log.Printf("failed to templatize downloadURL: #{err}")
		}
		downloadURL = urlTemplateResult.String()
	}

	u, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", constants.KernelParamConfig, err)
	}

	values := u.Query()

	for key, qValues := range values {
		switch key {
		case "uuid":
			// don't touch uuid field if it already has some value
			if !(len(qValues) == 1 && len(strings.TrimSpace(qValues[0])) > 0) {
				values.Set("uuid", uid)
			}
		default:
			log.Printf("unsupported query parameter: %q", key)
		}
	}

	u.RawQuery = values.Encode()

	return u.String(), nil
}

func getMacAddr() (string, error) {
	ifen0, en0Err := net.InterfaceByName("en0")
	if ifen0 != nil && en0Err == nil {
		return ifen0.HardwareAddr.String(), nil
	}
	ifEth0, eth0Err := net.InterfaceByName("eth0")
	if ifEth0 != nil && eth0Err == nil {
		return ifEth0.HardwareAddr.String(), nil
	}

	ifas, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a != "" {
			return a, nil
		}
	}
	return "", nil
}

func getMachineConfigSubstitutions(uid string) MachineConfigUrlTemplate {
	tplData := MachineConfigUrlTemplate{
		UUID: uid,
	}
	name, err := os.Hostname()
	if err == nil {
		tplData.Hostname = name
	}
	macAddress, errMac := getMacAddr()
	if errMac == nil && macAddress != "" {
		tplData.MAC = macAddress
		// TODO: represent secondary interfaces on MAC_0, MAC_1, etc.
	}
	return tplData
}

func getSystemUUID() (string, error) {
	s, err := smbios.New()
	if err != nil {
		return "", err
	}

	return s.SystemInformation.UUID, nil
}

// Mode implements the platform.Platform interface.
func (m *Metal) Mode() runtime.Mode {
	return runtime.ModeMetal
}

func readConfigFromISO() (b []byte, err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(constants.MetalConfigISOLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to find %s iso: %w", constants.MetalConfigISOLabel, err)
	}

	//nolint:errcheck
	defer dev.Close()

	sb, err := filesystem.Probe(dev.Device().Name())
	if err != nil {
		return nil, err
	}

	if sb == nil {
		return nil, fmt.Errorf("failed to get filesystem type")
	}

	if err = unix.Mount(dev.Device().Name(), mnt, sb.Type(), unix.MS_RDONLY, ""); err != nil {
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
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0").Append("tty0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (m *Metal) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	return nil
}
