// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// PreflightChecks runs the preflight checks.
type PreflightChecks struct {
	disabled bool
	client   *client.Client

	installerTalosVersion *compatibility.TalosVersion
	hostTalosVersion      *compatibility.TalosVersion
}

// NewPreflightChecks initializes and returns the installation PreflightChecks.
func NewPreflightChecks(ctx context.Context) (*PreflightChecks, error) {
	if _, err := os.Stat(constants.MachineSocketPath); err != nil {
		log.Printf("pre-flight checks disabled, as host Talos version is too old")

		return &PreflightChecks{disabled: true}, nil //nolint:nilerr
	}

	c, err := client.New(ctx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the machine service: %w", err)
	}

	return &PreflightChecks{
		client: c,
	}, nil
}

// Close closes the client.
func (checks *PreflightChecks) Close() error {
	if checks.disabled {
		return nil
	}

	return checks.client.Close()
}

// Run the checks, return the error if the check fails.
func (checks *PreflightChecks) Run(ctx context.Context) error {
	if checks.disabled {
		return nil
	}

	log.Printf("running pre-flight checks")

	// inject "fake" authorization
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(constants.APIAuthzRoleMetadataKey, string(role.Admin)))

	for _, check := range []func(context.Context) error{
		checks.talosVersion,
		checks.kubernetesVersion,
	} {
		if err := check(ctx); err != nil {
			return fmt.Errorf("pre-flight checks failed: %w", err)
		}
	}

	log.Printf("all pre-flight checks successful")

	return nil
}

func (checks *PreflightChecks) talosVersion(ctx context.Context) error {
	resp, err := checks.client.Version(ctx)
	if err != nil {
		return fmt.Errorf("error getting Talos version: %w", err)
	}

	hostVersion := unpack(resp.Messages)

	log.Printf("host Talos version: %s", hostVersion.Version.Tag)

	checks.hostTalosVersion, err = compatibility.ParseTalosVersion(hostVersion.Version)
	if err != nil {
		return fmt.Errorf("error parsing host Talos version: %w", err)
	}

	checks.installerTalosVersion, err = compatibility.ParseTalosVersion(version.NewVersion())
	if err != nil {
		return fmt.Errorf("error parsing installer Talos version: %w", err)
	}

	return checks.installerTalosVersion.UpgradeableFrom(checks.hostTalosVersion)
}

type k8sVersions struct {
	kubelet           *compatibility.KubernetesVersion
	apiServer         *compatibility.KubernetesVersion
	scheduler         *compatibility.KubernetesVersion
	controllerManager *compatibility.KubernetesVersion
}

//nolint:gocyclo
func (versions *k8sVersions) gatherVersions(ctx context.Context, client *client.Client) error {
	kubeletSpec, err := safe.StateGet[*k8s.KubeletSpec](ctx, client.COSI, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting kubelet spec: %w", err)
	}

	if kubeletSpec != nil {
		versions.kubelet, err = KubernetesVersionFromImageRef(kubeletSpec.TypedSpec().Image)
		if err != nil {
			return fmt.Errorf("error parsing kubelet version: %w", err)
		}
	}

	apiServerSpec, err := safe.StateGet[*k8s.APIServerConfig](ctx, client.COSI, k8s.NewAPIServerConfig().Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting API server spec: %w", err)
	}

	if apiServerSpec != nil {
		versions.apiServer, err = KubernetesVersionFromImageRef(apiServerSpec.TypedSpec().Image)
		if err != nil {
			return fmt.Errorf("error parsing API server version: %w", err)
		}
	}

	schedulerSpec, err := safe.StateGet[*k8s.SchedulerConfig](ctx, client.COSI, k8s.NewSchedulerConfig().Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting scheduler spec: %w", err)
	}

	if schedulerSpec != nil {
		versions.scheduler, err = KubernetesVersionFromImageRef(schedulerSpec.TypedSpec().Image)
		if err != nil {
			return fmt.Errorf("error parsing scheduler version: %w", err)
		}
	}

	controllerManagerSpec, err := safe.StateGet[*k8s.ControllerManagerConfig](ctx, client.COSI, k8s.NewControllerManagerConfig().Metadata())
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting controller manager spec: %w", err)
	}

	if controllerManagerSpec != nil {
		versions.controllerManager, err = KubernetesVersionFromImageRef(controllerManagerSpec.TypedSpec().Image)
		if err != nil {
			return fmt.Errorf("error parsing controller manager version: %w", err)
		}
	}

	return nil
}

func (versions *k8sVersions) checkCompatibility(target *compatibility.TalosVersion) error {
	for _, component := range []struct {
		name    string
		version *compatibility.KubernetesVersion
	}{
		{
			name:    "kubelet",
			version: versions.kubelet,
		},
		{
			name:    "kube-apiserver",
			version: versions.apiServer,
		},
		{
			name:    "kube-scheduler",
			version: versions.scheduler,
		},
		{
			name:    "kube-controller-manager",
			version: versions.controllerManager,
		},
	} {
		if component.version == nil {
			continue
		}

		if err := component.version.SupportedWith(target); err != nil {
			return fmt.Errorf("component %s version issue: %w", component.name, err)
		}
	}

	return nil
}

func (versions *k8sVersions) String() string {
	var components []string //nolint:prealloc

	for _, component := range []struct {
		name    string
		version *compatibility.KubernetesVersion
	}{
		{
			name:    "kubelet",
			version: versions.kubelet,
		},
		{
			name:    "kube-apiserver",
			version: versions.apiServer,
		},
		{
			name:    "kube-scheduler",
			version: versions.scheduler,
		},
		{
			name:    "kube-controller-manager",
			version: versions.controllerManager,
		},
	} {
		if component.version == nil {
			continue
		}

		components = append(components, fmt.Sprintf("%s: %s", component.name, component.version))
	}

	return strings.Join(components, ", ")
}

func (checks *PreflightChecks) kubernetesVersion(ctx context.Context) error {
	var versions k8sVersions

	if err := versions.gatherVersions(ctx, checks.client); err != nil {
		return err
	}

	log.Printf("host Kubernetes versions: %s", &versions)

	return versions.checkCompatibility(checks.installerTalosVersion)
}

// KubernetesVersionFromImageRef parses the Kubernetes version from the image reference.
func KubernetesVersionFromImageRef(ref string) (*compatibility.KubernetesVersion, error) {
	idx := strings.LastIndex(ref, ":v")
	if idx == -1 {
		return nil, fmt.Errorf("invalid image reference: %q", ref)
	}

	versionPart := ref[idx+2:]

	if shaIndex := strings.Index(versionPart, "@"); shaIndex != -1 {
		versionPart = versionPart[:shaIndex]
	}

	return compatibility.ParseKubernetesVersion(versionPart)
}

func unpack[T any](s []T) T {
	if len(s) != 1 {
		panic("unpack: slice length is not 1")
	}

	return s[0]
}
