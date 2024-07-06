// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// StaticPodStatus adapter provides conversion from *v1.PodStatus.
//
//nolint:revive,golint
func StaticPodStatus(r *k8s.StaticPodStatus) staticPodStatus {
	return staticPodStatus{
		StaticPodStatus: r,
	}
}

type staticPodStatus struct {
	*k8s.StaticPodStatus
}

// SetStatus sets status from native Kubernetes resource.
func (a staticPodStatus) SetStatus(status *v1.PodStatus) error {
	jsonSerialized, err := json.Marshal(status)
	if err != nil {
		return err
	}

	a.StaticPodStatus.TypedSpec().PodStatus = map[string]any{}

	return json.Unmarshal(jsonSerialized, &a.StaticPodStatus.TypedSpec().PodStatus)
}

// Status gets status from native Kubernetes resource.
func (a staticPodStatus) Status() (*v1.PodStatus, error) {
	var spec v1.PodStatus

	jsonSerialized, err := json.Marshal(a.StaticPodStatus.TypedSpec().PodStatus)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonSerialized, &spec)

	return &spec, err
}
