// nolint: dupl,golint
package services

import (
	"fmt"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	"github.com/autonomy/talos/src/initramfs/pkg/version"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// OSD implements the Service interface. It serves as the concrete type with
// the required methods.
type OSD struct{}

// ID implements the Service interface.
func (o *OSD) ID(data *userdata.UserData) string {
	return "osd"
}

// PreFunc implements the Service interface.
func (o *OSD) PreFunc(data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (o *OSD) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (o *OSD) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

func (o *OSD) Start(data *userdata.UserData) error {
	// Set the image.
	var image string
	if data.Services.OSD != nil && data.Services.OSD.Image != "" {
		image = data.Services.OSD.Image
	} else {
		image = "docker.io/autonomy/osd:" + version.SHA
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          o.ID(data),
		ProcessArgs: []string{"/osd", "--userdata=" + constants.UserDataPath},
	}
	if data.IsWorker() {
		args.ProcessArgs = append(args.ProcessArgs, "--generate=true")
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.ContainerdSocket, Source: constants.ContainerdSocket, Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/var/run", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: "/run", Source: "/run", Options: []string{"rbind", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/ssl", Source: "/etc/ssl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/var/log", Source: "/var/log", Options: []string{"rbind", "rw"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			oci.WithMounts(mounts),
		),
	)
}
