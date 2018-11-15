// nolint: dupl,golint
package services

import (
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	"github.com/autonomy/talos/src/initramfs/pkg/version"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Proxyd implements the Service interface. It serves as the concrete type with
// the required methods.
type Proxyd struct{}

// ID implements the Service interface.
func (p *Proxyd) ID(data *userdata.UserData) string {
	return "proxyd"
}

// PreFunc implements the Service interface.
func (p *Proxyd) PreFunc(data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (p *Proxyd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (p *Proxyd) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.WaitForFilesToExist("/var/etc/kubernetes/pki/ca.crt", "/var/etc/kubernetes/admin.conf")
}

func (p *Proxyd) Start(data *userdata.UserData) error {
	// Set the image.
	var image string
	if data.Services.Proxyd != nil && data.Services.Proxyd.Image != "" {
		image = data.Services.Proxyd.Image
	} else {
		image = "docker.io/autonomy/proxyd:" + version.SHA
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          p.ID(data),
		ProcessArgs: []string{"/proxyd"},
	}

	// Set the mounts.
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/kubernetes/admin.conf", Source: "/var/etc/kubernetes/admin.conf", Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes/pki/ca.crt", Source: "/var/etc/kubernetes/pki/ca.crt", Options: []string{"rbind", "ro"}},
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithContainerImage(image),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			oci.WithMounts(mounts),
			oci.WithPrivileged,
		),
	)
}
