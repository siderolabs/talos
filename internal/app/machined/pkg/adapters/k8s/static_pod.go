// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// StaticPod adapter provides conversion from *v1.Pod.
//
//nolint:revive,golint
func StaticPod(r *k8s.StaticPod) staticPod {
	return staticPod{
		StaticPod: r,
	}
}

type staticPod struct {
	*k8s.StaticPod
}

// Pod returns native Kubernetes resource.
func (a staticPod) Pod() (*v1.Pod, error) {
	var spec v1.Pod

	jsonSerialized, err := json.Marshal(a.StaticPod.TypedSpec().Pod)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonSerialized, &spec)

	return &spec, err
}

// SetPod sets spec from native Kubernetes resource.
func (a staticPod) SetPod(podSpec *v1.Pod) error {
	jsonSerialized, err := json.Marshal(podSpec)
	if err != nil {
		return err
	}

	a.StaticPod.TypedSpec().Pod = map[string]any{}

	return json.Unmarshal(jsonSerialized, &a.StaticPod.TypedSpec().Pod)
}
