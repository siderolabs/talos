// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"github.com/siderolabs/gen/xslices"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

// StrategicMergePatch is a strategic merge config patch.
type StrategicMergePatch struct {
	coreconfig.Provider
}

// StrategicMerge performs strategic merge config patching.
//
// Strategic merge on two sets of documents - on the left hand side and on the right hand side.
// Documents with matching tuples (apiVersion, kind, name) are merged together.
// If the document on the right doesn't exist on the left, it is appended.
func StrategicMerge(cfg coreconfig.Provider, patch StrategicMergePatch) (coreconfig.Provider, error) {
	left := cfg.Clone().Documents()
	right := patch.Documents()

	documentID := func(doc config.Document) string {
		id := doc.APIVersion() + "/" + doc.Kind()

		if named, ok := doc.(config.NamedDocument); ok {
			id += "/" + named.Name()
		}

		return id
	}

	leftIndex := xslices.ToMap(left, func(d config.Document) (string, config.Document) {
		return documentID(d), d
	})

	for _, rightDoc := range right {
		id := documentID(rightDoc)

		if leftDoc, ok := leftIndex[id]; ok {
			if err := merge.Merge(leftDoc, rightDoc); err != nil {
				return nil, err
			}
		} else {
			left = append(left, rightDoc)
		}
	}

	return container.New(left...)
}
