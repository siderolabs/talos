/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
)

// Upgrade performs an upgrade of the userdata.
func (data *UserData) Upgrade() (ud *UserData, err error) {
	initConfiguration, ok := data.Services.Kubeadm.InitConfiguration.(*kubeadmapi.InitConfiguration)
	if !ok {
		return data, nil
	}

	log.Println("converting existing InitConfiguration to JoinConfiguration")
	join := &kubeadmapi.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		ControlPlane: &kubeadmapi.JoinControlPlane{},
		Discovery: kubeadmapi.Discovery{
			BootstrapToken: &kubeadmapi.BootstrapTokenDiscovery{
				Token:                    initConfiguration.BootstrapTokens[0].Token.String(),
				APIServerEndpoint:        data.Services.Trustd.Endpoints[1] + ":6443",
				UnsafeSkipCAVerification: true,
			},
			TLSBootstrapToken: initConfiguration.BootstrapTokens[0].Token.String(),
			Timeout: &metav1.Duration{
				Duration: 5 * time.Minute,
			},
		},
		NodeRegistration: initConfiguration.NodeRegistration,
		CACertPath:       "/etc/kubernetes/pki/ca.crt",
	}

	data.Services.Kubeadm.InitConfiguration = nil
	data.Services.Kubeadm.Token = nil
	data.Services.Kubeadm.JoinConfiguration = join

	_, err = data.Services.Kubeadm.MarshalYAML()
	if err != nil {
		return nil, err
	}

	return data, nil
}
