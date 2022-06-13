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

func keyToVar(key string) string {
	return `${` + key + `}`
}

type matcher struct {
	Key    string
	Regexp *regexp.Regexp
}

func newMatcher(key string) *matcher {
	return &matcher{
		Key:    keyToVar(key),
		Regexp: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(keyToVar(key))),
	}
}

type replacer struct {
	original string
	Regexp   *regexp.Regexp
	Matches  [][]int
}

func (m *matcher) process(original string) *replacer {
	var r replacer
	r.Regexp = m.Regexp
	r.original = original

	r.Matches = m.Regexp.FindAllStringIndex(original, -1)

	return &r
}

func (r *replacer) ReplaceMatches(replacement string) string {
	var res string

	if len(r.Matches) < 1 {
		return res
	}

	res += r.original[:r.Matches[0][0]]
	res += replacement

	for i := 0; i < len(r.Matches)-1; i++ {
		res += r.original[r.Matches[i][1]:r.Matches[i+1][0]]
		res += replacement
	}

	res += r.original[r.Matches[len(r.Matches)-1][1]:]

	return res
}

// PopulateURLParameters fills in empty parameters in the download URL.
//nolint:gocyclo
func PopulateURLParameters(ctx context.Context, downloadURL string, r state.State) (string, error) {
	populatedURL := downloadURL

	genErr := func(varOfKey string, errToWrap error) error {
		return fmt.Errorf("error while substituting %s: %w", varOfKey, errToWrap)
	}

	u, err := url.Parse(populatedURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", populatedURL, err)
	}

	values := u.Query()

	substitute := func(varToSubstitute string, getFunc func(ctx context.Context, r state.State) (string, error)) error {
		m := newMatcher(varToSubstitute)

		for qKey, qValues := range values {
			if len(qValues) == 0 {
				continue
			}

			qVal := qValues[0]

			// backwards compatible behavior for the uuid key
			if qKey == constants.UUIDKey && !(len(qValues) == 1 && len(strings.TrimSpace(qVal)) > 0) {
				uid, err := getSystemUUID(ctx, r)
				if err != nil {
					return fmt.Errorf("error while substituting UUID: %w", err)
				}

				values.Set(constants.UUIDKey, uid)

				continue
			}

			replacer := m.process(qVal)

			if len(replacer.Matches) < 1 {
				continue
			}

			val, err := getFunc(ctx, r)
			if err != nil {
				return genErr(m.Key, err)
			}

			qVal = replacer.ReplaceMatches(val)

			values.Set(qKey, qVal)
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
	return r.TypedSpec().UUID
}

func getAndCheckSerialNumber(r *hardwareResource.SystemInformation) string {
	return r.TypedSpec().SerialNumber
}

func getSystemUUID(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, hardwareResource.NamespaceName, hardwareResource.SystemInformationType, getAndCheckUUID)
}

func getSystemSerialNumber(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, hardwareResource.NamespaceName, hardwareResource.SystemInformationType, getAndCheckSerialNumber)
}

func getAndCheckMACAddr(r *network.LinkStatus) string {
	linkStatus := r.TypedSpec()
	if linkStatus.LinkState {
		return linkStatus.HardwareAddr.String()
	}

	return ""
}

func getMACAddress(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, network.NamespaceName, network.LinkStatusType, getAndCheckMACAddr)
}

func getAndCheckHostname(r *network.HostnameSpec) string {
	return r.TypedSpec().Hostname
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
