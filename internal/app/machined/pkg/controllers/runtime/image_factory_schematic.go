// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ImageFactorySchematicController watches ExtensionStatus resources and surfaces the
// Image Factory schematic extension as a first-class ImageFactorySchematic resource.
type ImageFactorySchematicController struct{}

// Name implements controller.Controller interface.
func (ctrl *ImageFactorySchematicController) Name() string {
	return "runtime.ImageFactorySchematicController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ImageFactorySchematicController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.ExtensionStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ImageFactorySchematicController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.ImageFactorySchematicType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ImageFactorySchematicController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		extensionStatuses, err := safe.ReaderListAll[*runtime.ExtensionStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing extension statuses: %w", err)
		}

		for extensionStatus := range extensionStatuses.All() {
			if extensionStatus.TypedSpec().Metadata.Name != constants.ImageFactorySchematicExtensionName {
				continue
			}

			schematicID := extensionStatus.TypedSpec().Metadata.Version
			flavor, apiURL := parseAuthor(extensionStatus.TypedSpec().Metadata.Author)

			if schematicID == "" {
				logger.Warn("schematic extension has empty version (schematic ID), skipping")

				continue
			}

			if err = safe.WriterModify(ctx, r,
				runtime.NewImageFactorySchematic(runtime.NamespaceName, runtime.ImageFactorySchematicID),
				func(res *runtime.ImageFactorySchematic) error {
					res.TypedSpec().SchematicID = schematicID
					res.TypedSpec().Flavor = flavor
					res.TypedSpec().APIURL = apiURL

					return nil
				},
			); err != nil {
				return fmt.Errorf("error updating image factory schematic: %w", err)
			}

			break
		}

		if err = safe.CleanupOutputs[*runtime.ImageFactorySchematic](ctx, r); err != nil {
			return err
		}
	}
}

// parseAuthor extracts the flavor name and URL from an author string like
// "Image Factory (https://factory.talos.dev)".
// If no " (" is found, the whole string is returned as flavor and apiURL is empty.
func parseAuthor(author string) (flavor, apiURL string) {
	idx := strings.LastIndex(author, " (")
	if idx == -1 {
		return author, ""
	}

	flavor = author[:idx]
	url := author[idx+2:]

	apiURL = strings.TrimSuffix(url, ")")

	return flavor, apiURL
}
