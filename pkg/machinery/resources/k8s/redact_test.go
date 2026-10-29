// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// TestControlPlaneConfigRedaction covers the control plane configs: the flags, the variables and the
// volumes stay readable, only the values which came from the machine configuration are taken out.
func TestControlPlaneConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.APIServerConfigSpec{
		Image:                "registry.k8s.io/kube-apiserver:v1.34.0",
		ControlPlaneEndpoint: "https://localhost:6443",
		ExtraArgs: map[string]k8s.ArgValues{
			"oidc-client-secret": {Values: []string{"s3cret"}},
		},
		Args: []string{
			"/usr/local/bin/kube-apiserver",
			"--advertise-address=10.5.0.2",
			"--allow-privileged",
		},
		EnvironmentVariables: map[string]string{
			"AWS_SECRET_ACCESS_KEY": "wJalrXU",
		},
		ExtraVolumes: []k8s.ExtraVolume{
			{Name: "audit", HostPath: "/var/log/audit", MountPath: "/var/log/audit"},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	// the names survive: which flags are set is what makes the resource worth reading
	assert.Equal(t, []string{constants.Redacted}, redacted.ExtraArgs["oidc-client-secret"].Values)
	assert.Equal(t, []string{
		"/usr/local/bin/kube-apiserver",
		"--advertise-address=" + constants.Redacted,
		"--allow-privileged",
	}, redacted.Args)
	assert.Equal(t, map[string]string{"AWS_SECRET_ACCESS_KEY": constants.Redacted}, redacted.EnvironmentVariables)

	// and the rest of the spec is left alone
	assert.Equal(t, spec.Image, redacted.Image)
	assert.Equal(t, spec.ControlPlaneEndpoint, redacted.ControlPlaneEndpoint)
	assert.Equal(t, spec.ExtraVolumes, redacted.ExtraVolumes)

	// redacting the copy may not reach back into the spec it was copied from
	assert.Equal(t, []string{"s3cret"}, spec.ExtraArgs["oidc-client-secret"].Values)
	assert.Equal(t, "wJalrXU", spec.EnvironmentVariables["AWS_SECRET_ACCESS_KEY"])
	assert.Equal(t, "--advertise-address=10.5.0.2", spec.Args[1])
}

// TestControlPlaneConfigRedactionEmpty makes sure a config with nothing configured on it is not
// reported as carrying a secret: the annotation the API adds means something was taken away.
func TestControlPlaneConfigRedactionEmpty(t *testing.T) {
	t.Parallel()

	assert.False(t, (&k8s.AdmissionControlConfigSpec{Config: []k8s.AdmissionPluginSpec{{Name: "PodSecurity"}}}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.APIServerConfigSpec{Image: "kube-apiserver"}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.ControllerManagerConfigSpec{Image: "kube-controller-manager"}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.SchedulerConfigSpec{Image: "kube-scheduler"}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.ExtraManifestsConfigSpec{}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.StaticPodSpec{}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.ManifestSpec{}).HasSecrets(resource.Metadata{}))
}

// TestAdmissionControlConfigRedaction covers the admission plugin configuration, which is dropped
// whole: the plugin names say which plugins are configured, and that is what is left.
func TestAdmissionControlConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.AdmissionControlConfigSpec{
		Config: []k8s.AdmissionPluginSpec{
			{
				Name: "PodSecurity",
				Configuration: map[string]any{
					"apiVersion": "pod-security.admission.config.k8s.io/v1alpha1",
					"defaults":   map[string]any{"enforce": "baseline"},
				},
			},
			{
				Name: "ImagePolicyWebhook",
			},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, "PodSecurity", redacted.Config[0].Name)
	assert.Nil(t, redacted.Config[0].Configuration)
	assert.Equal(t, spec.Config[1], redacted.Config[1])

	assert.NotEmpty(t, spec.Config[0].Configuration, "redacting the copy should not empty the spec it was copied from")
}

// TestSchedulerConfigRedaction covers the free-form scheduler configuration, which is dropped whole.
func TestSchedulerConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.SchedulerConfigSpec{
		Image:  "registry.k8s.io/kube-scheduler:v1.34.0",
		Config: map[string]any{"apiVersion": "kubescheduler.config.k8s.io/v1"},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Nil(t, redacted.Config)
	assert.Equal(t, spec.Image, redacted.Image)
	assert.NotEmpty(t, spec.Config, "redacting the copy should not empty the spec it was copied from")
}

// TestExtraManifestsConfigRedaction covers the manifest download: the headers authenticate it, an
// inline manifest may be a Secret, and the URL itself may carry credentials.
func TestExtraManifestsConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.ExtraManifestsConfigSpec{
		ExtraManifests: []k8s.ExtraManifest{
			{
				Name:         "cni",
				URL:          "https://user:hunter2@example.com/cni.yaml",
				Priority:     "05",
				ExtraHeaders: map[string]string{"Authorization": "Bearer token"},
			},
			{
				Name:           "inline",
				InlineManifest: "apiVersion: v1\nkind: Secret\n",
			},
			{
				Name: "plain",
				URL:  "https://example.com/plain.yaml",
			},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, "https://user:"+constants.Redacted+"@example.com/cni.yaml", redacted.ExtraManifests[0].URL)
	assert.Equal(t, map[string]string{"Authorization": constants.Redacted}, redacted.ExtraManifests[0].ExtraHeaders)
	assert.Equal(t, "cni", redacted.ExtraManifests[0].Name)
	assert.Equal(t, "05", redacted.ExtraManifests[0].Priority)

	assert.Equal(t, constants.Redacted, redacted.ExtraManifests[1].InlineManifest)

	// a manifest which carries nothing sensitive comes through as it is
	assert.Equal(t, spec.ExtraManifests[2], redacted.ExtraManifests[2])

	assert.Equal(t, "Bearer token", spec.ExtraManifests[0].ExtraHeaders["Authorization"],
		"redacting the copy should not mutate the spec it was copied from")
}

// TestManifestRedaction covers the rendered manifests: everything but the fields naming the object
// goes, so the reader still sees which objects Talos applies.
func TestManifestRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.ManifestSpec{
		Items: []k8s.SingleManifest{
			{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":        "registry-credentials",
						"namespace":   "kube-system",
						"annotations": map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "{...}"},
					},
					"data": map[string]any{".dockerconfigjson": "eyJhdXRocyI6"},
				},
			},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      "registry-credentials",
			"namespace": "kube-system",
		},
	}, redacted.Items[0].Object)

	assert.Equal(t, "eyJhdXRocyI6", spec.Items[0].Object["data"].(map[string]any)[".dockerconfigjson"],
		"redacting the copy should not mutate the spec it was copied from")
}

