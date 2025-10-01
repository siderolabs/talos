// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configdiff

import (
	"bytes"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"

	configcore "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/documentid"
)

type unstructured map[string]any

// Patch will return a slice of Patches capable of converting the original config to the modified config when applied in order.
func Patch(original, modified configcore.Provider) ([]configpatcher.Patch, error) {
	patches := make([]configpatcher.Patch, 0, 2)

	firstPass, err := patch(original, modified, true)
	if err != nil {
		return nil, err
	}

	if firstPass != nil {
		firstPatch, ok := (*firstPass).(configpatcher.Patch)
		if !ok {
			return nil, fmt.Errorf("expected Patch, got %T", *firstPass)
		}

		patches = append(patches, firstPatch)
	}

	secondPass, err := patch(original, modified, false)
	if err != nil {
		return nil, err
	}

	if secondPass != nil {
		secondPatch, ok := (*secondPass).(configpatcher.Patch)
		if !ok {
			return nil, fmt.Errorf("expected Patch, got %T", *secondPass)
		}

		patches = append(patches, secondPatch)
	}

	return patches, nil
}

// nolint: gocyclo
func patch(original, modified configcore.Provider, firstPass bool) (*configpatcher.StrategicMergePatch, error) {
	originalIDToDoc := documentsToMap(original.Documents())
	modifiedIDToDoc := documentsToMap(modified.Documents())

	var removed, added, common []documentid.DocumentID // nolint:prealloc

	for id := range originalIDToDoc {
		if _, ok := modifiedIDToDoc[id]; !ok {
			removed = append(removed, id)

			continue
		}

		common = append(common, id)
	}

	for id := range modifiedIDToDoc {
		if _, ok := originalIDToDoc[id]; !ok {
			added = append(added, id)
		}
	}

	unstructuredPatches := make([]unstructured, 0, len(removed)+len(added)+len(common))

	if firstPass {
		for _, removedID := range removed {
			meta := removedID.Meta()
			meta["$patch"] = "delete"

			unstructuredPatches = append(unstructuredPatches, meta)
		}

		for _, addedID := range added {
			addedDoc := modifiedIDToDoc[addedID]

			addedUnstructured, err := documentToUnstructured(addedDoc)
			if err != nil {
				return nil, err
			}

			unstructuredPatches = append(unstructuredPatches, addedUnstructured)
		}
	}

	for _, commonID := range common {
		originalDoc := originalIDToDoc[commonID]
		modifiedDoc := modifiedIDToDoc[commonID]

		originalUnstructured, err := documentToUnstructured(originalDoc)
		if err != nil {
			return nil, err
		}

		modifiedUnstructured, err := documentToUnstructured(modifiedDoc)
		if err != nil {
			return nil, err
		}

		mergePatch, err := createMergePatch(originalUnstructured, modifiedUnstructured, &commonID, firstPass)
		if err != nil {
			return nil, err
		}

		if len(mergePatch) == 0 {
			continue
		}

		unstructuredPatches = append(unstructuredPatches, mergePatch)
	}

	if len(unstructuredPatches) == 0 {
		return nil, nil
	}

	patchYAML, err := encodeToYAML(unstructuredPatches)
	if err != nil {
		return nil, err
	}

	cfg, err := configloader.NewFromBytes(patchYAML, configloader.WithAllowPatchDelete())
	if err != nil {
		return nil, err
	}

	mergePatch := configpatcher.NewStrategicMergePatch(cfg)

	return &mergePatch, nil
}

func encodeToYAML(docs []unstructured) ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return nil, err
		}
	}

	if err := enc.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func documentsToMap(docs []config.Document) map[documentid.DocumentID]config.Document {
	out := make(map[documentid.DocumentID]config.Document, len(docs))

	for _, doc := range docs {
		out[documentid.Extract(doc)] = doc
	}

	return out
}

func documentToUnstructured(doc config.Document) (unstructured, error) {
	c, err := container.New(doc)
	if err != nil {
		return nil, err
	}

	unstructuredBytes, err := c.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return nil, err
	}

	var out unstructured
	if err = yaml.Unmarshal(unstructuredBytes, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// nolint: gocyclo,cyclop
func createMergePatch(original, modified unstructured, documentID *documentid.DocumentID, firstPass bool) (unstructured, error) {
	meta := unstructured{}
	mergePatch := unstructured{}

	// First, handle all modified and added values
	for key, modV := range modified {
		// Discover and keep meta fields obtained from documentId
		if documentID != nil {
			if metaV, ok := documentID.Meta()[key]; ok && metaV != "" {
				meta[key] = metaV

				continue
			}
		}

		origV, ok := original[key]
		if !ok {
			setValue(&mergePatch, key, modV, firstPass)

			continue
		}

		if reflect.TypeOf(origV) != reflect.TypeOf(modV) {
			setValue(&mergePatch, key, modV, firstPass)

			continue
		}

		switch origT := origV.(type) {
		case unstructured:
			modT := modV.(unstructured)

			patchV, err := createMergePatch(origT, modT, nil, firstPass)
			if err != nil {
				return nil, err
			}

			if len(patchV) > 0 {
				mergePatch[key] = patchV
			}
		case []any:
			modT := modV.([]any)
			if !reflect.DeepEqual(origT, modT) {
				if firstPass {
					mergePatch[key] = map[string]string{"$patch": "delete"}
				} else {
					setValue(&mergePatch, key, modV, true)
				}
			}
		case string, int, uint, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float32, float64, bool:
			if !reflect.DeepEqual(origV, modV) {
				setValue(&mergePatch, key, modV, firstPass)
			}
		case nil:
			switch modV.(type) {
			case nil:
				// Both nil, fine.
			default:
				setValue(&mergePatch, key, modV, firstPass)
			}
		default:
			return nil, fmt.Errorf("unknown type:%T in key %s", origV, key)
		}
	}

	// Now handle all deleted keys
	for key := range original {
		if _, ok := modified[key]; !ok {
			setValue(&mergePatch, key, map[string]string{"$patch": "delete"}, firstPass)

			continue
		}
	}

	// Finally, merge meta into mergePatch
	if len(mergePatch) > 0 && len(meta) > 0 {
		for key, metaV := range meta {
			mergePatch[key] = metaV
		}
	}

	return mergePatch, nil
}

func setValue(mergePatch *unstructured, key string, value any, shouldSet bool) {
	if shouldSet {
		(*mergePatch)[key] = value
	}
}
