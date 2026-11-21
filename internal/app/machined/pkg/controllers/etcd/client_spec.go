// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// ClientSpecController provides the configuration of the etcd client: the credentials and the endpoints.
//
// The client spec allows to talk to etcd without touching the etcd PKI on disk,
// and without discovering the etcd endpoints over and over again.
type ClientSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *ClientSpecController) Name() string {
	return "etcd.ClientSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClientSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.EtcdRootType,
			ID:        optional.Some(secrets.EtcdRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.EtcdType,
			ID:        optional.Some(secrets.EtcdID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.EndpointType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClientSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.ClientSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ClientSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		rootSecrets, err := safe.ReaderGetByID[*secrets.EtcdRoot](ctx, r, secrets.EtcdRootID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting etcd root secrets: %w", err)
		}

		etcdSecrets, err := safe.ReaderGetByID[*secrets.Etcd](ctx, r, secrets.EtcdID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting etcd secrets: %w", err)
		}

		if rootSecrets == nil || etcdSecrets == nil || etcdSecrets.TypedSpec().EtcdAdmin == nil {
			// no credentials (yet, or anymore)
			if err = r.Destroy(ctx, etcd.NewClientSpec().Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying etcd client spec: %w", err)
			}

			r.ResetRestartBackoff()

			continue
		}

		// the list is empty until the control plane machines are discovered
		clusterEndpoints, err := etcdcli.ListEndpoints(ctx, r)
		if err != nil {
			return err
		}

		if err = safe.WriterModify(ctx, r, etcd.NewClientSpec(), func(spec *etcd.ClientSpec) error {
			spec.TypedSpec().AcceptedCAs = rootSecrets.TypedSpec().AcceptedCAs
			spec.TypedSpec().Certificate = etcdSecrets.TypedSpec().EtcdAdmin
			spec.TypedSpec().LocalEndpoints = etcdcli.LocalEndpoints()
			spec.TypedSpec().ClusterEndpoints = clusterEndpoints

			return nil
		}); err != nil {
			return fmt.Errorf("error updating etcd client spec: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
