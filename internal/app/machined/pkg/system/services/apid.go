// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containerd/containerd/oci"
	"github.com/fsnotify/fsnotify"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/copy"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/time"
)

// APID implements the Service interface. It serves as the concrete type with
// the required methods.
type APID struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ID implements the Service interface.
func (o *APID) ID(r runtime.Runtime) string {
	return "apid"
}

// PreFunc implements the Service interface.
func (o *APID) PreFunc(ctx context.Context, r runtime.Runtime) error {
	if r.Config().Machine().Type() == machine.TypeJoin {
		o.syncKubeletPKI()
	}

	return prepareRootfs(o.ID(r))
}

// PostFunc implements the Service interface.
func (o *APID) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	if o.cancel != nil {
		o.cancel()
	}

	o.wg.Wait()

	return nil
}

// Condition implements the Service interface.
func (o *APID) Condition(r runtime.Runtime) conditions.Condition {
	conds := []conditions.Condition{
		time.NewSyncCondition(r.State().V1Alpha2().Resources()),
	}

	if r.Config().Machine().Type() == machine.TypeJoin {
		conds = append(conds, conditions.WaitForFileToExist(constants.KubeletKubeconfig))
	}

	return conditions.WaitForAll(conds...)
}

// DependsOn implements the Service interface.
func (o *APID) DependsOn(r runtime.Runtime) []string {
	return []string{"containerd", "networkd"}
}

// Runner implements the Service interface.
func (o *APID) Runner(r runtime.Runtime) (runner.Runner, error) {
	// Ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.APISocketPath), 0o750); err != nil {
		return nil, err
	}

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(r),
		ProcessArgs: []string{
			"/apid",
		},
	}

	isWorker := r.Config().Machine().Type() == machine.TypeJoin

	if !isWorker {
		args.ProcessArgs = append(args.ProcessArgs, "--endpoints="+strings.Join([]string{"127.0.0.1"}, ","))
	} else {
		args.ProcessArgs = append(args.ProcessArgs, "--use-kubernetes-endpoints")
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.MachineSocketPath), Source: filepath.Dir(constants.MachineSocketPath), Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: filepath.Dir(constants.APISocketPath), Source: filepath.Dir(constants.APISocketPath), Options: []string{"rbind", "rw"}},
	}

	if isWorker {
		// worker requires kubelet config to refresh the certs via Kubernetes
		mounts = append(mounts,
			specs.Mount{Type: "bind", Destination: filepath.Dir(constants.KubeletKubeconfig), Source: constants.SystemKubeletPKIDir, Options: []string{"rbind", "ro"}},
			specs.Mount{Type: "bind", Destination: constants.KubeletPKIDir, Source: constants.SystemKubeletPKIDir, Options: []string{"rbind", "ro"}},
		)
	}

	env := []string{}

	for key, val := range r.Config().Machine().Env() {
		switch strings.ToLower(key) {
		// explicitly exclude proxy variables from apid since this will
		// negatively impact grpc connections.
		// ref: https://github.com/grpc/grpc-go/blob/0f32486dd3c9bc29705535bd7e2e43801824cbc4/clientconn.go#L199-L206
		// ref: https://github.com/grpc/grpc-go/blob/63ae68c9686cc0dd26c4f7476d66bb2f5c31789f/proxy.go#L118-L144
		case "no_proxy":
		case "http_proxy":
		case "https_proxy":
		default:
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	b, err := r.Config().Bytes()
	if err != nil {
		return nil, err
	}

	stdin := bytes.NewReader(b)

	return restart.New(containerd.NewRunner(
		r.Config().Debug(),
		&args,
		runner.WithStdin(stdin),
		runner.WithLoggingManager(r.Logging()),
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
			oci.WithRootFSPath(filepath.Join(constants.SystemLibexecPath, o.ID(r))),
			oci.WithRootFSReadonly(),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (o *APID) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer

		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", constants.ApidPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (o *APID) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}

func (o *APID) syncKubeletPKI() {
	copyAll := func() {
		if err := copy.Dir(constants.KubeletPKIDir, constants.SystemKubeletPKIDir, copy.WithMode(0o700)); err != nil {
			log.Printf("failed to sync %s dir contents into %s: %s", constants.KubeletPKIDir, constants.SystemKubeletPKIDir, err)

			return
		}

		if err := copy.File(constants.KubeletKubeconfig, filepath.Join(constants.SystemKubeletPKIDir, filepath.Base(constants.KubeletKubeconfig)), copy.WithMode(0o700)); err != nil {
			log.Printf("failed to sync %s into %s: %s", constants.KubeletKubeconfig, constants.SystemKubeletPKIDir, err)

			return
		}
	}

	if err := os.MkdirAll(constants.KubeletPKIDir, 0o700); err != nil {
		log.Printf("failed creating kubelet PKI directory: %s", err)

		return
	}

	copyAll()

	o.ctx, o.cancel = context.WithCancel(context.Background())
	o.wg.Add(1)

	go func() {
		defer o.wg.Done()

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Printf("failed to create directory watcher %s", err)

			return
		}

		defer watcher.Close() //nolint:errcheck

		err = watcher.Add(constants.KubeletPKIDir)
		if err != nil {
			log.Printf("failed to watch dir %s %s", constants.KubeletPKIDir, err)

			return
		}

		for {
			select {
			case <-o.ctx.Done():
				return
			case <-watcher.Events:
				copyAll()
			case err = <-watcher.Errors:
				log.Printf("directory watch error %s", err)
			}
		}
	}()
}
