// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// requireContentLibrarySupport is requireContentLibrary for the cases kept out of the short pipeline.
func (suite *LibvirtSuite) requireContentLibrarySupport() {
	suite.T().Helper()

	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	suite.requireContentLibrary()
}

// requireContentLibrary skips unless the cluster can hold the library these tests source their
// image from. LibvirtSuite itself only requires the hypervisor.
func (suite *LibvirtSuite) requireContentLibrary() {
	suite.T().Helper()

	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}
}

// digestOfISO hashes the ISO in one streaming pass, so a large image never lands in memory.
func (suite *LibvirtSuite) digestOfISO() digest.Digest {
	suite.T().Helper()

	f, err := os.Open(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	defer f.Close() //nolint:errcheck

	digester := digest.SHA256.Digester()

	_, err = io.Copy(digester.Hash(), f)
	suite.Require().NoError(err)

	return digester.Digest()
}

// provisionContentLibraryPool creates a distinct pool directory on the library's filesystem volume.
// The pool owns its own mount request and files, independently of the source library resource.
func (suite *LibvirtSuite) provisionContentLibraryPool(ctx context.Context, node, library string) (string, string) {
	suite.T().Helper()
	suite.AssertServicesRunning(ctx, node, map[string]string{"ext-virtstoraged": "Running"})

	pool := storagecfg.NewStoragePoolConfigV1Alpha1()
	pool.MetaName = "copy-" + uuid.NewString()
	pool.VolumeConfig.VolumeName = library

	var target string

	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		nodeCtx := client.WithNode(cleanupCtx, node)
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.StoragePoolConfigKind, pool.MetaName)
		rtestutils.AssertNoResource[*storage.StoragePoolStatus](nodeCtx, suite.T(), suite.Client.State(), pool.MetaName)

		if target != "" {
			output, code := suite.RunDebugContainer(cleanupCtx, node, "/nix/var/nix/profiles/default/bin/busybox", "rm", "-rf", "--", target)
			suite.Assert().Zero(code, "remove test-created pool directory: %s", output)
		}
	})
	suite.PatchMachineConfig(ctx, pool)
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), pool.MetaName,
		func(status *storage.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.Equal(storage.StoragePoolPhaseReady, status.TypedSpec().Phase, status.TypedSpec().Error)

			if !asrt.Equal(pool.MetaName, filepath.Base(status.TypedSpec().TargetPath)) || !asrt.True(filepath.IsAbs(status.TypedSpec().TargetPath)) {
				return
			}

			target = filepath.Clean(status.TypedSpec().TargetPath)
		})

	return pool.MetaName, target
}

// diskStatusID is the ID the disk status of one configured disk is published under.
//
// The ID covers what the disk is provisioned from as well as its name, so it is built from the same
// document the test applied rather than written out by hand.
func diskStatusID(vm string, disk hypervisorcfg.VirtualMachineDisk) string {
	image := disk.ProvisionConfig.FromImageConfig

	return hypervisor.VirtualMachineDiskStatusID(vm, hypervisor.VirtualMachineDiskSpec{
		Name:   disk.Name(),
		Pool:   disk.Pool(),
		Format: disk.Format().String(),
		Type:   disk.Type().String(),
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{
				Library: image.ImageLibrary,
				File:    image.ImageFile,
				Digest:  image.ImageDigest,
			},
		},
	})
}

// provisionContentLibrary declares a content library on a directory user volume of node and waits
// for it to come up.
//
// It returns the library's ID and the path its contents sit at. The library and the volume backing
// it are removed when the test ends.
func provisionContentLibrary(suite *base.APISuite, ctx context.Context, node string) (string, string) { //nolint:revive
	suite.T().Helper()

	// Randomized so repeated runs against the same cluster don't collide on a leftover volume.
	name := fmt.Sprintf("cl-%04x", rand.Int31())
	volumeID := constants.UserVolumePrefix + name

	suite.T().Logf("testing the content library %q on node %s", name, node)

	// A directory volume, not a partition: none of these tests store more than a few kilobytes, and a
	// partition would claim a spare disk which the storage tests need, and which removing the document
	// does not hand back.
	volumeDoc := blockcfg.NewUserVolumeConfigV1Alpha1()
	volumeDoc.MetaName = name
	volumeDoc.VolumeType = new(block.VolumeTypeDirectory)

	libraryDoc := hypervisorcfg.NewContentLibraryConfigV1Alpha1()
	libraryDoc.MetaName = name
	libraryDoc.BackingConfig.VolumeName = name

	suite.PatchMachineConfig(ctx, volumeDoc, libraryDoc)

	suite.T().Cleanup(func() {
		// A context of its own: cleanups run after TearDownTest, which has already canceled the
		// suite's context by then.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		nodeCtx := client.WithNode(cleanupCtx, node)

		suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.ContentLibraryConfigKind, name)
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, name)

		rtestutils.AssertNoResource[*hypervisor.ContentLibraryStatus](nodeCtx, suite.T(), suite.Client.State(), name)
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.State(), []string{volumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.State(), []string{name},
		func(cls *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
			asrt.True(cls.TypedSpec().Phase == hypervisor.ContentLibraryPhaseReady, "error: %q", cls.TypedSpec().Error)
		},
	)

	return name, filepath.Join(constants.UserVolumeMountPoint, name)
}

func contentLibraryObjectPath(path string, identity digest.Digest) string {
	return filepath.Join(path, "objects", "sha256", identity.Encoded())
}
