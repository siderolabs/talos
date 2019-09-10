/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services/kubeadm"
	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// ID implements the Service interface.
func (k *Kubeadm) ID(data *userdata.UserData) string {
	return "kubeadm"
}

// PreFunc implements the Service interface.
// nolint: gocyclo
func (k *Kubeadm) PreFunc(ctx context.Context, data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.IsBootstrap() {
		if err = kubeadm.WritePKIFiles(data); err != nil {
			return err
		}
	}

	if err = kubeadm.WriteConfig(data); err != nil {
		return err
	}

	containerdctx := namespaces.WithNamespace(ctx, "k8s.io")
	client, err := containerdapi.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	// Pull the image and unpack it.

	if _, err = client.Pull(containerdctx, constants.KubernetesImage, containerdapi.WithPullUnpack); err != nil {
		return fmt.Errorf("failed to pull image %q: %v", constants.KubernetesImage, err)
	}

	// Run kubeadm init phase certs all. This should fill in whatever gaps
	// we have in the provided certs.
	if data.Services.Kubeadm.IsBootstrap() {
		if err = kubeadm.PhaseCerts(); err != nil {
			return err
		}
	}

	if data.Services.Kubeadm.IsWorker() || data.Services.Kubeadm.IsBootstrap() || data.Services.Trustd == nil {
		log.Println("skipping retrieval of files from peers via trustd")
		return nil
	}

	// Initialize trustd peer client connection
	var trustds []proto.TrustdClient
	if trustds, err = kubeadm.CreateTrustdClients(data); err != nil {
		return err
	}

	// Wait for all files to get synced
	var wg sync.WaitGroup
	wg.Add(len(kubeadm.FileSet(kubeadm.RequiredFiles())))

	log.Println("retrieving needed files via trustd")
	// Generate a list of files we need to request
	// ( filtered by ones we already have ) and
	// Get assets from remote nodes
	for _, fileRequest := range kubeadm.FileSet(kubeadm.RequiredFiles()) {
		// Handle all file requests in parallel
		go func(ctx context.Context, fileRequest *proto.ReadFileRequest) {
			defer wg.Done()

			trustctx, ctxCancel := context.WithCancel(ctx)
			defer ctxCancel()

			// Have a single chan shared across all clients
			// for a given file
			content := make(chan []byte)

			// kick off a goroutine for each trustd client
			// to fetch the given file
			for _, trustdClient := range trustds {
				go kubeadm.Download(trustctx, trustdClient, fileRequest, content)
			}

			select {
			case <-trustctx.Done():
				return
			case filecontent := <-content:
				// TODO replace this with proper error handling
				// nolint: errcheck
				// read from the content chan to write out the
				// given file
				kubeadm.WriteTrustdFiles(fileRequest.Path, filecontent)
			}
		}(ctx, fileRequest)
	}
	wg.Wait()

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubeadm) PostFunc(data *userdata.UserData) error {
	return nil
}

// DependsOn implements the Service interface.
func (k *Kubeadm) DependsOn(data *userdata.UserData) []string {
	deps := []string{"containerd", "networkd"}

	if data.Services.Kubeadm.IsControlPlane() {
		deps = append(deps, "trustd")
	}

	return deps
}

// Condition implements the Service interface.
func (k *Kubeadm) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// Runner implements the Service interface.
func (k *Kubeadm) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := constants.KubernetesImage

	// We only wan't to run kubeadm if it hasn't been ran already.
	if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); !os.IsNotExist(err) {
		return nil, nil
	}

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(data),
	}

	ignorePreflightErrors := []string{"cri", "kubeletversion", "numcpu", "ipvsproxiercheck"}
	ignorePreflightErrors = append(ignorePreflightErrors, data.Services.Kubeadm.IgnorePreflightErrors...)
	ignore := "--ignore-preflight-errors=" + strings.Join(ignorePreflightErrors, ",")

	switch {
	case data.Services.Kubeadm.InitConfiguration != nil:
		args.ProcessArgs = []string{
			"kubeadm",
			"init",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			ignore,
			"--skip-token-print",
			"--skip-certificate-key-print",
		}
	case data.Services.Kubeadm.JoinConfiguration != nil:
		// Worker
		args.ProcessArgs = []string{
			"kubeadm",
			"join",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			ignore,
		}
	default:
		return nil, errors.New("invalid kubeadm configuration type")
	}

	args.ProcessArgs = append(args.ProcessArgs, data.Services.Kubeadm.ExtraArgs...)

	// Set the mounts.
	// nolint: dupl
	mounts := []specs.Mount{
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/lib/modules", Source: "/lib/modules", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/crictl", Source: "/bin/crictl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm", Source: "/bin/kubeadm", Options: []string{"bind", "ro"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return containerd.NewRunner(
		data,
		&args,
		runner.WithNamespace(criconstants.K8sContainerdNamespace),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			containerd.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	), nil
}
