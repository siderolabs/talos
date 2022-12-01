// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	hardwareResource "github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// PopulateURLParameters fills in empty parameters in the download URL.
//
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

//nolint:gocyclo
func getResource[T resource.Resource](ctx context.Context, r state.State, namespace, typ, valName string, isReadyFunc func(T) bool, checkAndGetFunc func(T) string) (string, error) {
	metadata := resource.NewMetadata(namespace, typ, "", resource.VersionUndefined)

	watchCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	events := make(chan safe.WrappedStateEvent[T])

	err := safe.StateWatchKind(watchCtx, r, metadata, events, state.WithBootstrapContents(true))
	if err != nil {
		return "", fmt.Errorf("failed to watch %s resources: %w", typ, err)
	}

	var watchErr error

	for {
		select {
		case <-watchCtx.Done():
			err := fmt.Errorf("failed to determine %s of %s: %w", valName, typ, watchCtx.Err())
			err = fmt.Errorf("%s; %w", err.Error(), watchErr)

			return "", err
		case event := <-events:
			switch event.Type() {
			case state.Created, state.Updated:
				// ok, proceed
			case state.Destroyed, state.Bootstrapped:
				// ignore
			case state.Errored:
				return "", event.Error()
			}

			eventResource, err := event.Resource()
			if err != nil {
				watchErr = fmt.Errorf("%s; invalid resource in wrapped event: %w", watchErr.Error(), err)
			}

			if !isReadyFunc(eventResource) {
				continue
			}

			val := checkAndGetFunc(eventResource)
			if val == "" {
				return "", fmt.Errorf("%s property of resource %s is empty", valName, typ)
			}

			return val, nil
		}
	}
}

func getUUIDProperty(r *hardwareResource.SystemInformation) string {
	return r.TypedSpec().UUID
}

func getSerialNumberProperty(r *hardwareResource.SystemInformation) string {
	return r.TypedSpec().SerialNumber
}

func getSystemUUID(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, hardwareResource.NamespaceName, hardwareResource.SystemInformationType, "UUID", func(*hardwareResource.SystemInformation) bool { return true }, getUUIDProperty)
}

func getSystemSerialNumber(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx,
		r,
		hardwareResource.NamespaceName,
		hardwareResource.SystemInformationType,
		"Serial Number",
		func(*hardwareResource.SystemInformation) bool { return true },
		getSerialNumberProperty)
}

func getMACAddressProperty(r *network.LinkStatus) string {
	return r.TypedSpec().HardwareAddr.String()
}

func checkLinkUp(r *network.LinkStatus) bool {
	return r.TypedSpec().LinkState
}

func getMACAddress(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, network.NamespaceName, network.LinkStatusType, "MAC Address", checkLinkUp, getMACAddressProperty)
}

func getHostnameProperty(r *network.HostnameSpec) string {
	return r.TypedSpec().Hostname
}

func getHostname(ctx context.Context, r state.State) (string, error) {
	return getResource(ctx, r, network.NamespaceName, network.HostnameSpecType, "Hostname", func(*network.HostnameSpec) bool { return true }, getHostnameProperty)
}
