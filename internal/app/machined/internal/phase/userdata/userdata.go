/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
)

// UserData represents the UserData task.
type UserData struct{}

// NewUserDataTask initializes and returns an UserData task.
func NewUserDataTask() phase.Task {
	return &UserData{}
}

// RuntimeFunc returns the runtime function.
func (task *UserData) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return task.container
	default:
		return task.standard
	}
}

func (task *UserData) standard(platform platform.Platform, data *userdata.UserData) (err error) {
	var d *userdata.UserData
	d, err = platform.UserData()
	if err != nil {
		return err
	}
	*data = *d

	return nil
}

func (task *UserData) container(platform platform.Platform, data *userdata.UserData) (err error) {
	var d *userdata.UserData
	d, err = platform.UserData()
	if err != nil {
		return err
	}
	*data = *d

	data.Services.Kubeadm.IgnorePreflightErrors = []string{"FileContent--proc-sys-net-bridge-bridge-nf-call-iptables", "Swap", "SystemVerification"}
	if data.Services.Kubeadm.KubeletConfiguration != nil {
		kubeletConfig, ok := data.Services.Kubeadm.KubeletConfiguration.(*kubeletconfig.KubeletConfiguration)
		if !ok {
			return errors.New("unable to assert kubelet config")
		}
		f := false
		kubeletConfig.FailSwapOn = &f
	}
	if data.Services.Kubeadm.KubeProxyConfiguration != nil {
		kubeproxyConfig, ok := data.Services.Kubeadm.KubeProxyConfiguration.(*kubeproxyconfig.KubeProxyConfiguration)
		if !ok {
			return errors.New("unable to assert kubeproxy config")
		}
		// See https://github.com/kubernetes/kubernetes/issues/58610#issuecomment-359552443
		maxPerCore := int32(0)
		kubeproxyConfig.Conntrack.MaxPerCore = &maxPerCore
	}

	return nil
}
