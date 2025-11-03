// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talos implements safe Talos API PKI rotation for the cluster.
package talos

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-retry/retry"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/cluster"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	secretsres "github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/siderolabs/talos/pkg/rotate/pki/internal/helpers"
)

// Options is the input to the Talos API rotation process.
type Options struct {
	// DryRun is the flag to enable dry-run mode.
	//
	// In dry-run mode, the rotation process will not make any changes to the cluster.
	DryRun bool

	// CurrentClient is a Talos client for the existing PKI.
	CurrentClient *client.Client
	// ClusterInfo provides information about cluster topology.
	ClusterInfo cluster.Info

	// ContextName is the context name for the 'talosconfig'.
	ContextName string
	// Endpoints is the list of endpoints for the 'talosconfig'.
	Endpoints []string

	// NewTalosCA is the new CA for Talos API.
	NewTalosCA *x509.PEMEncodedCertificateAndKey

	// EncoderOption is the option for encoding machine configuration (while patching).
	EncoderOption encoder.Option

	// Printf is the function used to print messages.
	Printf func(format string, args ...any)
}

type rotator struct {
	opts Options

	currentCA []byte

	intermediateTalosconfig *clientconfig.Config
	newTalosconfig          *clientconfig.Config

	intermediateClient *client.Client
	newClient          *client.Client
}

// Rotate rotates the Talos API PKI.
//
// The process overview:
//   - fetch current information
//   - verify connectivity with the existing PKI
//   - add new Talos CA as accepted
//   - verify connectivity with the intermediate PKI
//   - make new CA issuing, old CA is still accepted
//   - verify connectivity with the new PKI
//   - remove old Talos CA
//   - verify connectivity with the new PKI.
func Rotate(ctx context.Context, opts Options) (*clientconfig.Config, error) {
	r := rotator{
		opts: opts,
	}

	defer func() {
		if r.intermediateClient != nil {
			r.intermediateClient.Close() //nolint:errcheck
		}

		if r.newClient != nil {
			r.newClient.Close() //nolint:errcheck
		}
	}()

	err := r.rotate(ctx)

	return r.newTalosconfig, err
}

//nolint:gocyclo
func (r *rotator) rotate(ctx context.Context) error {
	r.printIntro()

	if err := r.fetchCurrentCA(ctx); err != nil {
		return err
	}

	if err := r.printNewCA(); err != nil {
		return err
	}

	if err := r.generateClients(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.opts.CurrentClient, "existing PKI"); err != nil {
		return err
	}

	if err := r.addNewCAAccepted(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.intermediateClient, "new client cert, but old server CA"); err != nil {
		return err
	}

	if err := r.swapCAs(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.newClient, "new PKI"); err != nil {
		return err
	}

	if err := r.dropOldCA(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.newClient, "new PKI"); err != nil {
		return err
	}

	return nil
}

func (r *rotator) printIntro() {
	r.opts.Printf("> Starting Talos API PKI rotation, dry-run mode %v...\n", r.opts.DryRun)
	r.opts.Printf("> Using config context: %q\n", r.opts.ContextName)
	r.opts.Printf("> Using Talos API endpoints: %q\n", r.opts.Endpoints)

	r.opts.Printf("> Cluster topology:\n")

	r.opts.Printf("  - control plane nodes: %q\n",
		append(
			helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeInit)),
			helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane))...,
		),
	)
	r.opts.Printf("  - worker nodes: %q\n",
		helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeWorker)),
	)
}

func (r *rotator) fetchCurrentCA(ctx context.Context) error {
	r.opts.Printf("> Current Talos CA:\n")

	firstNode := append(
		r.opts.ClusterInfo.NodesByType(machine.TypeInit),
		r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane)...,
	)[0]

	osRoot, err := safe.StateGetByID[*secretsres.OSRoot](client.WithNode(ctx, firstNode.InternalIP.String()), r.opts.CurrentClient.COSI, secretsres.OSRootID)
	if err != nil {
		return fmt.Errorf("error fetching existing Talos CA: %w", err)
	}

	r.currentCA = osRoot.TypedSpec().IssuingCA.Crt

	var b bytes.Buffer

	if err = yaml.NewEncoder(&b).Encode(osRoot.TypedSpec().IssuingCA); err != nil {
		return fmt.Errorf("error encoding new Talos CA: %w", err)
	}

	for scanner := bufio.NewScanner(&b); scanner.Scan(); {
		r.opts.Printf("  %s\n", scanner.Text())
	}

	return nil
}

func (r *rotator) printNewCA() error {
	r.opts.Printf("> New Talos CA:\n")

	var b bytes.Buffer

	if err := yaml.NewEncoder(&b).Encode(r.opts.NewTalosCA); err != nil {
		return fmt.Errorf("error encoding new Talos CA: %w", err)
	}

	for scanner := bufio.NewScanner(&b); scanner.Scan(); {
		r.opts.Printf("  %s\n", scanner.Text())
	}

	return nil
}

