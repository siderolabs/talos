// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/crypto/x509"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/grpc/gen"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/role"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/network"
	"github.com/talos-systems/talos/pkg/resources/secrets"
	timeresource "github.com/talos-systems/talos/pkg/resources/time"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// APIController manages secrets.API based on configuration to provide apid certificate.
type APIController struct{}

// Name implements controller.Controller interface.
func (ctrl *APIController) Name() string {
	return "secrets.APIController"
}

// Inputs implements controller.Controller interface.
func (ctrl *APIController) Inputs() []controller.Input {
	// initial set of inputs: wait for machine type to be known and network to be partially configured
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        pointer.ToString(network.StatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *APIController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.APIType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *APIController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		networkResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		networkStatus := networkResource.(*network.Status).TypedSpec()

		if !(networkStatus.AddressReady && networkStatus.HostnameReady) {
			continue
		}

		// machine type is known and network is ready, we can now proceed to one or another reconcile loop
		switch machineType {
		case machine.TypeInit, machine.TypeControlPlane:
			if err = ctrl.reconcileControlPlane(ctx, r, logger); err != nil {
				return err
			}
		case machine.TypeJoin:
			if err = ctrl.reconcileJoin(ctx, r, logger); err != nil {
				return err
			}
		case machine.TypeUnknown: // nothig to do
		}

		if err = ctrl.teardownAll(ctx, r); err != nil {
			return err
		}
	}
}

