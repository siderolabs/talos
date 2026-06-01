// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package components

import (
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/rivo/tview"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/internal/pkg/dashboard/apidata"
	"github.com/siderolabs/talos/internal/pkg/dashboard/resourcedata"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type staticPodStatuses struct {
	apiServer         string
	controllerManager string
	scheduler         string
}

type kubernetesInfoData struct {
	isControlPlane    bool
	typ               string
	kubernetesVersion string
	kubeletStatus     string

	podStatuses        staticPodStatuses
	staticPodStatusMap map[resource.ID]*k8s.StaticPodStatus
}

// KubernetesInfo represents the kubernetes info widget.
type KubernetesInfo struct {
	tview.TextView

	selectedNode string
	nodeMap      map[string]*kubernetesInfoData
}

// NewKubernetesInfo initializes KubernetesInfo.
func NewKubernetesInfo() *KubernetesInfo {
	kubernetes := &KubernetesInfo{
		TextView: *tview.NewTextView(),
		nodeMap:  make(map[string]*kubernetesInfoData),
	}

	kubernetes.SetDynamicColors(true).
		SetText(noData).
		SetBorderPadding(1, 0, 1, 0)

	return kubernetes
}

// OnNodeSelect implements the NodeSelectListener interface.
func (widget *KubernetesInfo) OnNodeSelect(node string) {
	if node != widget.selectedNode {
		widget.selectedNode = node

		widget.redraw()
	}
}

// OnResourceDataChange implements the ResourceDataListener interface.
func (widget *KubernetesInfo) OnResourceDataChange(data resourcedata.Data) {
	widget.updateNodeData(data)

	if data.Node == widget.selectedNode {
		widget.redraw()
	}
}

// OnAPIDataChange implements the APIDataListener interface.
func (widget *KubernetesInfo) OnAPIDataChange(node string, data *apidata.Data) {
	nodeAPIData := data.Nodes[node]

	widget.updateNodeAPIData(node, nodeAPIData)

	if node == widget.selectedNode {
		widget.redraw()
	}
}

func (widget *KubernetesInfo) updateNodeData(data resourcedata.Data) {
	nodeData := widget.getOrCreateNodeData(data.Node)

	switch res := data.Resource.(type) {
	case *k8s.KubeletSpec:
		if data.Deleted {
			nodeData.kubernetesVersion = notAvailable
		} else {
			imageParts := strings.Split(res.TypedSpec().Image, ":")
			if len(imageParts) > 0 {
				nodeData.kubernetesVersion = imageParts[len(imageParts)-1]
			}
		}
	case *k8s.StaticPodStatus:
		if data.Deleted {
			delete(nodeData.staticPodStatusMap, res.Metadata().ID())
		} else {
			nodeData.staticPodStatusMap[res.Metadata().ID()] = res
		}

		nodeData.podStatuses = widget.staticPodStatuses(maps.Values(nodeData.staticPodStatusMap))
	case *config.MachineType:
		if data.Deleted {
			nodeData.isControlPlane = false
			nodeData.typ = notAvailable
		} else {
			nodeData.isControlPlane = res.MachineType() == machine.TypeControlPlane
			nodeData.typ = res.MachineType().String()
		}
	}
}

func (widget *KubernetesInfo) updateNodeAPIData(node string, data *apidata.Node) {
	nodeData := widget.getOrCreateNodeData(node)

	if data != nil && data.ServiceList != nil {
		for _, info := range data.ServiceList.GetServices() {
			if info.Id == "kubelet" {
				nodeData.kubeletStatus = toHealthStatus(info.GetHealth().Healthy)

				break
			}
		}
	}
}

func (widget *KubernetesInfo) getOrCreateNodeData(node string) *kubernetesInfoData {
	nodeData, ok := widget.nodeMap[node]
	if !ok {
		nodeData = &kubernetesInfoData{
			typ:               notAvailable,
			kubernetesVersion: notAvailable,
			kubeletStatus:     notAvailable,
			podStatuses: staticPodStatuses{
				apiServer:         notAvailable,
				controllerManager: notAvailable,
				scheduler:         notAvailable,
			},
			staticPodStatusMap: make(map[resource.ID]*k8s.StaticPodStatus),
		}

		widget.nodeMap[node] = nodeData
	}

	return nodeData
}

func (widget *KubernetesInfo) redraw() {
	data := widget.getOrCreateNodeData(widget.selectedNode)

	fieldList := make([]field, 0, 5)

	fieldList = append(fieldList,
		field{
			Name:  "TYPE",
			Value: data.typ,
		},
		field{
			Name:  "KUBERNETES",
			Value: data.kubernetesVersion,
		},
		field{
			Name:  "KUBELET",
			Value: data.kubeletStatus,
		})

	if data.isControlPlane {
		fieldList = append(fieldList,
			field{
				Name:  "APISERVER",
				Value: data.podStatuses.apiServer,
			},
			field{
				Name:  "CONTROLLER-MANAGER",
				Value: data.podStatuses.controllerManager,
			},
			field{
				Name:  "SCHEDULER",
				Value: data.podStatuses.scheduler,
			})
	}

	fields := fieldGroup{
		fields: fieldList,
	}

	widget.SetText(fields.String())
}

func (widget *KubernetesInfo) staticPodStatuses(statuses []*k8s.StaticPodStatus) staticPodStatuses {
	result := staticPodStatuses{
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

	for _, status := range statuses {
		podStatus := status.TypedSpec().PodStatus

		switch {
		case strings.Contains(status.Metadata().ID(), "kube-apiserver"):
			result.apiServer = isReady(podStatus)
		case strings.Contains(status.Metadata().ID(), "kube-controller-manager"):
			result.controllerManager = isReady(podStatus)
		case strings.Contains(status.Metadata().ID(), "kube-scheduler"):
			result.scheduler = isReady(podStatus)
		}
	}

	return result
}
