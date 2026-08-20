// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
)

const (
	// remotePatchTimeout bounds a single fetch of a patch over http/https.
	remotePatchTimeout = 30 * time.Second
	// remotePatchSizeLimit bounds how much of a remote response is read; a config patch
	// that does not fit is a mistake or a hostile response, not a patch.
	remotePatchSizeLimit = 1 << 20 // 1 MiB
)

type patch []map[string]any

// LoadPatch loads the strategic merge patch or JSON patch (JSON/YAML for JSON patch).
func LoadPatch(in []byte) (Patch, error) {
	// Try configloader first, as it is more strict about the config format
	cfg, strategicErr := configloader.NewFromBytes(in, configloader.WithAllowPatchDelete(), configloader.WithNoValidation())
	if strategicErr == nil {
		return NewStrategicMergePatch(cfg), nil
	}

	var (
		jsonErr error
		p       jsonpatch.Patch
	)

	// try JSON first
	if p, jsonErr = jsonpatch.DecodePatch(in); jsonErr == nil {
		return p, nil
	}

	// try YAML
	var yamlPatch patch

	if err := yaml.Unmarshal(in, &yamlPatch); err != nil {
		// not YAML either, return previous error
		// see if input looks like JSON Patch as JSON
		if bytes.HasPrefix(bytes.TrimSpace(in), []byte("[")) {
			return nil, jsonErr
		}

		// nope, return config loading error (assume it was strategic merge patch)
		return nil, strategicErr
	}

	p = make(jsonpatch.Patch, 0, len(yamlPatch))

	for _, yp := range yamlPatch {
		op := make(jsonpatch.Operation, len(yp))

		for key, value := range yp {
			m, err := json.Marshal(value)
			if err != nil {
				return p, err
			}

			op[key] = (*json.RawMessage)(&m)
		}

		p = append(p, op)
	}

	return p, nil
}

// isRemotePatch reports whether the patch source is an absolute http/https URL.
//
// Only those two schemes are recognized: anything else keeps the pre-existing meaning of the
// argument (a filename, or an inline patch).
func isRemotePatch(source string) bool {
	u, err := url.Parse(source)
	if err != nil {
		return false
	}

	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// loadRemotePatch fetches a patch over http/https.
func loadRemotePatch(ctx context.Context, source string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, remotePatchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("error building request for patch %q: %w", source, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching patch %q: %w", source, err)
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching patch %q: unexpected status %s", source, resp.Status)
	}

	// read one byte past the limit, so that a response which is exactly at the limit is
	// accepted and a longer one is reported instead of being silently truncated
	contents, err := io.ReadAll(io.LimitReader(resp.Body, remotePatchSizeLimit+1))
	if err != nil {
		return nil, fmt.Errorf("error reading patch %q: %w", source, err)
	}

	if len(contents) > remotePatchSizeLimit {
		return nil, fmt.Errorf("error reading patch %q: response exceeds %d bytes", source, remotePatchSizeLimit)
	}

	return contents, nil
}

// LoadPatches loads the JSON patch either from value literal, from a file if the patch starts
// with '@', or over http/https if the patch is a URL.
//
// It also tries to guess if the filename was given without '@' prefix.
//
// It is LoadPatchesWithContext with a background context; a remote patch is still bounded by
// its own timeout, but cannot be cancelled by the caller.
func LoadPatches(in []string) ([]Patch, error) {
	return LoadPatchesWithContext(context.Background(), in)
}

// LoadPatchesWithContext is LoadPatches with a context governing any http/https fetch it does.
//
// The context is not used for patches read from the local filesystem or supplied inline.
//
//nolint:gocyclo
func LoadPatchesWithContext(ctx context.Context, in []string) ([]Patch, error) {
	var result []Patch

	for _, patchString := range in {
		var (
			p        Patch
			contents []byte
			err      error
		)

		// '@' means "not an inline patch"; what follows is a filename or a URL
		source, explicitRef := strings.CutPrefix(patchString, "@")

		switch {
		case isRemotePatch(source):
			contents, err = loadRemotePatch(ctx, source)
			if err != nil {
				return result, err
			}
		case explicitRef:
			contents, err = os.ReadFile(source)
			if err != nil {
				return result, err
			}
		case !strings.ContainsAny(patchString, "\n ") &&
			!strings.HasPrefix(patchString, "[") &&
			!strings.HasPrefix(patchString, "{"):
			// any valid patch supplied inline should contain either '\n' or space, or start with '[' or '{'
			// so if none of this is true, assume it's a filename, but without '@' prefix
			contents, err = os.ReadFile(patchString)
			if err != nil {
				return result, err
			}
		default:
			contents = []byte(patchString)
		}

		p, err = LoadPatch(contents)
		if err != nil {
			return result, err
		}

		// merge JSON patches if they come one after another
		_, isJSONPatch := p.(jsonpatch.Patch)
		lastJSONPatch := false

		if len(result) > 0 {
			if _, ok := result[len(result)-1].(jsonpatch.Patch); ok {
				lastJSONPatch = true
			}
		}

		if isJSONPatch && lastJSONPatch {
			result[len(result)-1] = append(result[len(result)-1].(jsonpatch.Patch), p.(jsonpatch.Patch)...)
		} else {
			result = append(result, p)
		}
	}

	return result, nil
}
