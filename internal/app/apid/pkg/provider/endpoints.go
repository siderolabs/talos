// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/pkg/kubernetes"
)

// Endpoints interfaces describes a control plane endpoints provider.
type Endpoints interface {
	GetEndpoints() (endpoints []string, err error)
}

// StaticEndpoints provides static list of endpoints.
type StaticEndpoints struct {
	Endpoints []string
}

// GetEndpoints implements Endpoints inteface.
func (e *StaticEndpoints) GetEndpoints() (endpoints []string, err error) {
	return e.Endpoints, nil
}

// KubernetesEndpoints provides dynamic list of control plane endpoints via Kubernetes Endpoints resource.
type KubernetesEndpoints struct{}

// GetEndpoints implements Endpoints inteface.
func (e *KubernetesEndpoints) GetEndpoints() (endpoints []string, err error) {
	err = retry.Constant(8*time.Minute, retry.WithUnits(3*time.Second), retry.WithJitter(time.Second), retry.WithErrorLogging(true)).Retry(func() error {
		ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ctxCancel()

		var client *kubernetes.Client

		client, err = kubernetes.NewClientFromKubeletKubeconfig()
		if err != nil {
			return retry.ExpectedError(fmt.Errorf("failed to create client: %w", err))
		}

		endpoints, err = client.MasterIPs(ctx)
		if err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})

	return endpoints, err
}
