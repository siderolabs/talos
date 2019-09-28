/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl,golint
package services

import (
	"context"
	"fmt"
	"net"
	"strings"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/userdata"
)

// OSD implements the Service interface. It serves as the concrete type with
// the required methods.
type OSD struct{}

// ID implements the Service interface.
func (o *OSD) ID(data *userdata.UserData) string {
	return "osd"
}

// PreFunc implements the Service interface.
func (o *OSD) PreFunc(ctx context.Context, data *userdata.UserData) error {
	importer := containerd.NewImporter(constants.SystemContainerdNamespace, containerd.WithContainerdAddress(constants.SystemContainerdAddress))
	return importer.Import(&containerd.ImportRequest{
		Path: "/usr/images/osd.tar",
		Options: []containerdapi.ImportOpt{
			containerdapi.WithIndexName("talos/osd"),
		},
	})
}

// PostFunc implements the Service interface.
func (o *OSD) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (o *OSD) Condition(data *userdata.UserData) conditions.Condition {
	if data.Services.Kubeadm.IsWorker() {
		return conditions.WaitForFileToExist("/etc/kubernetes/kubelet.conf")
	}

	return nil
}

// DependsOn implements the Service interface.
func (o *OSD) DependsOn(data *userdata.UserData) []string {
	return []string{"system-containerd", "containerd"}
}

func (o *OSD) Runner(data *userdata.UserData) (runner.Runner, error) {
	image := "talos/osd"

	endpoints := data.Services.Trustd.Endpoints
	if data.Services.Kubeadm.IsWorker() {
		h, err := kubernetes.NewHelper()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create client")
		}

		endpoints, err = h.MasterIPs()
		if err != nil {
			return nil, err
		}
	}

	// Set the process arguments.
	args := runner.Args{
		ID: o.ID(data),
		ProcessArgs: []string{
			"/osd",
			"--userdata=" + constants.UserDataPath,
			"--endpoints=" + strings.Join(endpoints, ","),
		},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/tmp", Source: "/tmp", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.ContainerdAddress, Source: constants.ContainerdAddress, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: constants.SystemRunPath, Source: constants.SystemRunPath, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/log/pods", Source: "/var/log/pods", Options: []string{"bind", "ro"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	return restart.New(containerd.NewRunner(
		data,
		&args,
		runner.WithContainerdAddress(constants.SystemContainerdAddress),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithMounts(mounts),
		),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface
func (o *OSD) HealthFunc(*userdata.UserData) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", "127.0.0.1", constants.OsdPort))
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface
func (o *OSD) HealthSettings(*userdata.UserData) *health.Settings {
	return &health.DefaultSettings
}
