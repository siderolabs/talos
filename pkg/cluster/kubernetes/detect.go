// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DetectLowestVersion returns lowest Kubernetes components versions in the cluster.
//
//nolint:gocyclo
func DetectLowestVersion(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) (string, error) {
	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return "", fmt.Errorf("error building kubernetes client: %w", err)
	}

	apps := map[string]struct{}{
		"kube-apiserver":          {},
		"kube-controller-manager": {},
		"kube-proxy":              {},
		"kube-scheduler":          {},
	}

	pods, err := k8sClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	var version *semver.Version

	for _, pod := range pods.Items {
		app := pod.GetObjectMeta().GetLabels()["k8s-app"]
		if _, ok := apps[app]; !ok {
			continue
		}

		for _, container := range pod.Spec.Containers {
			if container.Name != app {
				continue
			}

			idx := strings.LastIndex(container.Image, ":")
			if idx == -1 {
				continue
			}

			v, err := semver.ParseTolerant(strings.TrimLeft(container.Image[idx+1:], "v"))
			if err != nil {
				options.Log("failed to parse %s container version %s", app, err)

				continue
			}

			if version == nil || v.LT(*version) {
				version = &v
			}
		}
	}

	if version == nil {
		return "", errors.New("failed to detect lowest Kubernetes version")
	}

	return version.String(), nil
}
