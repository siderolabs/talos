// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// Resources returns Kubernetes resource requirements based on the provided configuration and defaults.
func Resources(resourcesConfig k8s.Resources, defaultCPU, defaultMemory string) (corev1.ResourceRequirements, error) {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    apiresource.MustParse(defaultCPU),
			corev1.ResourceMemory: apiresource.MustParse(defaultMemory),
		},
		Limits: corev1.ResourceList{},
	}

	if cpu := resourcesConfig.Requests[string(corev1.ResourceCPU)]; cpu != "" {
		parsedCPU, err := apiresource.ParseQuantity(cpu)
		if err != nil {
			return corev1.ResourceRequirements{}, fmt.Errorf("error parsing CPU request: %w", err)
		}

		resources.Requests[corev1.ResourceCPU] = parsedCPU
	}

	if memory := resourcesConfig.Requests[string(corev1.ResourceMemory)]; memory != "" {
		parsedMemory, err := apiresource.ParseQuantity(memory)
		if err != nil {
			return corev1.ResourceRequirements{}, fmt.Errorf("error parsing memory request: %w", err)
		}

		resources.Requests[corev1.ResourceMemory] = parsedMemory
	}

	if cpu := resourcesConfig.Limits[string(corev1.ResourceCPU)]; cpu != "" {
		parsedCPU, err := apiresource.ParseQuantity(cpu)
		if err != nil {
			return corev1.ResourceRequirements{}, fmt.Errorf("error parsing CPU limit: %w", err)
		}

		resources.Limits[corev1.ResourceCPU] = parsedCPU
	}

	if memory := resourcesConfig.Limits[string(corev1.ResourceMemory)]; memory != "" {
		parsedMemory, err := apiresource.ParseQuantity(memory)
		if err != nil {
			return corev1.ResourceRequirements{}, fmt.Errorf("error parsing memory limit: %w", err)
		}

		resources.Limits[corev1.ResourceMemory] = parsedMemory
	}

	return resources, nil
}

// GoGCMemLimitPercentage sets the percentage of memorylimit to use for the golang garbage collection target limit.
const GoGCMemLimitPercentage = 95

// GoGCEnvFromResources returns an environment variable for Go's garbage collection target limit
// based on the provided resource requirements.
func GoGCEnvFromResources(resources corev1.ResourceRequirements) (envVar corev1.EnvVar) {
	memoryLimit := resources.Limits[corev1.ResourceMemory]
	if memoryLimit.Value() > 0 {
		gcMemLimit := memoryLimit.Value() * GoGCMemLimitPercentage / 100
		envVar = corev1.EnvVar{
			Name:  "GOMEMLIMIT",
			Value: strconv.FormatInt(gcMemLimit, 10),
		}
	}

	return envVar
}
