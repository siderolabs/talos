// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/container"
)

// load parses a single-document machine configuration and returns the container document.
func load(t *testing.T, doc string) *container.ContainerConfigV1Alpha1 {
	t.Helper()

	provider, err := configloader.NewFromBytes([]byte(doc))
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	cfg, ok := docs[0].(*container.ContainerConfigV1Alpha1)
	require.True(t, ok, "expected a ContainerConfig document, got %T", docs[0])

	return cfg
}

func TestContainerConfigMinimal(t *testing.T) {
	t.Parallel()

	cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: nginx
image: nginx
`)

	warnings, err := cfg.Validate(validationMode{})
	require.NoError(t, err)

	// A bare `nginx` is legal and normalized; it is also mutable, so it warns.
	assert.Equal(t, "index.docker.io/library/nginx:latest", cfg.Image())
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "not digest-pinned")

	// Defaults must be non-nil so callers never have to nil-check.
	assert.Equal(t, "restricted", string(cfg.Security().Profile()))
	assert.Equal(t, "default", string(cfg.Security().SeccompProfile()))
	assert.Equal(t, "none", string(cfg.Network().Mode()))
	assert.False(t, cfg.Resources().MemoryLimit().IsPresent())
	assert.Empty(t, cfg.DependsOn().Paths())
	assert.Equal(t, 10*time.Second, cfg.StopTimeout())
	assert.Equal(t, config.ContainerRestartPolicyAlways, cfg.Restart().Policy())
	assert.False(t, cfg.Restart().MaxRetries().IsPresent())
	assert.Equal(t, 5*time.Second, cfg.Restart().Delay())
	assert.Equal(t, 5*time.Second, cfg.Restart().MaxDelay())
}

func TestContainerConfigDependsOnDeprecatedNetworks(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		doc      string
		expected []string
	}{
		{
			name:     "networks only",
			doc:      "lifecycle:\n  dependsOn:\n    networks: [addresses]",
			expected: []string{"addresses"},
		},
		{
			name:     "network wins over networks",
			doc:      "lifecycle:\n  dependsOn:\n    network: [connectivity]\n    networks: [addresses]",
			expected: []string{"connectivity"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, "apiVersion: v1alpha1\nkind: ContainerConfig\nname: nginx\nimage: nginx\n"+test.doc+"\n")

			_, err := cfg.Validate(validationMode{})
			require.NoError(t, err)

			assert.Equal(t, test.expected, cfg.DependsOn().Network())
		})
	}
}

func TestContainerConfigDeprecatedDependsOn(t *testing.T) {
	t.Parallel()

	const header = "apiVersion: v1alpha1\nkind: ContainerConfig\nname: nginx\nimage: nginx@sha256:" +
		"0000000000000000000000000000000000000000000000000000000000000000\n"

	t.Run("top-level dependsOn is still honored, with a warning", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, header+"dependsOn:\n  paths: [/var/mnt/data]\n  network: [addresses]\n  time: true\n  containers: [database]\n")

		warnings, err := cfg.Validate(validationMode{})
		require.NoError(t, err)

		assert.Equal(t, []string{`container "nginx": dependsOn is deprecated, use lifecycle.dependsOn instead`}, warnings)
		assert.Equal(t, []string{"/var/mnt/data"}, cfg.DependsOn().Paths())
		assert.Equal(t, []string{"addresses"}, cfg.DependsOn().Network())
		assert.True(t, cfg.DependsOn().Time())
		assert.Equal(t, []string{"database"}, cfg.DependsOn().Containers())
	})

	// Both deprecations at once, as in a v1.14 configuration: top-level dependsOn with networks.
	t.Run("top-level dependsOn with the deprecated networks key", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, header+"dependsOn:\n  networks: [connectivity]\n")

		_, err := cfg.Validate(validationMode{})
		require.NoError(t, err)

		assert.Equal(t, []string{"connectivity"}, cfg.DependsOn().Network())
	})

	t.Run("lifecycle.dependsOn does not warn", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, header+"lifecycle:\n  dependsOn:\n    paths: [/var/mnt/data]\n")

		warnings, err := cfg.Validate(validationMode{})
		require.NoError(t, err)

		assert.Empty(t, warnings)
		assert.Equal(t, []string{"/var/mnt/data"}, cfg.DependsOn().Paths())
	})

	t.Run("an empty lifecycle falls back to the top-level dependsOn", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, header+"dependsOn:\n  time: true\nlifecycle: {}\n")

		_, err := cfg.Validate(validationMode{})
		require.NoError(t, err)

		assert.True(t, cfg.DependsOn().Time())
	})

	t.Run("both are rejected", func(t *testing.T) {
		t.Parallel()

		cfg := load(t, header+"dependsOn:\n  time: true\nlifecycle:\n  dependsOn:\n    paths: [/var/mnt/data]\n")

		_, err := cfg.Validate(validationMode{})
		require.ErrorContains(t, err, "dependsOn and lifecycle.dependsOn are mutually exclusive")
	})
}

func TestContainerConfigDeprecatedUserVolumeMount(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name                string
		mount               string
		expectedName        string
		expectedDestination string
	}{
		{
			name:                "userVolume only",
			mount:               "  - userVolume:\n      name: data\n      destination: /data",
			expectedName:        "data",
			expectedDestination: "/data",
		},
		{
			name:                "volume wins over userVolume",
			mount:               "  - volume:\n      name: new\n      destination: /new\n    userVolume:\n      name: old\n      destination: /old",
			expectedName:        "new",
			expectedDestination: "/new",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, "apiVersion: v1alpha1\nkind: ContainerConfig\nname: nginx\nimage: nginx\nmounts:\n"+test.mount+"\n")

			_, err := cfg.Validate(validationMode{})
			require.NoError(t, err)

			mounts := cfg.Mounts()
			require.Len(t, mounts, 1)

			volume, ok := mounts[0].Volume().Get()
			require.True(t, ok)

			assert.Equal(t, test.expectedName, volume.Name())
			assert.Equal(t, test.expectedDestination, volume.Destination())
		})
	}
}

func TestContainerConfigDigestPinnedDoesNotWarn(t *testing.T) {
	t.Parallel()

	cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: nginx
image: docker.io/library/nginx@sha256:0000000000000000000000000000000000000000000000000000000000000000
`)

	warnings, err := cfg.Validate(validationMode{})
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestContainerConfigFull(t *testing.T) {
	t.Parallel()

	cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: director
image: ghcr.io/siderolabs/director:v1.0.0
entrypoint: ["/director"]
args: ["--verbose"]
workingDir: /srv
runAs:
  uid: 65534
  gid: 65534
environment:
  - LOG_LEVEL=debug
mounts:
  - volume:
      name: director-data
      destination: /var/lib/director
  - tmpfs:
      destination: /tmp
      size: 64MiB
  - hostPath:
      source: /dev
      destination: /dev
security:
  profile: privileged
  capabilities:
    add: [NET_ADMIN]
    drop: [ALL]
  seccompProfile: unconfined
network:
  mode: host
resources:
  limits:
    cpu: 1500m
    memory: 512MiB
    swap: "0"
lifecycle:
  dependsOn:
    paths:
      - /var/mnt/director-data
    network:
      - addresses
    time: true
  healthCheck:
    http:
      port: 8080
      path: /healthz
    interval: 30s
  stopTimeout: 2m
`)

	_, err := cfg.Validate(validationMode{})
	require.NoError(t, err)

	assert.Equal(t, "director", cfg.Name())
	assert.Equal(t, []string{"/director"}, cfg.Entrypoint())
	assert.Equal(t, "/srv", cfg.WorkingDir())
	assert.Equal(t, "privileged", string(cfg.Security().Profile()))
	assert.Equal(t, "unconfined", string(cfg.Security().SeccompProfile()))
	assert.Equal(t, "host", string(cfg.Network().Mode()))
	assert.True(t, cfg.DependsOn().Time())
	assert.Equal(t, 2*time.Minute, cfg.StopTimeout())

	uid, ok := cfg.RunAs().UID().Get()
	require.True(t, ok)
	assert.Equal(t, int32(65534), uid)

	gid, ok := cfg.RunAs().GID().Get()
	require.True(t, ok)
	assert.Equal(t, int32(65534), gid)

	limit, ok := cfg.Resources().MemoryLimit().Get()
	require.True(t, ok)
	assert.Equal(t, uint64(512*1024*1024), limit)

	cpu, ok := cfg.Resources().CPULimit().Get()
	require.True(t, ok)
	assert.Equal(t, uint64(1500), cpu)

	// Zero is a value here, not "unset": it is what keeps the container out of swap.
	swap, ok := cfg.Resources().SwapLimit().Get()
	require.True(t, ok)
	assert.Zero(t, swap)

	mounts := cfg.Mounts()
	require.Len(t, mounts, 3)

	// A user volume with no options is writable by default.
	uv, ok := mounts[0].Volume().Get()
	require.True(t, ok)
	assert.Equal(t, "director-data", uv.Name())
	assert.NotContains(t, uv.MountOptions(), "rw")
	assert.NotContains(t, uv.MountOptions(), "ro")

	// A tmpfs with no options is writable by default.
	tmpfs, ok := mounts[1].Tmpfs().Get()
	require.True(t, ok)
	assert.NotContains(t, tmpfs.MountOptions(), "ro")

	// A host path with no options is writable by default.
	hp, ok := mounts[2].HostPath().Get()
	require.True(t, ok)
	assert.NotContains(t, hp.MountOptions(), "ro")

	healthCheck, ok := cfg.HealthCheck().Get()
	require.True(t, ok)
	assert.Equal(t, config.ContainerHealthCheckKindHTTP, healthCheck.Kind())
	assert.Equal(t, uint16(8080), healthCheck.Port())
	assert.Equal(t, "/healthz", healthCheck.HTTPPath())
	assert.Equal(t, 30*time.Second, healthCheck.Interval())
}

func TestContainerConfigValidationErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		doc         string
		expectedErr string
	}{
		{
			name:        "no name",
			doc:         "image: nginx",
			expectedErr: "name is required",
		},
		{
			name:        "uppercase name",
			doc:         "name: NGINX\nimage: nginx",
			expectedErr: "name can only contain lowercase ASCII letters, digits and hyphens",
		},
		{
			name:        "no image",
			doc:         "name: nginx",
			expectedErr: "image is required",
		},
		{
			name:        "unparseable image",
			doc:         "name: nginx\nimage: \"not a ref\"",
			expectedErr: "invalid image reference",
		},
		{
			name:        "negative uid",
			doc:         "name: nginx\nimage: nginx\nrunAs:\n  uid: -1",
			expectedErr: "runAs.uid must be non-negative, got -1",
		},
		{
			name:        "negative gid",
			doc:         "name: nginx\nimage: nginx\nrunAs:\n  gid: -1",
			expectedErr: "runAs.gid must be non-negative, got -1",
		},
		{
			name:        "environment without =",
			doc:         "name: nginx\nimage: nginx\nenvironment: [\"BROKEN\"]",
			expectedErr: "must be in KEY=value form",
		},
		{
			name:        "mount with no source",
			doc:         "name: nginx\nimage: nginx\nmounts:\n  - {}",
			expectedErr: "exactly one of volume, tmpfs or hostPath must be set",
		},
		{
			name: "mount with two sources",
			doc: `name: nginx
image: nginx
mounts:
  - tmpfs:
      destination: /tmp
    hostPath:
      source: /dev
      destination: /dev`,
			expectedErr: "exactly one of volume, tmpfs or hostPath must be set",
		},
		{
			name: "relative destination",
			doc: `name: nginx
image: nginx
mounts:
  - tmpfs:
      destination: tmp`,
			expectedErr: "must be an absolute path",
		},
		{
			name: "duplicate destinations",
			doc: `name: nginx
image: nginx
mounts:
  - tmpfs:
      destination: /tmp
  - tmpfs:
      destination: /tmp`,
			expectedErr: `duplicate destination "/tmp"`,
		},
		{
			name: "ro and rw together",
			doc: `name: nginx
image: nginx
mounts:
  - tmpfs:
      destination: /tmp
      options: [ro, rw]`,
			expectedErr: "mutually exclusive",
		},
		{
			name: "unknown mount option",
			doc: `name: nginx
image: nginx
mounts:
  - tmpfs:
      destination: /tmp
      options: [sync]`,
			expectedErr: `unsupported mount option "sync"`,
		},
		{
			name:        "unknown security profile",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  profile: yolo",
			expectedErr: "unsupported security profile",
		},
		{
			name:        "unknown seccomp profile",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  seccompProfile: runtime/default",
			expectedErr: "unsupported seccomp profile",
		},
		{
			name:        "unknown seccomp profile type",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  seccompProfile:\n    type: RuntimeDefault",
			expectedErr: `unsupported seccomp profile "RuntimeDefault", expected default, unconfined or localhost`,
		},
		{
			name:        "localhost seccomp profile without name",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  seccompProfile:\n    type: localhost",
			expectedErr: "seccompProfile.localhostProfile is required with seccompProfile.type: localhost",
		},
		{
			name:        "seccomp localhostProfile without localhost type",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  seccompProfile:\n    localhostProfile: audit.json",
			expectedErr: `seccompProfile.localhostProfile is only allowed with seccompProfile.type: localhost, got "default"`,
		},
		{
			name:        "seccomp localhostProfile with unconfined type",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  seccompProfile:\n    type: unconfined\n    localhostProfile: audit.json",
			expectedErr: `seccompProfile.localhostProfile is only allowed with seccompProfile.type: localhost, got "unconfined"`,
		},
		{
			name:        "capability with CAP_ prefix",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  capabilities:\n    add: [CAP_NET_ADMIN]",
			expectedErr: "without the CAP_ prefix",
		},
		{
			name:        "ALL in add",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  capabilities:\n    add: [ALL]",
			expectedErr: "does not accept ALL",
		},
		{
			name:        "capability in both lists",
			doc:         "name: nginx\nimage: nginx\nsecurity:\n  capabilities:\n    add: [NET_ADMIN]\n    drop: [NET_ADMIN]",
			expectedErr: "appears in both add and drop",
		},
		{
			name:        "unknown network mode",
			doc:         "name: nginx\nimage: nginx\nnetwork:\n  mode: bridge",
			expectedErr: "unsupported network mode",
		},
		{
			name:        "cpu without millicores",
			doc:         "name: nginx\nimage: nginx\nresources:\n  limits:\n    cpu: \"2\"",
			expectedErr: "must be expressed in millicores",
		},
		{
			name:        "zero cpu",
			doc:         "name: nginx\nimage: nginx\nresources:\n  limits:\n    cpu: 0m",
			expectedErr: "must be greater than zero",
		},
		{
			name:        "bad memory size",
			doc:         "name: nginx\nimage: nginx\nresources:\n  limits:\n    memory: lots",
			expectedErr: "is not a valid size",
		},
		{
			name:        "bad swap size",
			doc:         "name: nginx\nimage: nginx\nresources:\n  limits:\n    swap: none",
			expectedErr: "limits.swap \"none\" is not a valid size",
		},
		{
			name:        "unknown network condition",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  dependsOn:\n    network: [dns]",
			expectedErr: `unsupported dependsOn.network condition "dns"`,
		},
		{
			name:        "unknown network condition via deprecated networks",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  dependsOn:\n    networks: [dns]",
			expectedErr: `unsupported dependsOn.network condition "dns"`,
		},
		{
			name:        "relative dependsOn path",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  dependsOn:\n    paths: [var/mnt/data]",
			expectedErr: "must be an absolute path",
		},
		{
			// The deprecated top-level dependsOn is still validated.
			name:        "self dependency",
			doc:         "name: nginx\nimage: nginx\ndependsOn:\n  containers: [nginx]",
			expectedErr: "cannot depend on itself",
		},
		{
			name:        "negative stop timeout",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  stopTimeout: -1s",
			expectedErr: "lifecycle.stopTimeout: -1s must be greater than 0 and at most 4m0s",
		},
		{
			name:        "zero stop timeout",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  stopTimeout: 0s",
			expectedErr: "lifecycle.stopTimeout: 0s must be greater than 0 and at most 4m0s",
		},
		{
			name:        "stop timeout too long",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  stopTimeout: 5m",
			expectedErr: "lifecycle.stopTimeout: 5m0s must be greater than 0 and at most 4m0s",
		},
		{
			name:        "unknown restart policy",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    policy: sometimes",
			expectedErr: `lifecycle.restart.policy "sometimes" is not supported`,
		},
		{
			name:        "maxRetries without on-failure",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    maxRetries: 3",
			expectedErr: "lifecycle.restart.maxRetries is only supported with the on-failure policy",
		},
		{
			name:        "negative maxRetries",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    policy: on-failure\n    maxRetries: -1",
			expectedErr: "lifecycle.restart.maxRetries must be between 0 and 4294967295, got -1",
		},
		{
			name:        "maxRetries beyond uint32",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    policy: on-failure\n    maxRetries: 4294967296",
			expectedErr: "lifecycle.restart.maxRetries must be between 0 and 4294967295, got 4294967296",
		},
		{
			name:        "negative restart delay",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    delay: -1s",
			expectedErr: "lifecycle.restart.delay must be positive, got -1s",
		},
		{
			name:        "zero restart delay",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    delay: 0s",
			expectedErr: "lifecycle.restart.delay must be positive, got 0s",
		},
		{
			name:        "zero restart maxDelay",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    maxDelay: 0s",
			expectedErr: "lifecycle.restart.maxDelay must be positive, got 0s",
		},
		{
			name:        "maxDelay shorter than delay",
			doc:         "name: nginx\nimage: nginx\nlifecycle:\n  restart:\n    delay: 10s\n    maxDelay: 5s",
			expectedErr: "lifecycle.restart.maxDelay 5s must not be shorter than lifecycle.restart.delay 10s",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, "apiVersion: v1alpha1\nkind: ContainerConfig\n"+test.doc+"\n")

			_, err := cfg.Validate(validationMode{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedErr)
		})
	}
}

func TestValidateAbsPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		kind        string
		path        string
		expectedErr string
	}{
		{
			name:        "valid absolute path",
			kind:        "test.path",
			path:        "/absolute/path",
			expectedErr: "",
		},
		{
			name:        "valid absolute path with special characters",
			kind:        "test.path",
			path:        "/var/lib/some-file_123",
			expectedErr: "",
		},
		{
			name:        "valid root path",
			kind:        "test.path",
			path:        "/",
			expectedErr: "",
		},
		{
			name:        "relative path with subdirs",
			kind:        "mount.destination",
			path:        "var/lib/app",
			expectedErr: "must be an absolute path",
		},
		{
			name:        "relative path single component",
			kind:        "config.source",
			path:        "tmp",
			expectedErr: "must be an absolute path",
		},
		{
			name:        "relative path with parent dir",
			kind:        "hostPath.source",
			path:        "../etc/config",
			expectedErr: "must be an absolute path",
		},
		{
			name:        "current dir reference",
			kind:        "test.path",
			path:        "./app",
			expectedErr: "must be an absolute path",
		},
		{
			name:        "empty path",
			kind:        "container.mount",
			path:        "",
			expectedErr: "is required",
		},
		{
			name:        "empty kind and path",
			kind:        "",
			path:        "",
			expectedErr: "is required",
		},
		{
			name:        "kind with spaces",
			kind:        "my special kind",
			path:        "rel/path",
			expectedErr: "must be an absolute path",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := container.ValidateAbsPath(test.kind, test.path)

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
				// Verify the kind is in the error message for non-empty kinds
				if test.kind != "" && test.path != "" {
					assert.Contains(t, err.Error(), test.kind)
				}
			}
		})
	}
}

func TestContainerConfigValidateName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		metaName    string
		expectedErr string
	}{
		{
			name:     "valid simple name",
			metaName: "nginx",
		},
		{
			name:     "valid name with digits and hyphens",
			metaName: "web-server-1",
		},
		{
			name:     "name at max length",
			metaName: strings.Repeat("a", 63),
		},
		{
			name:        "name over max length",
			metaName:    strings.Repeat("a", 64),
			expectedErr: "must be 63 characters or fewer",
		},
		{
			name:        "empty name",
			metaName:    "",
			expectedErr: "name is required",
		},
		{
			name:        "uppercase name",
			metaName:    "NGINX",
			expectedErr: "name can only contain lowercase ASCII letters, digits and hyphens",
		},
		{
			name:        "name with underscore",
			metaName:    "web_server",
			expectedErr: "name can only contain lowercase ASCII letters, digits and hyphens",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := &container.ContainerConfigV1Alpha1{MetaName: test.metaName}

			err := cfg.ValidateName()

			if test.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)
			}
		})
	}
}

func TestContainerConfigValidateMounts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		mounts       []container.ContainerMount
		expectedErrs []string
	}{
		{
			name: "no mounts",
		},
		{
			name: "single valid mount",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"}},
			},
		},
		{
			name: "distinct destinations",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"}},
				{HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"}},
			},
		},
		{
			name: "duplicate destinations",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"}},
				{HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/tmp"}},
			},
			expectedErrs: []string{`mounts[1]: duplicate destination "/tmp"`},
		},
		{
			name: "mount with no source",
			mounts: []container.ContainerMount{
				{},
			},
			expectedErrs: []string{"mounts[0]: exactly one of volume, tmpfs or hostPath must be set"},
		},
		{
			name: "mount with two sources",
			mounts: []container.ContainerMount{
				{
					TmpfsMount:    &container.TmpfsMount{MountDestination: "/tmp"},
					HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"},
				},
			},
			expectedErrs: []string{"mounts[0]: exactly one of volume, tmpfs or hostPath must be set"},
		},
		{
			name: "invalid sub-mount is wrapped with its index",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"}},
				{VolumeMount: &container.VolumeMount{MountDestination: "/data"}},
			},
			expectedErrs: []string{"mounts[1]: volume.name is required"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := &container.ContainerConfigV1Alpha1{MountsConfig: test.mounts}

			err := cfg.ValidateMounts()

			if len(test.expectedErrs) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, expected := range test.expectedErrs {
				assert.Contains(t, err.Error(), expected)
			}
		})
	}
}

// validationMode is a minimal validation.RuntimeMode for the document-level tests.
type validationMode struct{}

func (validationMode) String() string        { return "test" }
func (validationMode) RequiresInstall() bool { return false }
func (validationMode) InContainer() bool     { return false }

func TestContainerConfigMergeReplacesEntrypointAndArgs(t *testing.T) {
	t.Parallel()

	base := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: list-test
image: docker.io/library/python:3.13-alpine
entrypoint: ["python3", "-c"]
args: ["print('one')"]
`)

	patch := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: list-test
