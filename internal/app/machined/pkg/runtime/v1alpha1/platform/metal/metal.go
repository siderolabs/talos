// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-smbios/smbios"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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
func (m *Metal) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	var option *string
	if option = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, errors.ErrNoConfigSource
	}

	if *option == constants.ConfigNone {
		return nil, errors.ErrNoConfigSource
	}

	downloadURL, err := PopulateURLParameters(ctx, *option, r, getSystemUUIDAndSerialNumber, getMACAddress, getHostname)
	if err != nil {
		log.Fatalf("failed to populate talos.config fetch URL: %q ; %s", *option, err.Error())

		return nil, err
	}

	log.Printf("fetching machine config from: %q", downloadURL)

	switch downloadURL {
	case constants.MetalConfigISOLabel:
		return readConfigFromISO()
	default:
		return download.Download(ctx, downloadURL)
	}
}

// PopulateURLParameters fills in empty parameters in the download URL.
//nolint:gocyclo
func PopulateURLParameters(ctx context.Context, downloadURL string, r state.State,
	getSystemUUIDAndSerialNumberFunc func() (string, string, error),
	getMACAddressFunc, getHostnameFunc func(ctx context.Context, r state.State) (string, error),
) (string, error) {
	populatedURL := downloadURL

	const uuidKey = "uuid"

	const serialNumberKey = "serial"

	const hostnameKey = "hostname"

	const macKey = "mac"

	uid, serialNumber, getUUIDAndSerialNumberErr := getSystemUUIDAndSerialNumberFunc()

	keyToVar := func(key string) string {
		return `${` + key + `}`
	}

	genErr := func(varOfKey string, errToWrap error) error {
		return fmt.Errorf("error while substituting %s: %w", varOfKey, errToWrap)
	}

	substituteSerialOrUUID := func(key, valToSubstitute string) error {
		varOfKey := keyToVar(key)
		if strings.Contains(populatedURL, varOfKey) {
			if getUUIDAndSerialNumberErr != nil {
				return genErr(varOfKey, getUUIDAndSerialNumberErr)
			}

			populatedURL = strings.ReplaceAll(populatedURL, varOfKey, valToSubstitute)
		}

		return nil
	}

	if err := substituteSerialOrUUID(uuidKey, uid); err != nil {
		return "", err
	}

	if err := substituteSerialOrUUID(serialNumberKey, serialNumber); err != nil {
		return "", err
	}

	substitute := func(key string, getFunc func(ctx context.Context, r state.State) (string, error)) error {
		varOfKey := keyToVar(key)
		if strings.Contains(populatedURL, varOfKey) {
			val, err := getFunc(context.Background(), r)
			if err != nil {
				return genErr(varOfKey, err)
			}

			populatedURL = strings.ReplaceAll(populatedURL, varOfKey, val)
		}

		return nil
	}

	if err := substitute(macKey, getMACAddressFunc); err != nil {
		return "", err
	}

	if err := substitute(hostnameKey, getHostnameFunc); err != nil {
		return "", err
	}

	u, err := url.Parse(populatedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", populatedURL, err)
	}

	values := u.Query()

	// Although the UUID can be substituted via the ${uuid} variable, we keep the behavior that the empty uuid= parameter is filled in with the UUID. This is backwards compatible.
	for key, qValues := range values {
		if key == uuidKey {
			if getUUIDAndSerialNumberErr != nil {
				return "", fmt.Errorf("error while substituting UUID: %w", getUUIDAndSerialNumberErr)
			}
			// don't touch uuid field if it already has some value
			if !(len(qValues) == 1 && len(strings.TrimSpace(qValues[0])) > 0) {
				values.Set(uuidKey, uid)
			}
		}
	}

	u.RawQuery = values.Encode()

	return u.String(), nil
}

func getResource(ctx context.Context, r state.State, namespace, typ string, checkAndGetFunc func(resource.Resource) string) (string, error) {
	metadata := resource.NewMetadata(namespace, typ, "", resource.VersionUndefined)

	list, err := r.List(ctx, metadata)
	if err != nil {
		return "", fmt.Errorf("failed to list %s resources: %w", typ, err)
	}

	for _, item := range list.Items {
		val := checkAndGetFunc(item)
		if val != "" {
			return val, nil
		}
	}

	watchCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	events := make(chan state.Event)

	err = r.WatchKind(watchCtx, metadata, events)
	if err != nil {
		return "", fmt.Errorf("failed to watch %s resources: %w", typ, err)
	}

	for {
		select {
		case <-watchCtx.Done():
			return "", fmt.Errorf("failed to determine %s: %w", typ, watchCtx.Err())
		case event := <-events:
			val := checkAndGetFunc(event.Resource)
			if val != "" {
				return val, nil
			}
		}
	}
}

func getSystemUUIDAndSerialNumber() (string, string, error) {
	s, err := smbios.New()
	if err != nil {
		return "", "", err
	}

	return s.SystemInformation.UUID, s.SystemInformation.SerialNumber, nil
}

func getAndCheckMACAddr(r resource.Resource) string {
	if resource.IsTombstone(r) {
		return ""
	}

	linkStatus := r.(*network.LinkStatus).TypedSpec() //nolint:forcetypeassert,errcheck
	if linkStatus != nil && linkStatus.LinkState {
		return linkStatus.HardwareAddr.String()
	}

	return ""
}

func getMACAddress(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, network.NamespaceName, network.LinkStatusType, getAndCheckMACAddr)
}

func getAndCheckHostname(r resource.Resource) string {
	if resource.IsTombstone(r) {
		return ""
	}

	return r.(*network.HostnameSpec).TypedSpec().Hostname //nolint:forcetypeassert,errcheck
}

func getHostname(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, network.NamespaceName, network.HostnameSpecType, getAndCheckHostname)
}

// Mode implements the platform.Platform interface.
func (m *Metal) Mode() runtime.Mode {
	return runtime.ModeMetal
}

func readConfigFromISO() ([]byte, error) {
	dev, err := probe.GetDevWithFileSystemLabel(constants.MetalConfigISOLabel)
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
		return nil, fmt.Errorf("error while substituting filesystem type")
	}

	if err = unix.Mount(dev.Device().Name(), mnt, sb.Type(), unix.MS_RDONLY, ""); err != nil {
		return nil, fmt.Errorf("failed to mount iso: %w", err)
	}

	b, err := ioutil.ReadFile(filepath.Join(mnt, filepath.Base(constants.ConfigPath)))
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