// TestManifestRedactionNothingToTakeOut covers a manifest which is already down to its name: it holds
// nothing to redact, so it is served as it is rather than annotated as redacted.
func TestManifestRedactionNothingToTakeOut(t *testing.T) {
	t.Parallel()

	spec := k8s.ManifestSpec{
		Items: []k8s.SingleManifest{
			{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata":   map[string]any{"name": "ci"},
				},
			},
		},
	}

	assert.False(t, spec.HasSecrets(resource.Metadata{}))
}

// TestStaticPodRedaction covers the rendered pod, which carries the same values once more, in a shape
// no struct tag can reach.
func TestStaticPodRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.StaticPodSpec{Pod: apiServerPod()}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	container := podContainer(t, redacted.Pod)

	assert.Equal(t, []any{
		"/usr/local/bin/kube-apiserver",
		"--advertise-address=" + constants.Redacted,
		"--allow-privileged",
	}, container["command"])
	assert.Equal(t, []any{
		map[string]any{"name": "AWS_SECRET_ACCESS_KEY", "value": constants.Redacted},
	}, container["env"])

	// everything which is not a value stays: the pod is still recognizable
	assert.Equal(t, "kube-apiserver", container["name"])
	assert.Equal(t, "registry.k8s.io/kube-apiserver:v1.34.0", container["image"])

	// the deep copy of the spec only copies the top-level map, so redaction has to copy its way down
	// rather than write into the pod it was handed
	original := podContainer(t, spec.Pod)
	assert.Equal(t, "--advertise-address=10.5.0.2", original["command"].([]any)[1])
	assert.Equal(t, "wJalrXU", original["env"].([]any)[0].(map[string]any)["value"])
}