image: docker.io/library/python:3.13-alpine
entrypoint: ["python3", "-u", "-c"]
args: ["print('two')"]
`)

	require.NoError(t, merge.Merge(base, patch))
	assert.Equal(t, []string{"python3", "-u", "-c"}, base.Entrypoint())
	assert.Equal(t, []string{"print('two')"}, base.Args())

	// Re-applying the same document must be idempotent.
	require.NoError(t, merge.Merge(base, patch))
	assert.Equal(t, []string{"python3", "-u", "-c"}, base.Entrypoint())
	assert.Equal(t, []string{"print('two')"}, base.Args())

	// Omitting the lists in the patch keeps the existing values.
	omitted := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: list-test
image: docker.io/library/python:3.13-alpine
`)

	require.NoError(t, merge.Merge(base, omitted))
	assert.Equal(t, []string{"python3", "-u", "-c"}, base.Entrypoint())
	assert.Equal(t, []string{"print('two')"}, base.Args())
}

func TestContainerConfigMergeReplacesHealthCheck(t *testing.T) {
	t.Parallel()

	const header = "apiVersion: v1alpha1\nkind: ContainerConfig\nname: web\nimage: docker.io/library/nginx:1.27\n"

	t.Run("patch without lifecycle keeps the probe", func(t *testing.T) {
		t.Parallel()

		base := load(t, header+"lifecycle:\n  healthCheck:\n    http:\n      port: 8080\n")

		require.NoError(t, merge.Merge(base, load(t, header)))

		healthCheck, ok := base.HealthCheck().Get()
		require.True(t, ok)
		assert.Equal(t, config.ContainerHealthCheckKindHTTP, healthCheck.Kind())
		assert.Equal(t, uint16(8080), healthCheck.Port())
	})

	t.Run("patch with another probe kind replaces the probe", func(t *testing.T) {
		t.Parallel()

		base := load(t, header+"lifecycle:\n  healthCheck:\n    http:\n      port: 8080\n    interval: 30s\n")

		require.NoError(t, merge.Merge(base, load(t, header+"lifecycle:\n  healthCheck:\n    tcp:\n      port: 5432\n")))

		_, err := base.Validate(validationMode{})
		require.NoError(t, err)

		healthCheck, ok := base.HealthCheck().Get()
		require.True(t, ok)
		assert.Equal(t, config.ContainerHealthCheckKindTCP, healthCheck.Kind())
		assert.Equal(t, uint16(5432), healthCheck.Port())
		assert.Equal(t, 10*time.Second, healthCheck.Interval())
	})

	t.Run("patch with only timing drops the probe", func(t *testing.T) {
		t.Parallel()

		base := load(t, header+"lifecycle:\n  healthCheck:\n    http:\n      port: 8080\n")

		require.NoError(t, merge.Merge(base, load(t, header+"lifecycle:\n  healthCheck:\n    interval: 30s\n")))

		_, err := base.Validate(validationMode{})
		require.ErrorContains(t, err, "lifecycle.healthCheck: exactly one of tcp, http or grpc must be set")
	})
}

