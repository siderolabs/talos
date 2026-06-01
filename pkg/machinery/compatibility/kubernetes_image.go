// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package compatibility

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/version"
)

// ValidateKubernetesImageTag validates the Kubernetes image tag format.
func ValidateKubernetesImageTag(imageRef string) error {
	// this method is called from Validate in !Local mode, so we are inside running Talos,
	// so the version of Talos is available, and we can check compatibility
	currentTalosVersion, err := ParseTalosVersion(version.NewVersion())
	if err != nil {
		return fmt.Errorf("failed to parse Talos version: %w", err)
	}

	k8sVersion, err := KubernetesVersionFromImageRef(imageRef)
	if err != nil {
		return fmt.Errorf("failed to parse Kubernetes version from image reference %q: %w", imageRef, err)
	}

	return k8sVersion.SupportedWith(currentTalosVersion)
}

// KubernetesVersionFromImageRef parses the Kubernetes version from the image reference.
func KubernetesVersionFromImageRef(ref string) (*KubernetesVersion, error) {
	idx := strings.LastIndex(ref, ":v")
	if idx == -1 {
		return nil, fmt.Errorf("invalid image reference: %q", ref)
	}

	versionPart := ref[idx+2:]

	if shaIndex := strings.Index(versionPart, "@"); shaIndex != -1 {
		versionPart = versionPart[:shaIndex]
	}

	return ParseKubernetesVersion(versionPart)
}
