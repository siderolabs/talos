// nolint: dupl,golint
package services

import (
	"fmt"
	"os"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	"github.com/autonomy/talos/src/initramfs/pkg/version"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Blockd implements the Service interface. It serves as the concrete type with
// the required methods.
type Blockd struct{}

// ID implements the Service interface.
func (t *Blockd) ID(data *userdata.UserData) string {
	return "blockd"
}

// PreFunc implements the Service interface.
func (t *Blockd) PreFunc(data *userdata.UserData) error {
	return os.Mkdir("/run/blockd", os.ModeDir)
}

// PostFunc implements the Service interface.
func (t *Blockd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (t *Blockd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

func (t *Blockd) Start(data *userdata.UserData) error {
	// Set the image.
	var image string
	if data.Services.Blockd != nil && data.Services.Blockd.Image != "" {
		image = data.Services.Blockd.Image
	} else {
		image = "docker.io/autonomy/blockd:" + version.SHA
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          t.ID(data),
		ProcessArgs: []string{"/blockd", "--userdata=" + constants.UserDataPath},
	}
	if data.IsWorker() {
		args.ProcessArgs = append(args.ProcessArgs, "--generate=true")
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/var/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/run/factory", Source: "/run/blockd", Options: []string{"rbind", "rshared", "rw"}},
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
