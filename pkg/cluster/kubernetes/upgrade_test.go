// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
)

func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple image",
			ref:     "k8s.gcr.io/kube-apiserver:v1.23.0",
			wantErr: false,
		},
		{
			name:    "valid image with digest",
			ref:     "k8s.gcr.io/kube-apiserver@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: false,
		},
		{
			name:    "valid image with port",
			ref:     "localhost:5000/kube-apiserver:latest",
			wantErr: false,
		},
		{
			name:    "invalid image reference",
			ref:     "invalid/image@sha256:invalid",
			wantErr: true,
			errMsg:  "invalid image reference",
		},
		{
			name:    "invalid image reference v2",
			ref:     ":v1.32.1",
			wantErr: true,
			errMsg:  "invalid image reference",
		},
		{
			name:    "empty image reference",
			ref:     "",
			wantErr: true,
			errMsg:  "invalid image reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := kubernetes.ValidateImageReference(tt.ref)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpgradeOptions_validate(t *testing.T) {
	tests := []struct {
		name    string
		options kubernetes.UpgradeOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid images",
			options: kubernetes.UpgradeOptions{
				KubeletImage:           "k8s.gcr.io/kubelet:v1.23.0",
				APIServerImage:         "k8s.gcr.io/kube-apiserver:v1.23.0",
				ControllerManagerImage: "k8s.gcr.io/kube-controller-manager:v1.23.0",
				SchedulerImage:         "k8s.gcr.io/kube-scheduler:v1.23.0",
				ProxyImage:             "k8s.gcr.io/kube-proxy:v1.23.0",
			},
			wantErr: false,
		},
		{
			name: "invalid kubelet image",
			options: kubernetes.UpgradeOptions{
				KubeletImage:           "invalid/image@sha256:invalid",
				APIServerImage:         "k8s.gcr.io/kube-apiserver:v1.23.0",
				ControllerManagerImage: "k8s.gcr.io/kube-controller-manager:v1.23.0",
				SchedulerImage:         "k8s.gcr.io/kube-scheduler:v1.23.0",
				ProxyImage:             "k8s.gcr.io/kube-proxy:v1.23.0",
			},
			wantErr: true,
			errMsg:  "kubelet: invalid image reference",
		},
		{
			name: "invalid apiserver image",
			options: kubernetes.UpgradeOptions{
				KubeletImage:           "k8s.gcr.io/kubelet:v1.23.0",
				APIServerImage:         ":v1.23.0",
				ControllerManagerImage: "k8s.gcr.io/kube-controller-manager:v1.23.0",
				SchedulerImage:         "k8s.gcr.io/kube-scheduler:v1.23.0",
				ProxyImage:             "k8s.gcr.io/kube-proxy:v1.23.0",
			},
			wantErr: true,
			errMsg:  "apiserver: invalid image reference",
		},
		{
			name: "image with digest",
			options: kubernetes.UpgradeOptions{
				KubeletImage:           "k8s.gcr.io/kubelet@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				APIServerImage:         "k8s.gcr.io/kube-apiserver:v1.23.0@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				ControllerManagerImage: "k8s.gcr.io/kube-controller-manager:v1.23.0",
				SchedulerImage:         "k8s.gcr.io/kube-scheduler:v1.23.0",
				ProxyImage:             "k8s.gcr.io/kube-proxy:v1.23.0",
			},
			wantErr: false,
		},
		{
			name: "image with port number",
			options: kubernetes.UpgradeOptions{
				KubeletImage:           "localhost:5000/kubelet:latest",
				APIServerImage:         "k8s.gcr.io/kube-apiserver:v1.23.0",
				ControllerManagerImage: "k8s.gcr.io/kube-controller-manager:v1.23.0",
				SchedulerImage:         "k8s.gcr.io/kube-scheduler:v1.23.0",
				ProxyImage:             "k8s.gcr.io/kube-proxy:v1.23.0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
