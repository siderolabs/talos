// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
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
	assert.Equal(t, "none", string(cfg.Network().Mode()))
	assert.False(t, cfg.Resources().MemoryLimit().IsPresent())
	assert.Empty(t, cfg.DependsOn().Paths())
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
  - userVolume:
      name: director-data
      destination: /var/lib/director
      options: [rw]
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
network:
  mode: host
resources:
  limits:
    cpu: 1500m
    memory: 512MiB
dependsOn:
  paths:
    - /var/mnt/director-data
  networks:
    - addresses
  time: true
`)

	_, err := cfg.Validate(validationMode{})
	require.NoError(t, err)

	assert.Equal(t, "director", cfg.Name())
	assert.Equal(t, []string{"/director"}, cfg.Entrypoint())
	assert.Equal(t, "/srv", cfg.WorkingDir())
	assert.Equal(t, "privileged", string(cfg.Security().Profile()))
	assert.Equal(t, "host", string(cfg.Network().Mode()))
	assert.True(t, cfg.DependsOn().Time())

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

	mounts := cfg.Mounts()
	require.Len(t, mounts, 3)

	// rw is honored, and stripped from the option list handed downstream.
	uv, ok := mounts[0].UserVolume().Get()
	require.True(t, ok)
	assert.Equal(t, "director-data", uv.Name())
	assert.NotContains(t, uv.MountOptions(), "rw")
	assert.NotContains(t, uv.MountOptions(), "ro")

	// A tmpfs with no options is writable by default.
	tmpfs, ok := mounts[1].Tmpfs().Get()
	require.True(t, ok)
	assert.NotContains(t, tmpfs.MountOptions(), "ro")

	// A mount with no options is read-only by default.
	hp, ok := mounts[2].HostPath().Get()
	require.True(t, ok)
	assert.Equal(t, []string{"ro"}, hp.MountOptions())
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
			expectedErr: "exactly one of userVolume, tmpfs or hostPath must be set",
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
			expectedErr: "exactly one of userVolume, tmpfs or hostPath must be set",
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
			name:        "unknown network condition",
			doc:         "name: nginx\nimage: nginx\ndependsOn:\n  networks: [dns]",
			expectedErr: `unsupported dependsOn.networks condition "dns"`,
		},
		{
			name:        "relative dependsOn path",
			doc:         "name: nginx\nimage: nginx\ndependsOn:\n  paths: [var/mnt/data]",
			expectedErr: "must be an absolute path",
		},
		{
			name:        "self dependency",
			doc:         "name: nginx\nimage: nginx\ndependsOn:\n  containers: [nginx]",
			expectedErr: "cannot depend on itself",
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
			expectedErrs: []string{"mounts[0]: exactly one of userVolume, tmpfs or hostPath must be set"},
		},
		{
			name: "mount with two sources",
			mounts: []container.ContainerMount{
				{
					TmpfsMount:    &container.TmpfsMount{MountDestination: "/tmp"},
					HostPathMount: &container.HostPathMount{MountSource: "/dev", MountDestination: "/dev"},
				},
			},
			expectedErrs: []string{"mounts[0]: exactly one of userVolume, tmpfs or hostPath must be set"},
		},
		{
			name: "invalid sub-mount is wrapped with its index",
			mounts: []container.ContainerMount{
				{TmpfsMount: &container.TmpfsMount{MountDestination: "/tmp"}},
				{UserVolumeMount: &container.UserVolumeMount{MountDestination: "/data"}},
			},
			expectedErrs: []string{"mounts[1]: userVolume.name is required"},
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
