// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"strings"

	"github.com/rivo/tview"

	"github.com/siderolabs/talos/internal/pkg/dashboard/data"
)

// KubernetesInfo represents the kubernetes info widget.
type KubernetesInfo struct {
	tview.TextView
}

// NewKubernetesInfo initializes KubernetesInfo.
func NewKubernetesInfo() *KubernetesInfo {
	kubernetes := &KubernetesInfo{
		TextView: *tview.NewTextView(),
	}

	kubernetes.SetDynamicColors(true).
		SetText(noData).
		SetBorderPadding(1, 0, 1, 0)

	return kubernetes
}

type staticPodStatuses struct {
	apiServer         string
	controllerManager string
	scheduler         string
}

// Update implements the NodeDataComponent interface.
// Update implements the DataWidget interface.
func (widget *KubernetesInfo) Update(node string, data *data.Data) {
	nodeData := data.Nodes[node]
	if nodeData == nil {
		widget.SetText(noData)

		return
	}

	podStatuses := widget.staticPodStatuses(nodeData)

	kubernetesVersion := notAvailable
	kubeletStatus := notAvailable

	if nodeData.KubeletSpec != nil {
		imageParts := strings.Split(nodeData.KubeletSpec.TypedSpec().Image, ":")
		if len(imageParts) > 0 {
			kubernetesVersion = imageParts[len(imageParts)-1]
		}
	}

	if nodeData.ServiceList != nil {
		for _, info := range nodeData.ServiceList.GetServices() {
			if info.Id == "kubelet" {
				kubeletStatus = toHealthStatus(info.GetHealth().Healthy)

				break
			}
		}
	}

	fields := fieldGroup{
		fields: []field{
			{
				Name:  "KUBERNETES",
				Value: kubernetesVersion,
			},
			{
				Name:  "KUBELET",
				Value: kubeletStatus,
			},
			{
				Name:  "APISERVER",
				Value: podStatuses.apiServer,
			},
			{
				Name:  "CONTROLLER-MANAGER",
				Value: podStatuses.controllerManager,
			},
			{
				Name:  "SCHEDULER",
				Value: podStatuses.scheduler,
			},
		},
	}

	widget.SetText(fields.String())
}

func (widget *KubernetesInfo) staticPodStatuses(nodeData *data.Node) staticPodStatuses {
	statuses := staticPodStatuses{
		apiServer:         notAvailable,
		controllerManager: notAvailable,
		scheduler:         notAvailable,
	}

	isReady := func(podStatus map[string]any) string {
		conditions, conditionsOk := podStatus["conditions"]
		if !conditionsOk {
			return notAvailable
		}

		conditionsSlc, conditionsSlcOk := conditions.([]any)
		if !conditionsSlcOk {
			return notAvailable
		}

		for _, condition := range conditionsSlc {
			conditionObj, conditionObjOk := condition.(map[string]any)
			if !conditionObjOk {
				return notAvailable
			}

			if conditionObj["type"] == "Ready" {
				return toHealthStatus(conditionObj["status"] == "True")
			}
		}

		return notAvailable
	}

	for _, status := range nodeData.StaticPodStatuses {
		podStatus := status.TypedSpec().PodStatus

		switch {
		case strings.Contains(status.Metadata().ID(), "kube-apiserver"):
			statuses.apiServer = isReady(podStatus)
		case strings.Contains(status.Metadata().ID(), "kube-controller-manager"):
			statuses.controllerManager = isReady(podStatus)
		case strings.Contains(status.Metadata().ID(), "kube-scheduler"):
			statuses.scheduler = isReady(podStatus)
		}
	}

	return statuses
}
