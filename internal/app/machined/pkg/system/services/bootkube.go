// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"go.etcd.io/etcd/clientv3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/retry"
)

// Bootkube implements the Service interface. It serves as the concrete type with
// the required methods.
type Bootkube struct {
	provisioned bool
}

// ID implements the Service interface.
func (b *Bootkube) ID(r runtime.Runtime) string {
	return "bootkube"
}

// PreFunc implements the Service interface.
func (b *Bootkube) PreFunc(ctx context.Context, r runtime.Runtime) (err error) {
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

	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))

	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/bootkube.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/bootkube"),
		},
	})
}

// PostFunc implements the Service interface.
func (b *Bootkube) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
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
			"--config=" + constants.ConfigPath,
		},
	}

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	// Set the required kubelet mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.ConfigPath, Source: constants.ConfigPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rshared", "rw"}},
	}

	return containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
		),
	), nil
}
