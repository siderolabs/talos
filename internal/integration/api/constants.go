// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

const (
	// MetalLBChartVersion is the version of the MetalLB Helm chart to use.
	// renovate: datasource=helm versioning=helm depName=metallb registryUrl=https://metallb.github.io/metallb
	MetalLBChartVersion = "0.16.1"
	// BGPBackendImage is the image used for local LoadBalancer service endpoints in BGP tests.
	// renovate: datasource=docker versioning=docker depName=library/nginx
	BGPBackendImage = "docker.io/library/nginx:1.29-alpine"
	// NvidiaGPUOperatorChartVersion is the version of the NVIDA device plugin chart to use
	// renovate: datasource=helm versioning=helm depName=gpu-operator registryUrl=https://helm.ngc.nvidia.com/nvidia
	NvidiaGPUOperatorChartVersion = "v26.7.0"
	// NvidiaCUDATestImageVersion is the version of the NVIDIA CUDA test image to use
	// renovate: datasource=docker versioning=docker depName=nvcr.io/nvidia/k8s/cuda-sample
	NvidiaCUDATestImageVersion = "vectoradd-cuda12.5.0"
)
