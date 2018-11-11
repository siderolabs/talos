package cis

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"

	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
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
func EnforceAuditingRequirements(cfg *kubeadm.InitConfiguration) error {
	if err := ioutil.WriteFile("/var/etc/kubernetes/audit-policy.yaml", []byte(auditPolicy), 0400); err != nil {
		return err
	}
	maxAge := int32(30)
	if cfg.FeatureGates == nil {
		cfg.FeatureGates = make(map[string]bool)
	}
	cfg.FeatureGates["Auditing"] = true
	cfg.ClusterConfiguration.AuditPolicyConfiguration.Path = "/etc/kubernetes/audit-policy.yaml"
	cfg.ClusterConfiguration.AuditPolicyConfiguration.LogDir = "/etc/kubernetes/logs"
	cfg.ClusterConfiguration.AuditPolicyConfiguration.LogMaxAge = &maxAge

	return nil
}

// EnforceSecretRequirements enforces CIS requirements for secrets.
func EnforceSecretRequirements(cfg *kubeadm.InitConfiguration) error {
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
	if err := ioutil.WriteFile("/var/etc/kubernetes/encryptionconfig.yaml", buf.Bytes(), 0400); err != nil {
		return err
	}
	cfg.APIServerExtraArgs["experimental-encryption-provider-config"] = "/etc/kubernetes/encryptionconfig.yaml"
	vol := kubeadm.HostPathMount{
		Name:      "encryptionconfig",
		HostPath:  "/etc/kubernetes/encryptionconfig.yaml",
		MountPath: "/etc/kubernetes/encryptionconfig.yaml",
		Writable:  false,
		PathType:  v1.HostPathFile,
	}
	if cfg.APIServerExtraVolumes == nil {
		cfg.APIServerExtraVolumes = make([]kubeadm.HostPathMount, 0)
	}
	cfg.APIServerExtraVolumes = append(cfg.APIServerExtraVolumes, vol)

	return nil
}

// EnforceTLSRequirements enforces CIS requirements for TLS.
func EnforceTLSRequirements(cfg *kubeadm.InitConfiguration) error {
	// nolint: lll
	cfg.APIServerExtraArgs["tls-cipher-suites"] = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256"

	return nil
}

// EnforceAdmissionPluginsRequirements enforces CIS requirements for admission plugins.
// TODO(andrewrynhard): Include any extra user specified plugins.
// TODO(andrewrynhard): Enable PodSecurityPolicy.
// TODO(andrewrynhard): Enable EventRateLimit.
func EnforceAdmissionPluginsRequirements(cfg *kubeadm.InitConfiguration) error {
	// nolint: lll
	cfg.APIServerExtraArgs["enable-admission-plugins"] = "AlwaysPullImages,SecurityContextDeny,DenyEscalatingExec,NamespaceLifecycle,ServiceAccount,NodeRestriction,LimitRanger,DefaultStorageClass,DefaultTolerationSeconds,ResourceQuota"

	return nil
}

// EnforceExtraRequirements enforces miscellaneous CIS requirements.
// TODO(andrewrynhard): Enable anonymous-auth, see https://github.com/kubernetes/kubeadm/issues/798.
// TODO(andrewrynhard): Enable kubelet-certificate-authority, see https://github.com/kubernetes/kubeadm/issues/118#issuecomment-407202481.
func EnforceExtraRequirements(cfg *kubeadm.InitConfiguration) error {
	if cfg.APIServerExtraArgs == nil {
		cfg.APIServerExtraArgs = make(map[string]string)
	}
	if cfg.ControllerManagerExtraArgs == nil {
		cfg.ControllerManagerExtraArgs = make(map[string]string)
	}
	if cfg.SchedulerExtraArgs == nil {
		cfg.SchedulerExtraArgs = make(map[string]string)
	}
	cfg.APIServerExtraArgs["profiling"] = disabled
	cfg.ControllerManagerExtraArgs["profiling"] = disabled
	cfg.SchedulerExtraArgs["profiling"] = disabled

	cfg.APIServerExtraArgs["service-account-lookup"] = "true"

	return nil
}

// EnforceMasterRequirements enforces the CIS requirements for master nodes.
func EnforceMasterRequirements() error {
	cfg := &kubeadmapiv1beta1.InitConfiguration{}
	internalCfg, err := configutil.ConfigFileAndDefaultsToInternalConfig(constants.KubeadmConfig, cfg)
	if err != nil {
		return err
	}

	if err := EnforceAuditingRequirements(internalCfg); err != nil {
		return err
	}
	if err := EnforceSecretRequirements(internalCfg); err != nil {
		return err
	}
	if err := EnforceTLSRequirements(internalCfg); err != nil {
		return err
	}
	if err := EnforceAdmissionPluginsRequirements(internalCfg); err != nil {
		return err
	}
	if err := EnforceExtraRequirements(internalCfg); err != nil {
		return err
	}

	b, err := configutil.MarshalInitConfigurationToBytes(internalCfg, kubeadmapiv1beta1.SchemeGroupVersion)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubeadmConfig, b, 0600); err != nil {
		return err
	}

	return nil
}

// EnforceWorkerRequirements enforces the CIS requirements for master nodes.
func EnforceWorkerRequirements() error {
	cfg := &kubeadmapiv1beta1.JoinConfiguration{}
	internalCfg, err := configutil.JoinConfigFileAndDefaultsToInternalConfig(constants.KubeadmConfig, cfg)
	if err != nil {
		return err
	}

	b, err := kubeadmutil.MarshalToYamlForCodecs(internalCfg, kubeadm.SchemeGroupVersion, scheme.Codecs)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubeadmConfig, b, 0600); err != nil {
		return err
	}

	return nil
}