// TestStaticPodRedactionNotAPod makes sure a pod of an unexpected shape is handled: nothing is found
// to redact, and nothing panics on the way.
func TestStaticPodRedactionNotAPod(t *testing.T) {
	t.Parallel()

	for _, pod := range []map[string]any{
		nil,
		{},
		{"spec": "not a map"},
		{"spec": map[string]any{"containers": "not a list"}},
		{"spec": map[string]any{"containers": []any{"not a container"}}},
		{"spec": map[string]any{"containers": []any{map[string]any{"env": []any{map[string]any{"name": "EMPTY"}}}}}},
	} {
		spec := k8s.StaticPodSpec{Pod: pod}

		assert.False(t, spec.HasSecrets(resource.Metadata{}))

		spec.RedactSecrets(resource.Metadata{})
	}
}

func apiServerPod() map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "kube-apiserver"},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "kube-apiserver",
					"image": "registry.k8s.io/kube-apiserver:v1.34.0",
					"command": []any{
						"/usr/local/bin/kube-apiserver",
						"--advertise-address=10.5.0.2",
						"--allow-privileged",
					},
					"env": []any{
						map[string]any{"name": "AWS_SECRET_ACCESS_KEY", "value": "wJalrXU"},
					},
				},
			},
		},
	}
}

func podContainer(t *testing.T, pod map[string]any) map[string]any {
	t.Helper()

	podSpec, ok := pod["spec"].(map[string]any)
	require.True(t, ok)

	containers, ok := podSpec["containers"].([]any)
	require.True(t, ok)
	require.Len(t, containers, 1)

	container, ok := containers[0].(map[string]any)
	require.True(t, ok)

	return container
}

// kubeletConfigFixture returns a kubelet configuration document carrying a static pod URL header.
func kubeletConfigFixture() map[string]any {
	return map[string]any{
		"maxPods":            int64(250),
		"staticPodURLHeader": map[string]any{"Authorization": []any{"Bearer s3cret"}},
		"systemReserved":     map[string]any{"memory": "512Mi"},
	}
}

// credentialProviderConfigFixture returns an image credential provider configuration with a provider
// which takes its credentials from the environment and the command line.
func credentialProviderConfigFixture() map[string]any {
	return map[string]any{
		"apiVersion": "kubelet.config.k8s.io/v1",
		"kind":       "CredentialProviderConfig",
		"providers": []any{
			map[string]any{
				"name":                 "ecr-credential-provider",
				"matchImages":          []any{"*.dkr.ecr.*.amazonaws.com"},
				"args":                 []any{"get-credentials", "--role-arn=arn:aws:iam::123:role/pull"},
				"env":                  []any{map[string]any{"name": "AWS_SECRET_ACCESS_KEY", "value": "wJalrXU"}},
				"defaultCacheDuration": "12h",
			},
		},
	}
}

// assertKubeletConfigRedacted checks the redacted forms of the two fixtures above.
func assertKubeletConfigRedacted(t *testing.T, config, credentialProviderConfig map[string]any) {
	t.Helper()

	// only the header values go, the rest of the kubelet configuration is what the resource is read for
	assert.Equal(t, map[string]any{
		"maxPods":            int64(250),
		"staticPodURLHeader": map[string]any{"Authorization": []any{constants.Redacted}},
		"systemReserved":     map[string]any{"memory": "512Mi"},
	}, config)

	provider := credentialProviderConfig["providers"].([]any)[0].(map[string]any) //nolint:forcetypeassert,errcheck

	assert.Equal(t, "ecr-credential-provider", provider["name"])
	assert.Equal(t, []any{"*.dkr.ecr.*.amazonaws.com"}, provider["matchImages"])
	assert.Equal(t, []any{"get-credentials", "--role-arn=" + constants.Redacted}, provider["args"])
	assert.Equal(t, []any{map[string]any{"name": "AWS_SECRET_ACCESS_KEY", "value": constants.Redacted}}, provider["env"])
	assert.Equal(t, "kubelet.config.k8s.io/v1", credentialProviderConfig["apiVersion"])
}