func (r *rotator) generateClients(ctx context.Context) error {
	r.opts.Printf("> Generating new talosconfig:\n")

	newBundle := &secrets.Bundle{
		Clock: secrets.NewFixedClock(time.Now()),
		Certs: &secrets.Certs{
			OS: r.opts.NewTalosCA,
		},
	}

	cert, err := newBundle.GenerateTalosAPIClientCertificate(role.MakeSet(role.Admin))
	if err != nil {
		return fmt.Errorf("error generating new talosconfig: %w", err)
	}

	// using old server CA, but new client cert
	r.intermediateTalosconfig = clientconfig.NewConfig(r.opts.ContextName, r.opts.Endpoints, r.currentCA, cert)

	// using new server CA and a new client cert
	r.newTalosconfig = clientconfig.NewConfig(r.opts.ContextName, r.opts.Endpoints, r.opts.NewTalosCA.Crt, cert)

	marshalledTalosconfig, err := r.newTalosconfig.Bytes()
	if err != nil {
		return fmt.Errorf("error marshaling talosconfig: %w", err)
	}

	r.opts.Printf("%s\n", string(marshalledTalosconfig))

	r.intermediateClient, err = client.New(ctx,
		client.WithConfig(r.intermediateTalosconfig),
	)
	if err != nil {
		return fmt.Errorf("error creating intermediate client: %w", err)
	}

	r.newClient, err = client.New(ctx,
		client.WithConfig(r.newTalosconfig),
	)
	if err != nil {
		return fmt.Errorf("error creating new client: %w", err)
	}

	return nil
}

func (r *rotator) verifyConnectivity(ctx context.Context, c *client.Client, label string) error {
	r.opts.Printf("> Verifying connectivity with %s:\n", label)

	for _, node := range r.opts.ClusterInfo.Nodes() {
		if r.opts.DryRun {
			r.opts.Printf("  - %s: OK (dry-run)\n", node.InternalIP)

			continue
		}

		var resp *machineapi.VersionResponse

		if err := retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond), retry.WithErrorLogging(true)).RetryWithContext(ctx, func(ctx context.Context) error {
			nodeCtx := client.WithNode(ctx, node.InternalIP.String())

			var respErr error

			resp, respErr = c.Version(nodeCtx)
			if respErr != nil {
				if client.StatusCode(respErr) == codes.Unavailable {
					return retry.ExpectedError(respErr)
				}

				return respErr
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error calling version API on node %s: %w", node.InternalIP, err)
		}

		r.opts.Printf("  - %s: OK (version %s)\n", node.InternalIP, resp.Messages[0].Version.GetTag())
	}

	return nil
}

func (r *rotator) addNewCAAccepted(ctx context.Context) error {
	r.opts.Printf("> Adding new Talos CA as accepted...\n")

	if err := r.patchAllNodes(ctx, r.opts.CurrentClient,
		func(_ machine.Type, config *v1alpha1.Config) error {
			config.MachineConfig.MachineAcceptedCAs = append(
				config.MachineConfig.MachineAcceptedCAs,
				&x509.PEMEncodedCertificate{
					Crt: r.opts.NewTalosCA.Crt,
				},
			)

			return nil
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) swapCAs(ctx context.Context) error {
	r.opts.Printf("> Making new Talos CA the issuing CA, old Talos CA the accepted CA...\n")

	if err := r.patchAllNodes(ctx, r.intermediateClient,
		func(machineType machine.Type, config *v1alpha1.Config) error {
			config.MachineConfig.MachineAcceptedCAs = append(
				config.MachineConfig.MachineAcceptedCAs,
				&x509.PEMEncodedCertificate{
					Crt: r.currentCA,
				},
			)
			config.MachineConfig.MachineAcceptedCAs = slices.DeleteFunc(config.Machine().Security().AcceptedCAs(), func(ca *x509.PEMEncodedCertificate) bool {
				return bytes.Equal(ca.Crt, r.opts.NewTalosCA.Crt)
			})

			if machineType.IsControlPlane() {
				config.MachineConfig.MachineCA = r.opts.NewTalosCA
			} else {
				config.MachineConfig.MachineCA = &x509.PEMEncodedCertificateAndKey{
					Crt: r.opts.NewTalosCA.Crt,
				}
			}

			return nil
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) dropOldCA(ctx context.Context) error {
	r.opts.Printf("> Removing old Talos CA from the accepted CAs...\n")

	if err := r.patchAllNodes(ctx, r.newClient,
		func(_ machine.Type, config *v1alpha1.Config) error {
			config.MachineConfig.MachineAcceptedCAs = slices.DeleteFunc(config.Machine().Security().AcceptedCAs(), func(ca *x509.PEMEncodedCertificate) bool {
				return bytes.Equal(ca.Crt, r.currentCA)
			})

			return nil
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) patchAllNodes(ctx context.Context, c *client.Client, patchFunc func(machineType machine.Type, config *v1alpha1.Config) error) error {
	for _, machineType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
		for _, node := range r.opts.ClusterInfo.NodesByType(machineType) {
			if r.opts.DryRun {
				r.opts.Printf("  - %s: skipped (dry-run)\n", node.InternalIP)

				continue
			}

			if err := helpers.PatchNodeConfig(ctx, c, node.InternalIP.String(), r.opts.EncoderOption, func(config *v1alpha1.Config) error {
				return patchFunc(machineType, config)
			}); err != nil {
				return fmt.Errorf("error patching node %s: %w", node.InternalIP, err)
			}

			r.opts.Printf("  - %s: OK\n", node.InternalIP)
		}
	}

	return nil
}
