// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/conditions"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Bootkube implements the Service interface. It serves as the concrete type with
// the required methods.
type Bootkube struct {
	Source  machineapi.RecoverRequest_Source
	Recover bool

	provisioned bool
}

// ID implements the Service interface.
func (b *Bootkube) ID(r runtime.Runtime) string {
	return "bootkube"
}

// PreFunc implements the Service interface.
func (b *Bootkube) PreFunc(ctx context.Context, r runtime.Runtime) (err error) {
	if !b.Recover {
		client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer client.Close()

		b.provisioned, err = client.GetInitialized(ctx)
		if err != nil {
			return fmt.Errorf("error querying cluster provisioned state in etcd: %w", err)
		}

		if b.provisioned {
			return nil
		}
	}

	return image.Import(ctx, "/usr/images/bootkube.tar", "talos/bootkube")
}

// PostFunc implements the Service interface.
//
// This is temorary and should be removed once we remove the init node type.
func (b *Bootkube) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	if r.Config().Machine().Type() != machine.TypeInit {
		return nil
	}

	if state != events.StateFinished {
		log.Println("bootkube run did not complete successfully. skipping etcd update")

		return nil
	}

	client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer client.Close()

	err = client.MarkAsInitialized(context.Background())
	if err != nil {
		return fmt.Errorf("failed to put state into etcd: %w", err)
	}

	log.Println("updated initialization status in etcd")

	return nil
}

// DependsOn implements the Service interface.
func (b *Bootkube) DependsOn(r runtime.Runtime) []string {
	return []string{"etcd", "kubelet"}
}

// Condition implements the Service interface.
func (b *Bootkube) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// Runner implements the Service interface.
func (b *Bootkube) Runner(r runtime.Runtime) (runner.Runner, error) {
	if b.provisioned {
		return nil, nil
	}

	image := "talos/bootkube"

	// Set the process arguments.
	args := runner.Args{
		ID: b.ID(r),
		ProcessArgs: []string{
			"/bootkube",
			"--strict=" + strconv.FormatBool(!b.Recover),
			"--recover=" + strconv.FormatBool(b.Recover),
			"--recover-source=" + b.Source.String(),
		},
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rshared", "rw"}},
	}

	bb, err := r.Config().Bytes()
	if err != nil {
		return nil, err
	}

	stdin := bytes.NewReader(bb)

	return containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithStdin(stdin),
		runner.WithLoggingManager(r.Logging()),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	), nil
}
