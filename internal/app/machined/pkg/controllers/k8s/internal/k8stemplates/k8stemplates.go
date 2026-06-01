// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8stemplates contains templates for Kubernetes resources.
package k8stemplates

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

var serializer = sync.OnceValue(func() *k8sjson.Serializer {
	return k8sjson.NewSerializerWithOptions(
		k8sjson.DefaultMetaFactory, nil, nil,
		k8sjson.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)
})

// Marshal serializes the given object into YAML format.
func Marshal(obj runtime.Object) ([]byte, error) {
	var buf bytes.Buffer

	if err := MarshalTo(obj, &buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// MarshalTo serializes the given object into YAML format and writes it to the provided buffer.
func MarshalTo(obj runtime.Object, w io.Writer) error {
	if err := serializer().Encode(obj, w); err != nil {
		return fmt.Errorf("error marshaling object %s: %w", obj.GetObjectKind().GroupVersionKind().String(), err)
	}

	return nil
}
