// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package security

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

// TUFTrustedRootController fetches root TUF trusted roots.
type TUFTrustedRootController struct {
	RefreshInterval time.Duration

	lastRefresh time.Time
}

// DefaultTUFRefreshInterval is the default interval for refreshing TUF trusted roots.
const DefaultTUFRefreshInterval = 24 * time.Hour

// Name implements controller.Controller interface.
func (ctrl *TUFTrustedRootController) Name() string {
	return "security.TUFTrustedRootController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TUFTrustedRootController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: security.NamespaceName,
			Type:      security.ImageVerificationRuleType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TUFTrustedRootController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: security.TUFTrustedRootType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *TUFTrustedRootController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.RefreshInterval == 0 {
		ctrl.RefreshInterval = DefaultTUFRefreshInterval
	}

	ticker := time.NewTicker(ctrl.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		// first, determine if we need to fetch TUF at all
		rules, err := safe.ReaderListAll[*security.ImageVerificationRule](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list image verification rules: %w", err)
		}

		needsTUF := false

		for rule := range rules.All() {
			if rule.TypedSpec().KeylessVerifier != nil {
				needsTUF = true

				break
			}
		}

		// suppress TUF refresh if not needed
		if needsTUF && time.Since(ctrl.lastRefresh) < ctrl.RefreshInterval {
			continue
		}

		r.StartTrackingOutputs()

		if needsTUF {
			tufData, err := ctrl.getTrustedRootTarget(security.TrustedRootID)
			if err != nil {
				return fmt.Errorf("failed to get TUF trusted root: %w", err)
			}

			ctrl.lastRefresh = time.Now()

			if err := safe.WriterModify(
				ctx, r,
				security.NewTUFTrustedRoot(security.TrustedRootID),
				func(root *security.TUFTrustedRoot) error {
					root.TypedSpec().JSONData = string(tufData)
					root.TypedSpec().LastRefreshTime = ctrl.lastRefresh

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to create/update TUF trusted root: %w", err)
			}

			logger.Info("refreshed TUF trusted root")
		} else {
			// we are going to remove TUF, so reset last refresh time
			ctrl.lastRefresh = time.Time{}
		}

		if err := safe.CleanupOutputs[*security.TUFTrustedRoot](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

func (ctrl *TUFTrustedRootController) getTrustedRootTarget(id string) ([]byte, error) {
	transport := httpdefaults.PatchTransport(cleanhttp.DefaultTransport())
	httpClient := &http.Client{
		Transport: transport,
	}

	fetcher := fetcher.NewDefaultFetcher()
	fetcher.SetHTTPClient(httpClient)
	fetcher.SetHTTPUserAgent(httpdefaults.UserAgent())

	opts := tuf.Options{
		Root:              tuf.DefaultRoot(),
		RepositoryBaseURL: tuf.DefaultMirror,
		DisableLocalCache: true,
		Fetcher:           fetcher,
	}

	client, err := tuf.New(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUF client: %w", err)
	}

	return client.GetTarget(id)
}
