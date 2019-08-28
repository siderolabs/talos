/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"bytes"
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmv1beta2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeletconfigv1beta1scheme "k8s.io/kubernetes/pkg/kubelet/apis/config/v1beta1"
	kubeproxyconfigv1alpha1scheme "k8s.io/kubernetes/pkg/proxy/apis/config/v1alpha1"
)

// Kubeadm describes the set of configuration options available for kubeadm.
type Kubeadm struct {
	CommonServiceOptions `yaml:",inline"`

	// ConfigurationStr is converted to Configuration and back in Marshal/UnmarshalYAML
	InitConfiguration      runtime.Object `yaml:"-"`
	ClusterConfiguration   runtime.Object `yaml:"-"`
	JoinConfiguration      runtime.Object `yaml:"-"`
	KubeletConfiguration   runtime.Object `yaml:"-"`
	KubeProxyConfiguration runtime.Object `yaml:"-"`

	ConfigurationStr string `yaml:"configuration"`

	ExtraArgs             []string `yaml:"extraArgs,omitempty"`
	CertificateKey        string   `yaml:"certificateKey,omitempty"`
	IgnorePreflightErrors []string `yaml:"ignorePreflightErrors,omitempty"`
	ControlPlane          bool
}

// MarshalYAML implements the yaml.Marshaler interface.
func (kdm *Kubeadm) MarshalYAML() (interface{}, error) {

	// Encode init and cluster configs
	encodedObjs := [][]byte{}
	for _, obj := range []runtime.Object{kdm.InitConfiguration, kdm.ClusterConfiguration, kdm.JoinConfiguration} {
		if obj == nil {
			continue
		}
		encoded, err := kubeadmutil.MarshalToYamlForCodecs(obj, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
		if err != nil {
			return nil, err
		}
		encodedObjs = append(encodedObjs, encoded)
	}

	// Encode proxy config
	if kdm.KubeProxyConfiguration != nil {
		if err := kubeproxyconfigv1alpha1scheme.AddToScheme(kubeadmscheme.Scheme); err != nil {
			return nil, err
		}
		encoded, err := kubeadmutil.MarshalToYamlForCodecs(kdm.KubeProxyConfiguration, kubeproxyconfigv1alpha1scheme.SchemeGroupVersion, kubeadmscheme.Codecs)
		if err != nil {
			return nil, err
		}
		encodedObjs = append(encodedObjs, encoded)
	}

	// Encode kubelet config
	if kdm.KubeletConfiguration != nil {
		if err := kubeletconfigv1beta1scheme.AddToScheme(kubeadmscheme.Scheme); err != nil {
			return nil, err
		}
		encoded, err := kubeadmutil.MarshalToYamlForCodecs(kdm.KubeletConfiguration, kubeletconfigv1beta1scheme.SchemeGroupVersion, kubeadmscheme.Codecs)
		if err != nil {
			return nil, err
		}
		encodedObjs = append(encodedObjs, encoded)
	}

	kubeadmConfig := bytes.Join(encodedObjs, []byte("---\n"))
	kdm.ConfigurationStr = string(kubeadmConfig)

	type KubeadmAlias Kubeadm

	return (*KubeadmAlias)(kdm), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
// nolint: gocyclo
func (kdm *Kubeadm) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type KubeadmAlias Kubeadm

	if err := unmarshal((*KubeadmAlias)(kdm)); err != nil {
		return err
	}

	b := []byte(kdm.ConfigurationStr)

	splitConfig := bytes.Split(b, []byte("---\n"))

	// Range through each config doc, determine kind, set kubeadm struct val
	for _, config := range splitConfig {
		gvks, err := kubeadmutil.GroupVersionKindsFromBytes(config)
		if err != nil {
			return err
		}

		switch {
		case kubeadmutil.GroupVersionKindsHasKind(gvks, "InitConfiguration"):
			cfg, err := kubeadmutil.UnmarshalFromYamlForCodecs(config, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return err
			}
			kdm.InitConfiguration = cfg
			kdm.ControlPlane = true
		case kubeadmutil.GroupVersionKindsHasKind(gvks, "JoinConfiguration"):
			cfg, err := kubeadmutil.UnmarshalFromYamlForCodecs(config, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return err
			}
			joinCfg, ok := cfg.(*kubeadmv1beta2.JoinConfiguration)
			if !ok {
				return errors.New("expected JoinConfiguration")
			}
			if joinCfg.ControlPlane != nil {
				kdm.ControlPlane = true
			}
			kdm.JoinConfiguration = cfg
		case kubeadmutil.GroupVersionKindsHasKind(gvks, "ClusterConfiguration"):
			cfg, err := kubeadmutil.UnmarshalFromYamlForCodecs(config, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return err
			}
			kdm.ClusterConfiguration = cfg
		case kubeadmutil.GroupVersionKindsHasKind(gvks, "KubeletConfiguration"):
			err := kubeletconfigv1beta1scheme.AddToScheme(kubeadmscheme.Scheme)
			if err != nil {
				return err
			}
			cfg, err := kubeadmutil.UnmarshalFromYamlForCodecs(config, kubeletconfigv1beta1scheme.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return err
			}
			kdm.KubeletConfiguration = cfg
		case kubeadmutil.GroupVersionKindsHasKind(gvks, "KubeProxyConfiguration"):
			err := kubeproxyconfigv1alpha1scheme.AddToScheme(kubeadmscheme.Scheme)
			if err != nil {
				return err
			}
			cfg, err := kubeadmutil.UnmarshalFromYamlForCodecs(config, kubeproxyconfigv1alpha1scheme.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return err
			}
			kdm.KubeProxyConfiguration = cfg
		}
	}

	return nil
}

// IsControlPlane indicates if the current kubeadm configuration is a worker
// acting as a master.
func (kdm *Kubeadm) IsControlPlane() bool {
	return kdm.ControlPlane
}

// IsBootstrap indicates if the current kubeadm configuration is a master init
// configuration.
func (kdm *Kubeadm) IsBootstrap() bool {
	return kdm.IsControlPlane() && kdm.InitConfiguration != nil
}

// IsWorker indicates if the current kubeadm configuration is a worker
// configuration.
func (kdm *Kubeadm) IsWorker() bool {
	return !kdm.IsControlPlane()
}
