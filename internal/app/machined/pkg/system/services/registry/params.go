// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"net/http"
	"path"
	"strings"

	"github.com/distribution/reference"
	"github.com/siderolabs/gen/xerrors"
)

func extractParams(req *http.Request) (params, error) {
	registry := req.URL.Query().Get("ns")

	value := req.PathValue("args")

	parts := strings.Split(path.Clean(value), "/")
	if len(parts) < 3 {
		return params{}, xerrors.NewTaggedf[notFoundTag]("incorrect args value '%s'", value)
	}

	numParts := len(parts)
	isBlob := parts[numParts-2] == "blobs"
	isManifest := parts[numParts-2] == "manifests"

	if !isBlob && !isManifest {
		return params{}, xerrors.NewTaggedf[notFoundTag]("incorrect ref: '%s'", parts[numParts-2])
	}

	name := strings.Join(parts[:numParts-2], "/")
	dig := parts[numParts-1]

	if !reference.NameRegexp.MatchString(name) {
		return params{}, xerrors.NewTaggedf[badRequestTag]("incorrect name: '%s'", name)
	}

	return params{registry: registry, name: name, dig: dig, isBlob: isBlob}, nil
}

type params struct {
	registry string
	name     string
	dig      string
	isBlob   bool
}

func (p params) String() string {
	var result strings.Builder

	if p.registry != "" {
		result.WriteString(p.registry)
		result.WriteByte('/')
	}

	result.WriteString(p.name)

	if strings.HasPrefix(p.dig, "sha256:") {
		result.WriteByte('@')
		result.WriteString(p.dig)
	} else {
		result.WriteByte(':')
		result.WriteString(p.dig)
	}

	return result.String()
}
