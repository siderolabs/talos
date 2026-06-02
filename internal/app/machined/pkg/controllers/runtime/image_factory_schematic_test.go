// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type ImageFactorySchematicSuite struct {
	ctest.DefaultSuite
}

func TestImageFactorySchematicSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ImageFactorySchematicSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.ImageFactorySchematicController{}))
			},
		},
	})
}

func (suite *ImageFactorySchematicSuite) createExtensionStatus(id, name, version, author string) {
	ext := runtime.NewExtensionStatus(runtime.NamespaceName, id)
	ext.TypedSpec().Metadata = extensions.Metadata{
		Name:    name,
		Version: version,
		Author:  author,
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), ext))
}

func (suite *ImageFactorySchematicSuite) TestSchematicExtensionPresent() {
	// Create some extensions first.
	suite.createExtensionStatus("amd-ucode.sqsh", "amd-ucode", "20230901", "AMD")
	suite.createExtensionStatus("zfs.sqsh", "zfs", "2.2.0", "OpenZFS")

	// Create the schematic extension injected by Image Factory.
	const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	suite.createExtensionStatus(
		"schematic.sqsh",
		"schematic",
		schematicID,
		"Image Factory (https://factory.talos.dev)",
	)

	ctest.AssertResource(suite, runtime.ImageFactorySchematicID, func(res *runtime.ImageFactorySchematic, asrt *assert.Assertions) {
		asrt.Equal(schematicID, res.TypedSpec().SchematicID)
		asrt.Equal("Image Factory", res.TypedSpec().Flavor)
		asrt.Equal("https://factory.talos.dev", res.TypedSpec().APIURL)
	})
}

func (suite *ImageFactorySchematicSuite) TestNoSchematicExtension() {
	// Only unrelated extensions, no schematic extension.
	suite.createExtensionStatus("amd-ucode.sqsh", "amd-ucode", "20230901", "AMD")

	ctest.AssertNoResource[*runtime.ImageFactorySchematic](suite, runtime.ImageFactorySchematicID)
}

func (suite *ImageFactorySchematicSuite) TestSchematicExtensionDeleted() {
	const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	suite.createExtensionStatus(
		"schematic.sqsh",
		"schematic",
		schematicID,
		"Image Factory (https://factory.talos.dev)",
	)

	// Wait for the resource to appear.
	ctest.AssertResource(suite, runtime.ImageFactorySchematicID, func(res *runtime.ImageFactorySchematic, asrt *assert.Assertions) {
		asrt.Equal(schematicID, res.TypedSpec().SchematicID)
	})

	// Delete the schematic extension, resource must be cleaned up.
	ext := runtime.NewExtensionStatus(runtime.NamespaceName, "schematic.sqsh")
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), ext.Metadata()))

	ctest.AssertNoResource[*runtime.ImageFactorySchematic](suite, runtime.ImageFactorySchematicID)
}

func (suite *ImageFactorySchematicSuite) TestCustomImageFactory() {
	// Self-hosted Image Factory with a custom URL.
	const schematicID = "cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f"

	suite.createExtensionStatus(
		"schematic.sqsh",
		"schematic",
		schematicID,
		"My Factory (https://factory.example.com)",
	)

	ctest.AssertResource(suite, runtime.ImageFactorySchematicID, func(res *runtime.ImageFactorySchematic, asrt *assert.Assertions) {
		asrt.Equal(schematicID, res.TypedSpec().SchematicID)
		asrt.Equal("My Factory", res.TypedSpec().Flavor)
		asrt.Equal("https://factory.example.com", res.TypedSpec().APIURL)
	})
}

func (suite *ImageFactorySchematicSuite) TestAuthorWithoutURL() {
	// Older or minimal extension with no URL in author field.
	suite.createExtensionStatus("schematic.sqsh", "schematic", "abc123", "Just A Name")

	ctest.AssertResource(suite, runtime.ImageFactorySchematicID, func(res *runtime.ImageFactorySchematic, asrt *assert.Assertions) {
		asrt.Equal("abc123", res.TypedSpec().SchematicID)
		asrt.Equal("Just A Name", res.TypedSpec().Flavor)
		asrt.Empty(res.TypedSpec().APIURL)
	})
}