// TestKubeletConfigRedaction covers the kubelet config: the extra arguments lose their values, the
// kubelet configuration and the credential provider configuration lose the values carrying credentials.
func TestKubeletConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.KubeletConfigSpec{
		Image:                    "ghcr.io/siderolabs/kubelet:v1.34.0",
		ClusterDNS:               []string{"10.96.0.10"},
		ExtraArgs:                map[string]k8s.ArgValues{"node-labels": {Values: []string{"role=storage"}}},
		ExtraConfig:              kubeletConfigFixture(),
		CredentialProviderConfig: credentialProviderConfigFixture(),
		RegisterWithTaints:       map[string]string{"dedicated": "storage:NoSchedule"},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, []string{constants.Redacted}, redacted.ExtraArgs["node-labels"].Values)
	assertKubeletConfigRedacted(t, redacted.ExtraConfig, redacted.CredentialProviderConfig)

	assert.Equal(t, spec.Image, redacted.Image)
	assert.Equal(t, spec.ClusterDNS, redacted.ClusterDNS)
	assert.Equal(t, spec.RegisterWithTaints, redacted.RegisterWithTaints)

	// the deep copy shares everything below the top level of the free-form maps, so this is what
	// catches a redaction written in place
	assert.Equal(t, kubeletConfigFixture(), spec.ExtraConfig)
	assert.Equal(t, credentialProviderConfigFixture(), spec.CredentialProviderConfig)
	assert.Equal(t, []string{"role=storage"}, spec.ExtraArgs["node-labels"].Values)
}

// TestKubeletSpecRedaction covers the rendered kubelet spec, which carries the same values once more.
func TestKubeletSpecRedaction(t *testing.T) {
	t.Parallel()

	spec := k8s.KubeletSpecSpec{
		Image: "ghcr.io/siderolabs/kubelet:v1.34.0",
		Args: []string{
			"--config=/etc/kubernetes/kubelet.yaml",
			"--node-labels=role=storage",
			"--rotate-server-certificates",
		},
		ExpectedNodename:         "worker-1",
		Config:                   kubeletConfigFixture(),
		CredentialProviderConfig: credentialProviderConfigFixture(),
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, []string{
		"--config=" + constants.Redacted,
		"--node-labels=" + constants.Redacted,
		"--rotate-server-certificates",
	}, redacted.Args)
	assertKubeletConfigRedacted(t, redacted.Config, redacted.CredentialProviderConfig)

	assert.Equal(t, spec.Image, redacted.Image)
	assert.Equal(t, spec.ExpectedNodename, redacted.ExpectedNodename)

	assert.Equal(t, kubeletConfigFixture(), spec.Config)
	assert.Equal(t, credentialProviderConfigFixture(), spec.CredentialProviderConfig)
	assert.Equal(t, "--node-labels=role=storage", spec.Args[1])
}

// TestKubeletRedactionEmpty makes sure a kubelet with no credential configured on it is not reported
// as carrying a secret.
func TestKubeletRedactionEmpty(t *testing.T) {
	t.Parallel()

	assert.False(t, (&k8s.KubeletConfigSpec{
		Image:       "kubelet",
		ExtraConfig: map[string]any{"maxPods": int64(250)},
		CredentialProviderConfig: map[string]any{
			"providers": []any{map[string]any{"name": "ecr-credential-provider", "args": []any{"get-credentials"}}},
		},
	}).HasSecrets(resource.Metadata{}))
	assert.False(t, (&k8s.KubeletSpecSpec{
		Image:  "kubelet",
		Args:   []string{"--rotate-server-certificates"},
		Config: map[string]any{"maxPods": int64(250)},
	}).HasSecrets(resource.Metadata{}))
}