//nolint:gocyclo,cyclop
func (ctrl *APIController) reconcileControlPlane(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.RootType,
			ID:        pointer.ToString(secrets.RootOSID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        pointer.ToString(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        pointer.ToString(network.NodeAddressAccumulativeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		// time status isn't fetched, but the fact that it is in dependencies means
		// that certs will be regenerated on time sync/jump (as reconcile will be triggered)
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeresource.StatusType,
			ID:        pointer.ToString(timeresource.StatusID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return fmt.Errorf("error updating inputs: %w", err)
	}

	r.QueueReconcile()

	refreshTicker := time.NewTicker(x509.DefaultCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		if machineType != machine.TypeInit && machineType != machine.TypeControlPlane {
			// machine type changed, terminate
			return fmt.Errorf("machine type changed")
		}

		rootResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, secrets.RootOSID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting etcd root secrets: %w", err)
		}

		rootSpec := rootResource.(*secrets.Root).OSSpec()

		hostnameResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		hostnameStatus := hostnameResource.(*network.HostnameStatus).TypedSpec()

		addressesResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressAccumulativeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		nodeAddresses := addressesResource.(*network.NodeAddress).TypedSpec()

		ca, err := x509.NewCertificateAuthorityFromCertificateAndKey(rootSpec.CA)
		if err != nil {
			return fmt.Errorf("failed to parse CA certificate: %w", err)
		}

		ips := make([]net.IP, 0, len(rootSpec.CertSANIPs)+len(nodeAddresses.Addresses))

		for _, ip := range rootSpec.CertSANIPs {
			ips = append(ips, ip.IPAddr().IP)
		}

		for _, ip := range nodeAddresses.Addresses {
			ips = append(ips, ip.IPAddr().IP)
		}

		dnsNames := make([]string, 0, len(rootSpec.CertSANDNSNames)+2)

		dnsNames = append(dnsNames, rootSpec.CertSANDNSNames...)
		dnsNames = append(dnsNames, hostnameStatus.Hostname)

		if hostnameStatus.FQDN() != hostnameStatus.Hostname {
			dnsNames = append(dnsNames, hostnameStatus.FQDN())
		}

		// TODO: add keyusage

		serverCert, err := x509.NewKeyPair(ca,
			x509.IPAddresses(ips),
			x509.DNSNames(dnsNames),
			x509.CommonName(hostnameStatus.FQDN()),
			x509.NotAfter(time.Now().Add(x509.DefaultCertificateValidityDuration)),
		)
		if err != nil {
			return fmt.Errorf("failed to generate API server cert: %w", err)
		}

		clientCert, err := x509.NewKeyPair(ca,
			x509.CommonName(hostnameStatus.FQDN()),
			x509.Organization(string(role.Impersonator)),
			x509.NotAfter(time.Now().Add(x509.DefaultCertificateValidityDuration)),
		)
		if err != nil {
			return fmt.Errorf("failed to generate API client cert: %w", err)
		}

		if err := r.Modify(ctx, secrets.NewAPI(),
			func(r resource.Resource) error {
				apiSecrets := r.(*secrets.API).TypedSpec()

				apiSecrets.CA = &x509.PEMEncodedCertificateAndKey{
					Crt: rootSpec.CA.Crt,
				}
				apiSecrets.Server = x509.NewCertificateAndKeyFromKeyPair(serverCert)
				apiSecrets.Client = x509.NewCertificateAndKeyFromKeyPair(clientCert)

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		clientFingerprint, _ := x509.SPKIFingerprintFromDER(clientCert.Certificate.Certificate[0]) //nolint:errcheck
		serverFingerprint, _ := x509.SPKIFingerprintFromDER(serverCert.Certificate.Certificate[0]) //nolint:errcheck

		logger.Debug("generated new certificates",
			zap.Stringer("client", clientFingerprint),
			zap.Stringer("server", serverFingerprint),
		)
	}
}

//nolint:gocyclo,cyclop
func (ctrl *APIController) reconcileJoin(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        pointer.ToString(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.RootType,
			ID:        pointer.ToString(secrets.RootOSID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        pointer.ToString(network.NodeAddressAccumulativeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.EndpointType,
			ID:        pointer.ToString(k8s.ControlPlaneEndpointsID),
			Kind:      controller.InputWeak,
		},
		// time status isn't fetched, but the fact that it is in dependencies means
		// that certs will be regenerated on time sync/jump (as reconcile will be triggered)
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeresource.StatusType,
			ID:        pointer.ToString(timeresource.StatusID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return fmt.Errorf("error updating inputs: %w", err)
	}

	r.QueueReconcile()

	refreshTicker := time.NewTicker(x509.DefaultCertificateValidityDuration / 2)
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-refreshTicker.C:
		}

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		if machineType != machine.TypeJoin {
			// machine type changed, terminate
			return fmt.Errorf("machine type changed")
		}

		endpointResource, err := r.Get(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.EndpointType, k8s.ControlPlaneEndpointsID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting endpoints resource: %w", err)
		}

		endpoints := endpointResource.(*k8s.Endpoint).TypedSpec()

		if len(endpoints.Addresses) == 0 {
			continue
		}

		rootResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, secrets.RootOSID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting etcd root secrets: %w", err)
		}

		rootSpec := rootResource.(*secrets.Root).OSSpec()

		hostnameResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		hostnameStatus := hostnameResource.(*network.HostnameStatus).TypedSpec()

		addressesResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressAccumulativeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		nodeAddresses := addressesResource.(*network.NodeAddress).TypedSpec()

		endpointsStr := make([]string, 0, len(endpoints.Addresses))

		for _, ip := range endpoints.Addresses {
			endpointsStr = append(endpointsStr, ip.String())
		}

		remoteGen, err := gen.NewRemoteGenerator(rootSpec.Token, endpointsStr)
		if err != nil {
			return fmt.Errorf("error creating remote generator: %w", err)
		}

		// TODO: close remoteGen

		ips := make([]net.IP, 0, len(rootSpec.CertSANIPs)+len(nodeAddresses.Addresses))

		for _, ip := range rootSpec.CertSANIPs {
			ips = append(ips, ip.IPAddr().IP)
		}

		for _, ip := range nodeAddresses.Addresses {
			ips = append(ips, ip.IPAddr().IP)
		}

		dnsNames := make([]string, 0, len(rootSpec.CertSANDNSNames)+2)

		dnsNames = append(dnsNames, rootSpec.CertSANDNSNames...)
		dnsNames = append(dnsNames, hostnameStatus.Hostname)

		if hostnameStatus.FQDN() != hostnameStatus.Hostname {
			dnsNames = append(dnsNames, hostnameStatus.FQDN())
		}

		serverCSR, serverCert, err := x509.NewEd25519CSRAndIdentity(
			x509.IPAddresses(ips),
			x509.DNSNames(dnsNames),
			x509.CommonName(hostnameStatus.FQDN()),
		)
		if err != nil {
			return fmt.Errorf("failed to generate API server CSR: %w", err)
		}

		var ca []byte

		ca, serverCert.Crt, err = remoteGen.Identity(serverCSR)
		if err != nil {
			return fmt.Errorf("failed to sign API server CSR: %w", err)
		}

		clientCSR, clientCert, err := x509.NewEd25519CSRAndIdentity(
			x509.CommonName(hostnameStatus.FQDN()),
			x509.Organization(string(role.Impersonator)),
		)
		if err != nil {
			return fmt.Errorf("failed to generate API client CSR: %w", err)
		}

		_, clientCert.Crt, err = remoteGen.Identity(clientCSR)
		if err != nil {
			return fmt.Errorf("failed to sign API client CSR: %w", err)
		}

		if err := r.Modify(ctx, secrets.NewAPI(),
			func(r resource.Resource) error {
				apiSecrets := r.(*secrets.API).TypedSpec()

				apiSecrets.CA = &x509.PEMEncodedCertificateAndKey{
					Crt: ca,
				}
				apiSecrets.Server = serverCert
				apiSecrets.Client = clientCert

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		clientFingerprint, _ := x509.SPKIFingerprintFromPEM(clientCert.Crt) //nolint:errcheck
		serverFingerprint, _ := x509.SPKIFingerprintFromPEM(serverCert.Crt) //nolint:errcheck

		logger.Debug("generated new certificates",
			zap.Stringer("client", clientFingerprint),
			zap.Stringer("server", serverFingerprint),
		)
	}
}

func (ctrl *APIController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.APIType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	// TODO: change this to proper teardown sequence

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}
