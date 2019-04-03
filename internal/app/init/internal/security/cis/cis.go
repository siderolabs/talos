/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cis

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"math/rand"
	"os"
	"text/template"
	"time"

	"github.com/talos-systems/talos/internal/pkg/constants"

	v1 "k8s.io/api/core/v1"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
)

const disabled = "false"

const auditPolicy string = `apiVersion: audit.k8s.io/v1beta1
kind: Policy
rules:
- level: Metadata
`

const encryptionConfig string = `kind: EncryptionConfig
apiVersion: v1
resources:
- resources:
  - secrets
  providers:
  - aescbc:
      keys:
      - name: key1
        secret: {{ .AESCBCEncryptionSecret }}
  - identity: {}
`

// EnforceAuditingRequirements enforces CIS requirements for auditing.
// TODO(andrewrynhard): Enable audit-log-maxbackup.
// TODO(andrewrynhard): Enable audit-log-maxsize.
func EnforceAuditingRequirements(cfg *kubeadmapi.InitConfiguration) error {
	if err := ioutil.WriteFile("/etc/kubernetes/audit-policy.yaml", []byte(auditPolicy), 0400); err != nil {
		return err
	}

	// TODO(andrewrynhard): We should log to a file, and the option to retrieve
	// the log files.
	cfg.APIServer.ExtraArgs["audit-log-path"] = "-"
	cfg.APIServer.ExtraArgs["audit-log-maxage"] = "30"
	cfg.APIServer.ExtraArgs["audit-log-maxbackup"] = "3"
	cfg.APIServer.ExtraArgs["audit-log-maxsize"] = "50"

	return nil
}

// EnforceSecretRequirements enforces CIS requirements for secrets.
func EnforceSecretRequirements(cfg *kubeadmapi.InitConfiguration) error {
	if _, err := os.Stat(constants.EncryptionConfigInitramfsPath); !os.IsNotExist(err) {
		return nil
	}

	random := func(min, max int) int {
		return rand.Intn(max-min) + min
	}
	var encryptionKeySecret string
	seed := time.Now().Unix()
	rand.Seed(seed)
	for i := 0; i < 32; i++ {
		n := random(0, 94)
		start := "!"
		encryptionKeySecret += string(start[0] + byte(n))
	}
	data := []byte(encryptionKeySecret)

	str := base64.StdEncoding.EncodeToString(data)
	aux := struct {
		AESCBCEncryptionSecret string
	}{
		AESCBCEncryptionSecret: str,
	}
	t, err := template.New("encryptionconfig").Parse(encryptionConfig)
	if err != nil {
		return err
	}

	encBytes := []byte{}
	buf := bytes.NewBuffer(encBytes)
	if err := t.Execute(buf, aux); err != nil {
		return err
	}
	if err := ioutil.WriteFile(constants.EncryptionConfigInitramfsPath, buf.Bytes(), 0400); err != nil {
		return err
	}
	cfg.APIServer.ExtraArgs["experimental-encryption-provider-config"] = constants.EncryptionConfigRootfsPath
	vol := kubeadmapi.HostPathMount{
		Name:      "encryptionconfig",
		HostPath:  constants.EncryptionConfigRootfsPath,
		MountPath: constants.EncryptionConfigRootfsPath,
		ReadOnly:  true,
		PathType:  v1.HostPathFile,
	}
	cfg.APIServer.ExtraVolumes = append(cfg.APIServer.ExtraVolumes, vol)

	return nil
}

// EnforceTLSRequirements enforces CIS requirements for TLS.
func EnforceTLSRequirements(cfg *kubeadmapi.InitConfiguration) error {
	// nolint: lll
	cfg.APIServer.ExtraArgs["tls-cipher-suites"] = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256"

	return nil
}

// EnforceAdmissionPluginsRequirements enforces CIS requirements for admission plugins.
// TODO(andrewrynhard): Include any extra user specified plugins.
// TODO(andrewrynhard): Enable EventRateLimit.
// TODO(andrewrynhard): Enable AlwaysPullImages (See https://github.com/kubernetes/kubernetes/issues/64333).
func EnforceAdmissionPluginsRequirements(cfg *kubeadmapi.InitConfiguration) error {
	// nolint: lll
	cfg.APIServer.ExtraArgs["enable-admission-plugins"] = "PodSecurityPolicy,NamespaceLifecycle,ServiceAccount,NodeRestriction,LimitRanger,DefaultStorageClass,DefaultTolerationSeconds,ResourceQuota"

	return nil
}

// EnforceExtraRequirements enforces miscellaneous CIS requirements.
// TODO(andrewrynhard): Enable anonymous-auth, see https://github.com/kubernetes/kubeadm/issues/798.
// TODO(andrewrynhard): Enable kubelet-certificate-authority, see https://github.com/kubernetes/kubeadm/issues/118#issuecomment-407202481.
func EnforceExtraRequirements(cfg *kubeadmapi.InitConfiguration) error {
	cfg.APIServer.ExtraArgs["profiling"] = disabled
	cfg.ControllerManager.ExtraArgs["profiling"] = disabled
	cfg.Scheduler.ExtraArgs["profiling"] = disabled

	cfg.APIServer.ExtraArgs["service-account-lookup"] = "true"

	return nil
}

// EnforceMasterRequirements enforces the CIS requirements for master nodes.
func EnforceMasterRequirements(cfg *kubeadmapi.InitConfiguration) error {
	ensureFieldsAreNotNil(cfg)

	if err := EnforceAuditingRequirements(cfg); err != nil {
		return err
	}
	if err := EnforceSecretRequirements(cfg); err != nil {
		return err
	}
	if err := EnforceTLSRequirements(cfg); err != nil {
		return err
	}
	if err := EnforceAdmissionPluginsRequirements(cfg); err != nil {
		return err
	}
	if err := EnforceExtraRequirements(cfg); err != nil {
		return err
	}

	return nil
}

// EnforceWorkerRequirements enforces the CIS requirements for master nodes.
func EnforceWorkerRequirements(cfg *kubeadmapi.JoinConfiguration) error {
	return nil
}

func ensureFieldsAreNotNil(cfg *kubeadmapi.InitConfiguration) {
	if cfg.APIServer.ExtraArgs == nil {
		cfg.APIServer.ExtraArgs = make(map[string]string)
	}
	if cfg.ControllerManager.ExtraArgs == nil {
		cfg.ControllerManager.ExtraArgs = make(map[string]string)
	}
	if cfg.Scheduler.ExtraArgs == nil {
		cfg.Scheduler.ExtraArgs = make(map[string]string)
	}

	if cfg.APIServer.ExtraVolumes == nil {
		cfg.APIServer.ExtraVolumes = make([]kubeadmapi.HostPathMount, 0)
	}

	if cfg.FeatureGates == nil {
		cfg.FeatureGates = make(map[string]bool)
	}
}
