// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package contentlibrary_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// Legacy consumers still attach library paths directly before the copy migration.
// Object deletion always refuses live aliases, even before a consumer takes its hold.
func TestDeleteProtectsAttachedLegacySources(t *testing.T) {
	t.Parallel()

	for _, consumer := range []string{"disk", "seed"} {
		for _, target := range []string{"alias", "object"} {
			for _, tc := range []struct {
				name      string
				library   string
				file      string
				held      bool
				aliasCode codes.Code
			}{
				{name: "attached", library: testLibrary, file: "image.raw", held: true, aliasCode: codes.FailedPrecondition},
				{name: "same object through another alias", library: testLibrary, file: "other.raw", held: true},
				{name: "detached", library: testLibrary, file: "image.raw"},
				{name: "other library", library: "unrelated", file: "image.raw", held: true},
				{name: "other object", library: testLibrary, file: "unrelated.raw", held: true},
			} {
				t.Run(consumer+"/"+target+"/"+tc.name, func(t *testing.T) {
					t.Parallel()

					svc, path, st := setupWithState(t, true)
					require.NoError(t, upload(t, svc, "image.raw", "payload"))
					require.NoError(t, upload(t, svc, "other.raw", "payload"))
					require.NoError(t, upload(t, svc, "unrelated.raw", "unrelated"))

					var attached resource.Resource

					if consumer == "disk" {
						disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "vm/install@aaaaaaaaaaaa")
						disk.TypedSpec().VirtualMachine = "vm"
						disk.TypedSpec().Name = "install"
						disk.TypedSpec().Image = hypervisor.VirtualMachineDiskFromImageSpec{Library: tc.library, File: tc.file}
						attached = disk
					} else {
						seed := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "vm@seed")
						seed.TypedSpec().VirtualMachine = "vm"
						seed.TypedSpec().Library = tc.library
						seed.TypedSpec().Name = tc.file
						attached = seed
					}

					require.NoError(t, st.Create(t.Context(), attached))

					if tc.held {
						require.NoError(t, st.AddFinalizer(t.Context(), attached.Metadata(), "hypervisor.VirtualMachineController"))
					}

					req := &machine.ContentLibraryServiceDeleteRequest{
						LibraryId: testLibrary,
						Target:    &machine.ContentLibraryServiceDeleteRequest_Name{Name: "image.raw"},
					}
					want := tc.aliasCode
					hash := digest.SHA256.FromString("payload")

					if target == "object" {
						req.Target = &machine.ContentLibraryServiceDeleteRequest_Sha256{Sha256: hash.String()}
						want = codes.FailedPrecondition
					}

					_, err := svc.Delete(t.Context(), req)
					require.Equal(t, want, grpcstatus.Code(err), "%v", err)

					if want == codes.FailedPrecondition {
						for _, name := range []string{"image.raw", "other.raw", filepath.Join("objects", "sha256", hash.Encoded())} {
							contents, readErr := os.ReadFile(filepath.Join(path, name))
							require.NoError(t, readErr)
							assert.Equal(t, "payload", string(contents))
						}
					} else {
						require.NoFileExists(t, filepath.Join(path, "image.raw"))
					}

					assert.True(t, libraryFinalizers(t, st).Empty(), "deletion releases its mutation marker")
				})
			}
		}
	}
}
