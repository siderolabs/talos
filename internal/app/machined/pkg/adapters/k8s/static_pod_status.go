// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
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

	a.StaticPodStatus.TypedSpec().PodStatus = map[string]interface{}{}

	return json.Unmarshal(jsonSerialized, &a.StaticPodStatus.TypedSpec().PodStatus)
}
