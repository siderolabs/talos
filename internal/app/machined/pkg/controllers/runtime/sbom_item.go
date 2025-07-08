// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// SBOMItemController is a controller that publishes Talos SBOMs as resources.
type SBOMItemController struct {
	SPDXPath string
}

// Name implements controller.Controller interface.
func (ctrl *SBOMItemController) Name() string {
	return "runtime.SBOMItemController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SBOMItemController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *SBOMItemController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.SBOMItemType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *SBOMItemController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.SPDXPath == "" {
		ctrl.SPDXPath = constants.SPDXPath
	}

	// the controller runs a single time
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	files, err := os.ReadDir(ctrl.SPDXPath)
	if err != nil {
		return fmt.Errorf("failed to read SBOM directory %q: %w", ctrl.SPDXPath, err)
	}

	for _, file := range files {
		if !file.Type().IsRegular() {
			logger.Debug("skipping non-regular file", zap.String("file", file.Name()))

			continue
		}

		if !strings.HasSuffix(file.Name(), ".spdx.json") {
			logger.Debug("skipping non-SPDX file", zap.String("file", file.Name()))

			continue
		}

		if err = ctrl.processSPDXFile(ctx, r, filepath.Join(ctrl.SPDXPath, file.Name())); err != nil {
			return fmt.Errorf("failed to process SBOM file %q: %w", file.Name(), err)
		}
	}

	return nil
}

// spdxDocument is a reduced structure of SPDX document.
//
// We are only interested in some fields.
type spdxDocument struct {
	Packages []spdxPackage `json:"packages"`
}

type spdxPackage struct {
	Name         string            `json:"name"`
	Version      string            `json:"versionInfo"`
	License      string            `json:"licenseDeclared"`
	ExternalRefs []spdxExternalRef `json:"externalRefs"`
}

type spdxExternalRef struct {
	Type    string `json:"referenceType"`
	Locator string `json:"referenceLocator"`
}

func (ctrl *SBOMItemController) processSPDXFile(ctx context.Context, r controller.Runtime, path string) error {
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open SBOM file %q: %w", path, err)
	}

	defer in.Close() //nolint:errcheck

	var doc spdxDocument

	if err := json.NewDecoder(in).Decode(&doc); err != nil {
		return fmt.Errorf("failed to decode SBOM file %q: %w", path, err)
	}

	for _, pkg := range doc.Packages {
		if strings.HasPrefix(pkg.Name, version.Name+" (") {
			pkg.Name = version.Name
		}

		if err := safe.WriterModify(ctx, r, runtimeres.NewSBOMItemSpec(runtimeres.NamespaceName, pkg.Name),
			func(item *runtimeres.SBOMItem) error {
				item.TypedSpec().Name = pkg.Name
				item.TypedSpec().Version = pkg.Version

				if pkg.License != "NOASSERTION" {
					item.TypedSpec().License = pkg.License
				}

				for _, ref := range pkg.ExternalRefs {
					switch ref.Type {
					case "cpe23Type":
						item.TypedSpec().CPEs = append(item.TypedSpec().CPEs, ref.Locator)
					case "purl":
						item.TypedSpec().PURLs = append(item.TypedSpec().PURLs, ref.Locator)
					}
				}

				return nil
			}); err != nil {
			return fmt.Errorf("failed to create SBOM item for package %q: %w", pkg.Name, err)
		}
	}

	return nil
}