func TestContainerConfigSeccompProfile(t *testing.T) {
	t.Parallel()

	const header = "apiVersion: v1alpha1\nkind: ContainerConfig\nname: web\nimage: docker.io/library/nginx:1.27\n"

	for _, test := range []struct {
		name string
		doc  string

		expectedProfile          config.ContainerSeccompProfile
		expectedLocalhostProfile string
	}{
		{
			name:            "unset",
			expectedProfile: config.ContainerSeccompProfileDefault,
		},
		{
			name:            "empty object",
			doc:             "security:\n  seccompProfile: {}\n",
			expectedProfile: config.ContainerSeccompProfileDefault,
		},
		{
			name:            "default",
			doc:             "security:\n  seccompProfile:\n    type: default\n",
			expectedProfile: config.ContainerSeccompProfileDefault,
		},
		{
			name:            "unconfined",
			doc:             "security:\n  seccompProfile:\n    type: unconfined\n",
			expectedProfile: config.ContainerSeccompProfileUnconfined,
		},
		{
			name:            "unconfined shorthand",
			doc:             "security:\n  seccompProfile: unconfined\n",
			expectedProfile: config.ContainerSeccompProfileUnconfined,
		},
		{
			name:                     "localhost",
			doc:                      "security:\n  seccompProfile:\n    type: localhost\n    localhostProfile: audit.json\n",
			expectedProfile:          config.ContainerSeccompProfileLocalhost,
			expectedLocalhostProfile: "audit.json",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := load(t, header+test.doc)

			_, err := cfg.Validate(validationMode{})
			require.NoError(t, err)

			assert.Equal(t, test.expectedProfile, cfg.Security().SeccompProfile())
			assert.Equal(t, test.expectedLocalhostProfile, cfg.Security().SeccompLocalhostProfile())
		})
	}
}

