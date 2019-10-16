/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
	"text/template"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/internal/cni"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/runtime"
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

	if err := ioutil.WriteFile(constants.KubernetesCACert, config.Cluster().CA().Crt, 0500); err != nil {
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

	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, config.Cluster().Version())
	if _, err = client.Pull(containerdctx, image, containerdapi.WithPullUnpack); err != nil {
		return fmt.Errorf("failed to pull image %q: %w", image, err)
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubelet) PostFunc(config runtime.Configurator) (err error) {
	return nil
}

// Condition implements the Service interface.
func (k *Kubelet) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (k *Kubelet) DependsOn(config runtime.Configurator) []string {
	return []string{"containerd"}
}

// Runner implements the Service interface.
func (k *Kubelet) Runner(config runtime.Configurator) (runner.Runner, error) {
	image := fmt.Sprintf("%s:v%s", constants.KubernetesImage, config.Cluster().Version())

	_, serviceCIDR, err := net.ParseCIDR(config.Cluster().Network().ServiceCIDR())
	if err != nil {
		return nil, err
	}

	dnsServiceIP, err := tnet.NthIPInNetwork(serviceCIDR, 10)
	if err != nil {
		return nil, err
	}

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(config),
		ProcessArgs: []string{
			"/hyperkube",
			"kubelet",
			"--bootstrap-kubeconfig=" + constants.KubeletBootstrapKubeconfig,
			"--kubeconfig=" + constants.KubeletKubeconfig,
			"--container-runtime=remote",
			"--container-runtime-endpoint=unix://" + constants.ContainerdAddress,
			"--anonymous-auth=false",
			"--cert-dir=/var/lib/kubelet/pki",
			"--client-ca-file=" + constants.KubernetesCACert,
			"--cni-conf-dir=/etc/cni/net.d",
			"--cluster-domain=cluster.local",
			"--pod-manifest-path=/etc/kubernetes/manifests",
			"--rotate-certificates",
			"--cluster-dns=" + dnsServiceIP.String(),
			// TODO(andrewrynhard): Only set this in the case of container run mode.
			"--fail-swap-on=false",
		},
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
	}

	// Add in the additional CNI mounts.
	cniMounts, err := cni.Mounts(config)
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
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithRootfsPropagation("shared"),
			oci.WithMounts(mounts),
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
