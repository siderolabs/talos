// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

const (
	// NvidiaGPUOperatorChartVersion is the version of the NVIDA device plugin chart to use
	// renovate: datasource=helm versioning=helm depName=gpu-operator registryUrl=https://helm.ngc.nvidia.com/nvidia
	NvidiaGPUOperatorChartVersion = "v26.3.0"
	// NvidiaCUDATestImageVersion is the version of the NVIDIA CUDA test image to use
	// renovate: datasource=docker versioning=docker depName=nvcr.io/nvidia/k8s/cuda-sample
	NvidiaCUDATestImageVersion = "vectoradd-cuda12.5.0"
)
