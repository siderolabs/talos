// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// WifiServiceManager is the interface to the v1alpha1 service subsystem.
type WifiServiceManager interface {
	IsRunning(id string) (system.Service, bool, error)
	Load(services ...system.Service) []string
	Start(serviceIDs ...string) error
	Stop(ctx context.Context, serviceIDs ...string) error
	Unload(ctx context.Context, serviceIDs ...string) error
}

// WifiServiceController manages per-interface wpa_supplicant services based on network.WifiSpec resources.
type WifiServiceController struct {
	V1Alpha1Services WifiServiceManager

	// map of link name -> rendered wpa_supplicant configuration for running supplicants
	running map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *WifiServiceController) Name() string {
	return "network.WifiServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *WifiServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.WifiSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *WifiServiceController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *WifiServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.running == nil {
		ctrl.running = map[string]string{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *WifiServiceController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Services == nil {
		return fmt.Errorf("wifi service manager is not configured")
	}

	specs, err := safe.ReaderListAll[*network.WifiSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing WifiSpecs: %w", err)
	}

	// desired set of supplicants: WifiSpecs whose link exists as a physical ethernet-like link
	desired := map[string]string{}

	for spec := range specs.All() {
		linkName := spec.Metadata().ID()

		linkStatus, err := safe.ReaderGetByID[*network.LinkStatus](ctx, r, linkName)
		if err != nil {
			continue // link is not (yet) present, skip
		}

		if linkStatus.TypedSpec().Type != nethelpers.LinkEther || linkStatus.TypedSpec().Kind != "" {
			logger.Warn("skipping wifi config for non-physical link", zap.String("link", linkName))

			continue
		}

		desired[linkName] = renderWpaSupplicantConfig(spec.TypedSpec())
	}

	// stop supplicants which are no longer desired or whose config changed
	for linkName, conf := range ctrl.running {
		if desiredConf, ok := desired[linkName]; ok && desiredConf == conf {
			continue
		}

		serviceID := services.WpaSupplicantServiceID(linkName)

		if err := ctrl.V1Alpha1Services.Stop(ctx, serviceID); err != nil {
			return fmt.Errorf("error stopping service %q: %w", serviceID, err)
		}

		if err := ctrl.V1Alpha1Services.Unload(ctx, serviceID); err != nil {
			return fmt.Errorf("error unloading service %q: %w", serviceID, err)
		}

		if err := os.Remove(services.WpaSupplicantConfigPath(linkName)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing wpa_supplicant config for %q: %w", linkName, err)
		}

		delete(ctrl.running, linkName)

		logger.Info("stopped wpa_supplicant", zap.String("link", linkName))
	}

	// start (or restart with new config) desired supplicants
	for linkName, conf := range desired {
		if runningConf, ok := ctrl.running[linkName]; ok && runningConf == conf {
			continue
		}

		if err := os.MkdirAll(constants.WifiSupplicantRunDir, 0o700); err != nil {
			return fmt.Errorf("error creating wpa_supplicant run directory: %w", err)
		}

		if err := os.WriteFile(services.WpaSupplicantConfigPath(linkName), []byte(conf), 0o600); err != nil {
			return fmt.Errorf("error writing wpa_supplicant config for %q: %w", linkName, err)
		}

		svc := &services.WpaSupplicant{LinkName: linkName}
		serviceID := services.WpaSupplicantServiceID(linkName)

		ctrl.V1Alpha1Services.Load(svc)

		_, running, err := ctrl.V1Alpha1Services.IsRunning(serviceID)
		if err != nil {
			return fmt.Errorf("error checking service %q: %w", serviceID, err)
		}

		if !running {
			if err := ctrl.V1Alpha1Services.Start(serviceID); err != nil {
				return fmt.Errorf("error starting service %q: %w", serviceID, err)
			}
		}

		ctrl.running[linkName] = conf

		logger.Info("started wpa_supplicant", zap.String("link", linkName))
	}

	return nil
}

// renderWpaSupplicantConfig renders a wpa_supplicant configuration file from the WifiSpec.
func renderWpaSupplicantConfig(spec *network.WifiSpecSpec) string {
	var sb strings.Builder

	sb.WriteString("ctrl_interface=" + constants.WifiSupplicantRunDir + "\n")

	if spec.CountryCode != "" {
		sb.WriteString("country=" + spec.CountryCode + "\n")
	}

	for i, net := range spec.Networks {
		sb.WriteString("\nnetwork={\n")
		// hex-encoded form is safe for any SSID contents
		sb.WriteString("\tssid=" + hex.EncodeToString([]byte(net.SSID)) + "\n")

		if net.Hidden {
			sb.WriteString("\tscan_ssid=1\n")
		}

		// earlier networks in the list are preferred
		fmt.Fprintf(&sb, "\tpriority=%d\n", len(spec.Networks)-i)

		if net.PSK == "" {
			sb.WriteString("\tkey_mgmt=NONE\n")
		} else {
			// support WPA2-PSK, WPA3-SAE and mixed-mode APs
			sb.WriteString("\tkey_mgmt=WPA-PSK SAE\n")
			sb.WriteString("\tieee80211w=1\n")
			sb.WriteString("\tpsk=" + quoteWpaSupplicantString(net.PSK) + "\n")
			sb.WriteString("\tsae_password=" + quoteWpaSupplicantString(net.PSK) + "\n")
		}

		sb.WriteString("}\n")
	}

	return sb.String()
}

// quoteWpaSupplicantString quotes a string for use in a wpa_supplicant configuration file.
func quoteWpaSupplicantString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
