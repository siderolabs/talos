// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// K8sUpgradeChecks is a set of checks to run before upgrading k8s components.
type K8sUpgradeChecks struct {
	state             state.State
	controlPlaneNodes []string
	log               func(string, ...interface{})

	upgradePath         string
	upgradeVersionCheck map[string]componentChecks
}

// K8sComponentRemovedItemsError is an error type for removed items.
type K8sComponentRemovedItemsError struct {
	AdmissionFlags []K8sComponentItem
	CLIFlags       []K8sComponentItem
	FeatureGates   []K8sComponentItem
}

// K8sComponentItem represents a component item.
type K8sComponentItem struct {
	Node      string
	Component string
	Value     string
}

type componentChecks struct {
	// feature gates are common to kube-apiserver, kube-controller-manager and kube-scheduler
	removedFeatureGates []string
	// checks specific to kube-apiserver
	kubeAPIServerChecks apiServerCheck
	// checks specific to kube-controller-manager
	kubeControllerManagerChecks k8sComponentCheck
	// checks specific to kube-scheduler
	kubeSchedulerChecks k8sComponentCheck
}

type apiServerCheck struct {
	// removedAPIResources represent the Kuberenetes API resources that are removed in the upgrade version
	removedAPIResources []string
	// removedAdmissionPlugins represent the Kuberenetes Admission Plugins that are removed in the upgrade version
	removedAdmissionPlugins []string
	k8sComponentCheck
}

type k8sComponentCheck struct {
	// removedFlags represent the Kuberenetes API server flags that are removed in the upgrade version
	removedFlags []string
}

// NewK8sUpgradeChecks initializes and returns K8sUpgradeChecks.
func NewK8sUpgradeChecks(state state.State, options UpgradeOptions, controlPlaneNodes []string) (*K8sUpgradeChecks, error) {
	checks := &K8sUpgradeChecks{
		log:               options.Log,
		upgradePath:       options.Path(),
		controlPlaneNodes: controlPlaneNodes,
		upgradeVersionCheck: map[string]componentChecks{
			"1.21->1.22": {
				kubeAPIServerChecks: apiServerCheck{
					removedAPIResources: []string{
						"validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
						"mutatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
						"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
						"apiservices.v1beta1.apiregistration.k8s.io",
						"leases.v1beta1.coordination.k8s.io",
						"ingresses.v1beta1.extensions",
						"ingresses.v1beta1.networking.k8s.io",
					},
				},
			},
			"1.24->1.25": {
				kubeAPIServerChecks: apiServerCheck{
					removedAPIResources: []string{
						"cronjobs.v1beta1.batch",
						"endpointslices.v1beta1.discovery.k8s.io",
						"events.v1beta1.events.k8s.io",
						"horizontalpodautoscalers.v2beta1.autoscaling",
						"poddisruptionbudgets.v1beta1.policy",
						"podsecuritypolicies.v1beta1.policy",
						"runtimeclasses.v1beta1.node.k8s.io",
					},
					k8sComponentCheck: k8sComponentCheck{
						removedFlags: []string{
							"service-account-api-audiences",
						},
					},
					removedAdmissionPlugins: []string{
						"PodSecurityPolicy",
					},
				},
				kubeControllerManagerChecks: k8sComponentCheck{
					removedFlags: []string{
						"deleting-pods-qps",
						"deleting-pods-burst",
						"register-retry-count",
					},
				},
				// https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates-removed/
				removedFeatureGates: []string{
					"CSIVolumeFSGroupPolicy",
					"ConfigurableFSGroupPolicy",
					"PodDisruptionBudget",
					"SelectorIndex",
				},
			},
			"1.25->1.26": {
				kubeAPIServerChecks: apiServerCheck{
					removedAPIResources: []string{
						"flowschemas.v1beta1.flowcontrol.apiserver.k8s.io",
						"prioritylevelconfigurations.v1beta1.flowcontrol.apiserver.k8s.io",
						"horizontalpodautoscalers.v2beta2.autoscaling",
					},
				},
				removedFeatureGates: []string{
					"DynamicKubeletConfig",
				},
			},
		},
	}

	checks.state = state

	return checks, nil
}

// Run executes the checks.
// nolint: gocyclo
func (checks *K8sUpgradeChecks) Run(ctx context.Context) error {
	var k8sComponentCheck K8sComponentRemovedItemsError

	checks.log("Checking for removed Kubernetes component flags")

	if k8sComponentChecks, ok := checks.upgradeVersionCheck[checks.upgradePath]; ok {
		for _, node := range checks.controlPlaneNodes {
			ctx = client.WithNode(ctx, node)

			for _, id := range []string{k8s.APIServerID, k8s.ControllerManagerID, k8s.SchedulerID} {
				staticPod, err := safe.StateGet[*k8s.StaticPod](ctx, checks.state, k8s.NewStaticPod(k8s.NamespaceName, id).Metadata())
				if err != nil {
					return err
				}

				pod, err := staticPodTypedResourceToK8sPodSpec(staticPod)
				if err != nil {
					return err
				}

				switch id {
				case k8s.APIServerID:
					k8sComponentCheck.PopulateRemovedAdmissionPlugins(node, id, pod.Spec.Containers[0].Command, k8sComponentChecks.kubeAPIServerChecks.removedAdmissionPlugins)
					k8sComponentCheck.PopulateRemovedCLIFlags(node, id, pod.Spec.Containers[0].Command, k8sComponentChecks.kubeAPIServerChecks.k8sComponentCheck.removedFlags)
				case k8s.ControllerManagerID:
					k8sComponentCheck.PopulateRemovedCLIFlags(node, id, pod.Spec.Containers[0].Command, k8sComponentChecks.kubeControllerManagerChecks.removedFlags)
				case k8s.SchedulerID:
					k8sComponentCheck.PopulateRemovedCLIFlags(node, id, pod.Spec.Containers[0].Command, k8sComponentChecks.kubeSchedulerChecks.removedFlags)
				}

				k8sComponentCheck.PopulateRemovedFeatureGates(node, id, pod.Spec.Containers[0].Command, k8sComponentChecks.removedFeatureGates)
			}
		}
	}

	return k8sComponentCheck.ErrorOrNil()
}

