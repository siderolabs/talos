// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"

	"github.com/talos-systems/go-retry/retry"

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

		err = retry.Exponential(3*time.Minute, retry.WithUnits(50*time.Millisecond), retry.WithJitter(25*time.Millisecond)).Retry(func() error {
			var resp *clientv3.GetResponse

			// limit single attempt to 15 seconds to allow for 12 attempts at least
			attemptCtx, attemptCtxCancel := context.WithTimeout(ctx, 15*time.Second)
			defer attemptCtxCancel()

			if resp, err = client.Get(clientv3.WithRequireLeader(attemptCtx), constants.InitializedKey); err != nil {
				if errors.Is(err, rpctypes.ErrGRPCKeyNotFound) {
					// no key set yet, treat as not provisioned yet
					return nil
				}

				return retry.ExpectedError(err)
			}

			if len(resp.Kvs) == 0 {
				// no key/values in the range, treat as not provisioned yet
				return nil
			}

			if string(resp.Kvs[0].Value) == "true" {
				b.provisioned = true
			}

			return nil
		})

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

	err = retry.Exponential(15*time.Second, retry.WithUnits(50*time.Millisecond), retry.WithJitter(25*time.Millisecond)).Retry(func() error {
		ctx := clientv3.WithRequireLeader(context.Background())
		if _, err = client.Put(ctx, constants.InitializedKey, "true"); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to put state into etcd: %w", err)
	}

	log.Println("updated initialization status in etcd")

	return nil
}

// DependsOn implements the Service interface.
func (b *Bootkube) DependsOn(r runtime.Runtime) []string {
	return []string{"etcd"}
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
