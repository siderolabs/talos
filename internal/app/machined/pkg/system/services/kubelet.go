// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	cni "github.com/containerd/go-cni"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	internalcni "github.com/talos-systems/talos/internal/app/machined/internal/cni"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/constants"
	tnet "github.com/talos-systems/talos/pkg/net"
)

var kubeletKubeConfigTemplate = []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .Server }}
    certificate-authority-data: {{ .CACert }}
users:
- name: kubelet
  user:
    token: {{ .BootstrapTokenID }}.{{ .BootstrapTokenSecret }}
contexts:
- context:
    cluster: local
    user: kubelet
`)

// Kubelet implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubelet struct{}

// ID implements the Service interface.
func (k *Kubelet) ID(config runtime.Configurator) string {
	return "kubelet"
}

// PreFunc implements the Service interface.
func (k *Kubelet) PreFunc(ctx context.Context, config runtime.Configurator) error {
	cfg := struct {
		Server               string
		CACert               string
		BootstrapTokenID     string
		BootstrapTokenSecret string
	}{
		Server:               config.Cluster().Endpoint().String(),
		CACert:               base64.StdEncoding.EncodeToString(config.Cluster().CA().Crt),
		BootstrapTokenID:     config.Cluster().Token().ID(),
		BootstrapTokenSecret: config.Cluster().Token().Secret(),
	}

	templ := template.Must(template.New("tmpl").Parse(string(kubeletKubeConfigTemplate)))

	var buf bytes.Buffer

	if err := templ.Execute(&buf, cfg); err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubeletBootstrapKubeconfig, buf.Bytes(), 0600); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(constants.KubernetesCACert), 0700); err != nil {
		return err
	}

	if err := os.MkdirAll(cni.DefaultCNIDir, 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(constants.KubernetesCACert, config.Cluster().CA().Crt, 0500); err != nil {
		return err
	}

	if err := writeKubeletConfig(config); err != nil {
		return err
	}

	client, err := containerdapi.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, "k8s.io")

	_, err = image.Pull(containerdctx, config.Machine().Registries(), client, config.Machine().Kubelet().Image())
	if err != nil {
		return err
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (k *Kubelet) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (k *Kubelet) DependsOn(config runtime.Configurator) []string {
	return []string{"containerd", "networkd"}
}

// Runner implements the Service interface.
func (k *Kubelet) Runner(config runtime.Configurator) (runner.Runner, error) {
	a, err := k.args(config)
	if err != nil {
		return nil, err
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          k.ID(config),
		ProcessArgs: append([]string{"/hyperkube", "kubelet"}, a...),
	}
	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "sysfs", Destination: "/sys", Source: "/sys", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/sys/fs/cgroup", Source: "/sys/fs/cgroup", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/lib/modules", Source: "/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/usr/libexec/kubernetes", Source: "/usr/libexec/kubernetes", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/containerd", Source: "/var/lib/containerd", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/opt/cni/bin", Source: "/opt/cni/bin", Options: []string{"rbind", "rshared", "rw"}},
	}

	// Add in the additional CNI mounts.
	cniMounts, err := internalcni.Mounts(config)
	if err != nil {
		return nil, err
	}

	mounts = append(mounts, cniMounts...)

	// Add extra mounts.
	// TODO(andrewrynhard): We should verify that the mount source is
	// whitelisted. There is the potential that a user can expose
	// sensitive information.
	mounts = append(mounts, config.Machine().Kubelet().ExtraMounts()...)

	env := []string{}
	for key, val := range config.Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		config.Debug(),
		&args,
		runner.WithNamespace(criconstants.K8sContainerdNamespace),
		runner.WithContainerImage(config.Machine().Kubelet().Image()),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithRootfsPropagation("shared"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (k *Kubelet) HealthFunc(runtime.Configurator) health.Check {
	return func(ctx context.Context) error {
		req, err := http.NewRequest("GET", "http://127.0.0.1:10248/healthz", nil)
		if err != nil {
			return err
		}

		req = req.WithContext(ctx)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		// nolint: errcheck
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("expected HTTP status OK, got %s", resp.Status)
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface
func (k *Kubelet) HealthSettings(runtime.Configurator) *health.Settings {
	settings := health.DefaultSettings
	settings.InitialDelay = 2 * time.Second // increase initial delay as kubelet is slow on startup

	return &settings
}

func newKubeletConfiguration(clusterDNS []string) *kubeletconfig.KubeletConfiguration {
	f := false
	t := true

	return &kubeletconfig.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubelet.config.k8s.io/v1beta1",
			Kind:       "KubeletConfiguration",
		},
		StaticPodPath:      "/etc/kubernetes/manifests",
		Address:            "0.0.0.0",
		Port:               10250,
		ReadOnlyPort:       10255, // TODO(andrewrynhard): Disable this.
		RotateCertificates: true,
		Authentication: kubeletconfig.KubeletAuthentication{
			X509: kubeletconfig.KubeletX509Authentication{
				ClientCAFile: constants.KubernetesCACert,
			},
			Webhook: kubeletconfig.KubeletWebhookAuthentication{
				Enabled: &t,
			},
			Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
				Enabled: &f,
			},
		},
		Authorization: kubeletconfig.KubeletAuthorization{
			Mode: kubeletconfig.KubeletAuthorizationModeWebhook,
		},
		ClusterDomain:       "cluster.local",
		ClusterDNS:          clusterDNS,
		SerializeImagePulls: &f,
		FailSwapOn:          &f,
	}
}

// nolint: gocyclo
func (k *Kubelet) args(config runtime.Configurator) ([]string, error) {
	blackListArgs := argsbuilder.Args{
		"bootstrap-kubeconfig":       constants.KubeletBootstrapKubeconfig,
		"kubeconfig":                 constants.KubeletKubeconfig,
		"container-runtime":          "remote",
		"container-runtime-endpoint": "unix://" + constants.ContainerdAddress,
		"config":                     "/etc/kubernetes/kubelet.yaml",
		"dynamic-config-dir":         "/etc/kubernetes/kubelet",

		"cert-dir":     "/var/lib/kubelet/pki",
		"cni-conf-dir": cni.DefaultNetDir,
	}

	extraArgs := argsbuilder.Args(config.Machine().Kubelet().ExtraArgs())

	for k := range blackListArgs {
		if extraArgs.Contains(k) {
			return nil, argsbuilder.NewBlacklistError(k)
		}
	}

	return blackListArgs.Merge(extraArgs).Args(), nil
}

func writeKubeletConfig(config runtime.Configurator) error {
	dnsServiceIPs := []string{}

	for _, cidr := range strings.Split(config.Cluster().Network().ServiceCIDR(), ",") {
		_, svcCIDR, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("failed to parse service CIDR %s: %v", cidr, err)
		}

		dnsIP, err := tnet.NthIPInNetwork(svcCIDR, 10)
		if err != nil {
			return fmt.Errorf("failed to calculate Nth IP in CIDR %s: %v", svcCIDR, err)
		}

		dnsServiceIPs = append(dnsServiceIPs, dnsIP.String())
	}

	kubeletConfiguration := newKubeletConfiguration(dnsServiceIPs)

	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		nil,
		nil,
		json.SerializerOptions{
			Yaml:   true,
			Pretty: true,
			Strict: true,
		},
	)

	var buf bytes.Buffer

	if err := serializer.Encode(kubeletConfiguration, &buf); err != nil {
		return err
	}

	if err := ioutil.WriteFile("/etc/kubernetes/kubelet.yaml", buf.Bytes(), 0600); err != nil {
		return err
	}

	return nil
}