// PopulateRemovedCLIFlags populates the removed flags.
func (e *K8sComponentRemovedItemsError) PopulateRemovedCLIFlags(node, component string, apiServerCLIFlags []string, removedFlags []string) {
	for _, removedFlag := range removedFlags {
		if slices.Contains(apiServerCLIFlags, func(s string) bool {
			return strings.HasPrefix(s, "--"+removedFlag)
		}) {
			e.CLIFlags = append(e.CLIFlags, K8sComponentItem{
				Node:      node,
				Component: component,
				Value:     removedFlag,
			})
		}
	}
}

// PopulateRemovedFeatureGates populates the removed feature gates.
func (e *K8sComponentRemovedItemsError) PopulateRemovedFeatureGates(node, component string, apiServerCLIFlags []string, removedFeatureGates []string) {
	featureGateFlags := slices.Filter(apiServerCLIFlags, func(s string) bool {
		return strings.HasPrefix(s, "--feature-gates")
	})

	if len(featureGateFlags) > 0 {
		featureGates := strings.Split(strings.TrimPrefix(featureGateFlags[0], "--feature-gates="), ",")

		for _, removedFeatureGate := range removedFeatureGates {
			if slices.Contains(featureGates, func(s string) bool {
				return removedFeatureGate == strings.Split(s, "=")[0]
			}) {
				e.FeatureGates = append(e.FeatureGates, K8sComponentItem{
					Node:      node,
					Component: component,
					Value:     removedFeatureGate,
				})
			}
		}
	}
}

// PopulateRemovedAdmissionPlugins populates the removed admission plugins.
func (e *K8sComponentRemovedItemsError) PopulateRemovedAdmissionPlugins(node, component string, apiServerCLIFlags []string, removedAdmissionPlugins []string) {
	admissionFlags := slices.Filter(apiServerCLIFlags, func(s string) bool {
		return strings.HasPrefix(s, "--enable-admission-plugins")
	})

	if len(admissionFlags) > 0 {
		admissionPlugins := strings.Split(strings.TrimPrefix(admissionFlags[0], "--enable-admission-plugins="), ",")

		for _, removedAdmissionPlugin := range removedAdmissionPlugins {
			if slices.Contains(admissionPlugins, func(s string) bool {
				return removedAdmissionPlugin == s
			}) {
				e.AdmissionFlags = append(e.AdmissionFlags, K8sComponentItem{
					Node:      node,
					Component: component,
					Value:     removedAdmissionPlugin,
				})
			}
		}
	}
}

func staticPodTypedResourceToK8sPodSpec(staticPod *k8s.StaticPod) (*v1.Pod, error) {
	var spec v1.Pod

	jsonSerialized, err := json.Marshal(staticPod.TypedSpec().Pod)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonSerialized, &spec)

	return &spec, err
}

// Error returns the error message.
func (e K8sComponentRemovedItemsError) Error() string {
	var buf bytes.Buffer

	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)

	if len(e.AdmissionFlags) > 0 {
		fmt.Fprintf(w, "\nNODE\tCOMPONENT\tREMOVED ADMISSION PLUGIN\n")

		for _, item := range e.AdmissionFlags {
			fmt.Fprintf(w, "%s\t%s\t%s\n", item.Node, item.Component, item.Value)
		}
	}

	if len(e.FeatureGates) > 0 {
		fmt.Fprintf(w, "\nNODE\tCOMPONENT\tREMOVED FEATURE GATE\n")

		for _, item := range e.FeatureGates {
			fmt.Fprintf(w, "%s\t%s\t%s\n", item.Node, item.Component, item.Value)
		}
	}

	if len(e.CLIFlags) > 0 {
		fmt.Fprintf(w, "\nNODE\tCOMPONENT\tREMOVED FLAG\n")

		for _, item := range e.CLIFlags {
			fmt.Fprintf(w, "%s\t%s\t%s\n", item.Node, item.Component, item.Value)
		}
	}

	// nolint: errcheck
	w.Flush()

	return buf.String()
}

// ErrorOrNil returns the error if it exists.
func (e K8sComponentRemovedItemsError) ErrorOrNil() error {
	if e.Error() != "" {
		return e
	}

	return nil
}
