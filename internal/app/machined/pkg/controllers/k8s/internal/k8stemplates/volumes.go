// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	"github.com/siderolabs/gen/xslices"
	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// VolumeMounts translates definition into K8s volume mount specs.
func VolumeMounts(volumes []k8s.ExtraVolume) []v1.VolumeMount {
	return xslices.Map(volumes, func(vol k8s.ExtraVolume) v1.VolumeMount {
		return v1.VolumeMount{
			Name:      vol.Name,
			MountPath: vol.MountPath,
			ReadOnly:  vol.ReadOnly,
		}
	})
}

// EphemeralWritableMounts returns the volume mounts for the scratch directories that
// control plane components need to write to even when running with a read-only root
// filesystem: self-signed serving certificates land in /var/run/kubernetes (the default
// --cert-dir), and the Go runtime and the binaries use /tmp for temporary files.
func EphemeralWritableMounts() []v1.VolumeMount {
	return []v1.VolumeMount{
		{
			Name:      "tmp",
			MountPath: "/tmp",
		},
		{
			Name:      "run",
			MountPath: "/var/run/kubernetes",
		},
	}
}

// EphemeralWritableVolumes returns the tmpfs-backed emptyDir volumes that back
// EphemeralWritableMounts.
func EphemeralWritableVolumes() []v1.Volume {
	return []v1.Volume{
		{
			Name: "tmp",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
		{
			Name: "run",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
	}
}

// Volumes translates definition into K8s volume specs.
func Volumes(volumes []k8s.ExtraVolume) []v1.Volume {
	return xslices.Map(volumes, func(vol k8s.ExtraVolume) v1.Volume {
		return v1.Volume{
			Name: vol.Name,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: vol.HostPath,
				},
			},
		}
	})
}