func TestContainerConfigSeccompProfileUnknownField(t *testing.T) {
	t.Parallel()

	_, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: ContainerConfig
name: web
image: docker.io/library/nginx:1.27
security:
  seccompProfile:
    type: localhost
    localhostprofile: audit.json
`))
	require.ErrorContains(t, err, "unknown keys found during decoding")
	require.ErrorContains(t, err, "localhostprofile: audit.json")
}

func TestContainerConfigSeccompProfileMarshal(t *testing.T) {
	t.Parallel()

	cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: web
image: docker.io/library/nginx:1.27
security:
  seccompProfile: unconfined
`)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	// The shorthand is normalized to the object form.
	assert.Contains(t, string(marshaled), "seccompProfile:\n        type: unconfined\n")
}

func TestContainerConfigMergeReplacesSeccompProfile(t *testing.T) {
	t.Parallel()

	const header = "apiVersion: v1alpha1\nkind: ContainerConfig\nname: web\nimage: docker.io/library/nginx:1.27\n"

	t.Run("patch without seccompProfile keeps it", func(t *testing.T) {
		t.Parallel()

		base := load(t, header+"security:\n  seccompProfile:\n    type: localhost\n    localhostProfile: audit.json\n")

		require.NoError(t, merge.Merge(base, load(t, header+"security:\n  profile: privileged\n")))

		assert.Equal(t, config.ContainerSeccompProfileLocalhost, base.Security().SeccompProfile())
		assert.Equal(t, "audit.json", base.Security().SeccompLocalhostProfile())
	})

	t.Run("patch with another type drops the profile name", func(t *testing.T) {
		t.Parallel()

		base := load(t, header+"security:\n  seccompProfile:\n    type: localhost\n    localhostProfile: audit.json\n")

		require.NoError(t, merge.Merge(base, load(t, header+"security:\n  seccompProfile:\n    type: default\n")))

		_, err := base.Validate(validationMode{})
		require.NoError(t, err)

		assert.Equal(t, config.ContainerSeccompProfileDefault, base.Security().SeccompProfile())
		assert.Empty(t, base.Security().SeccompLocalhostProfile())
	})
}

func TestContainerConfigRestart(t *testing.T) {
	t.Parallel()

	cfg := load(t, `apiVersion: v1alpha1
kind: ContainerConfig
name: migrate
image: ghcr.io/example/migrate:v1.0.0
lifecycle:
  restart:
    policy: on-failure
    maxRetries: 3
    delay: 1s
    maxDelay: 1m
`)

	_, err := cfg.Validate(validationMode{})
	require.NoError(t, err)

	assert.Equal(t, config.ContainerRestartPolicyOnFailure, cfg.Restart().Policy())
	assert.Equal(t, optional.Some(3), cfg.Restart().MaxRetries())
	assert.Equal(t, time.Second, cfg.Restart().Delay())
	assert.Equal(t, time.Minute, cfg.Restart().MaxDelay())
}
