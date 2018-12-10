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

// Trustd implements the Service interface. It serves as the concrete type with
// the required methods.
type Trustd struct{}

// ID implements the Service interface.
func (t *Trustd) ID(data *userdata.UserData) string {
	return "trustd"
}

// PreFunc implements the Service interface.
func (t *Trustd) PreFunc(data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (t *Trustd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (t *Trustd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

func (t *Trustd) Start(data *userdata.UserData) error {
	// Set the image.
	var image string
	if data.Services.Trustd != nil && data.Services.Trustd.Image != "" {
		image = data.Services.Trustd.Image
	} else {
		image = "docker.io/autonomy/trustd:" + version.SHA
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          t.ID(data),
		ProcessArgs: []string{"/trustd", "--userdata=" + constants.UserDataPath},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.UserDataPath, Source: constants.UserDataPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/var/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
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
