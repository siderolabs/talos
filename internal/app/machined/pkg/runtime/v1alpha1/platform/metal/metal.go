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
	"regexp"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/probe"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	hardwareResource "github.com/talos-systems/talos/pkg/machinery/resources/hardware"
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

	downloadURL, err := PopulateURLParameters(ctx, *option, r)
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
func PopulateURLParameters(ctx context.Context, downloadURL string, r state.State) (string, error) {
	populatedURL := downloadURL

	keyToVar := func(key string) string {
		return `${` + key + `}`
	}

	genErr := func(varOfKey string, errToWrap error) error {
		return fmt.Errorf("error while substituting %s: %w", varOfKey, errToWrap)
	}

	u, err := url.Parse(populatedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", populatedURL, err)
	}

	values := u.Query()

	substitute := func(key string, getFunc func(ctx context.Context, r state.State) (string, error)) error {
		for qKey, qValues := range values {
			if len(qValues) > 0 {
				qVal := qValues[0]

				varOfKey := keyToVar(key)
				varOfKeyExpr := regexp.QuoteMeta(varOfKey)
				matcher := regexp.MustCompile(`(?i)` + varOfKeyExpr)

				if matcher.MatchString(qVal) {
					val, err := getFunc(context.Background(), r)
					if err != nil {
						return genErr(varOfKey, err)
					}

					qVal = matcher.ReplaceAllLiteralString(qVal, val)

					values.Set(qKey, qVal)
				}
			}
		}

		return nil
	}

	if err := substitute(constants.UUIDKey, getSystemUUID); err != nil {
		return "", err
	}

	if err := substitute(constants.SerialNumberKey, getSystemSerialNumber); err != nil {
		return "", err
	}

	if err := substitute(constants.MacKey, getMACAddress); err != nil {
		return "", err
	}

	if err := substitute(constants.HostnameKey, getHostname); err != nil {
		return "", err
	}

	// Although the UUID can be substituted via the ${uuid} variable, we keep the behavior that the empty uuid= parameter is filled in with the UUID. This is backwards compatible.
	for key, qValues := range values {
		if key == constants.UUIDKey { // don't touch uuid field if it already has some value
			if !(len(qValues) == 1 && len(strings.TrimSpace(qValues[0])) > 0) {
				uid, err := getSystemUUID(ctx, r)
				if err != nil {
					return "", fmt.Errorf("error while substituting UUID: %w", err)
				}

				values.Set(constants.UUIDKey, uid)
			}
		}
	}

	u.RawQuery = values.Encode()

	return u.String(), nil
}

func getResource[T resource.Resource](ctx context.Context, r state.State, namespace, typ string, checkAndGetFunc func(T) string) (string, error) {
	metadata := resource.NewMetadata(namespace, typ, "", resource.VersionUndefined)

	watchCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	events := make(chan safe.WrappedStateEvent[T])

	err := safe.StateWatchKind[T](watchCtx, r, metadata, events, state.WithBootstrapContents(true))
	if err != nil {
		return "", fmt.Errorf("failed to watch %s resources: %w", typ, err)
	}

	var watchErr error

	for {
		select {
		case <-watchCtx.Done():
			err := fmt.Errorf("failed to determine %s: %w", typ, watchCtx.Err())
			err = fmt.Errorf("%s; %w", err.Error(), watchErr)

			return "", err
		case event := <-events:
			eventResource, err := event.Resource()
			if err != nil {
				watchErr = fmt.Errorf("%s; invalid resource in wrapped event: %w", watchErr.Error(), err)
			}

			val := checkAndGetFunc(eventResource)
			if val != "" {
				return val, nil
			}
		}
	}
}

func getAndCheckUUID(r *hardwareResource.SystemInformation) string {
	sysInfo := r.TypedSpec()
	if sysInfo != nil {
		return sysInfo.UUID
	}

	return ""
}

func getAndCheckSerialNumber(r *hardwareResource.SystemInformation) string {
	sysInfo := r.TypedSpec()
	if sysInfo != nil {
		return sysInfo.SerialNumber
	}

	return ""
}

func getSystemUUID(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, hardwareResource.NamespaceName, hardwareResource.SystemInformationType, getAndCheckUUID)
}

func getSystemSerialNumber(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, hardwareResource.NamespaceName, hardwareResource.SystemInformationType, getAndCheckSerialNumber)
}

func getAndCheckMACAddr(r *network.LinkStatus) string {
	linkStatus := r.TypedSpec()
	if linkStatus != nil && linkStatus.LinkState {
		return linkStatus.HardwareAddr.String()
	}

	return ""
}

func getMACAddress(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, network.NamespaceName, network.LinkStatusType, getAndCheckMACAddr)
}

func getAndCheckHostname(r *network.HostnameSpec) string {
	sysInfo := r.TypedSpec()
	if sysInfo != nil {
		return sysInfo.Hostname
	}

	return ""
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
