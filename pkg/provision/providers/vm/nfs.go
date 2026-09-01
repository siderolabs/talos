// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"fmt"

	dittonfs "github.com/marmos91/dittofs/pkg/adapter/nfs"
	"github.com/marmos91/dittofs/pkg/controlplane/models"
	dittoruntime "github.com/marmos91/dittofs/pkg/controlplane/runtime"
	controlplanestore "github.com/marmos91/dittofs/pkg/controlplane/store"
	metadatamemory "github.com/marmos91/dittofs/pkg/metadata/store/memory"
)

const (
	// NFSExport is the export path served by the development NFS server.
	NFSExport      = "/export"
	nfsPortBase    = 20000
	nfsPortSpan    = 20000
	nfsMemoryStore = "memory"
)

// NFSd serves an in-memory NFS export supporting NFSv3 and NFSv4.
func NFSd(ctx context.Context, bindAddress string, port int) error {
	store, err := controlplanestore.New(&controlplanestore.Config{
		Type: controlplanestore.DatabaseTypeSQLite,
		SQLite: controlplanestore.SQLiteConfig{
			Path: ":memory:",
		},
	})
	if err != nil {
		return fmt.Errorf("create NFS control plane store: %w", err)
	}
	defer store.Close() //nolint:errcheck

	metadataStoreID, err := store.CreateMetadataStore(ctx, &models.MetadataStoreConfig{
		Name: nfsMemoryStore,
		Type: nfsMemoryStore,
	})
	if err != nil {
		return fmt.Errorf("create NFS metadata store: %w", err)
	}

	localBlockStoreID, err := store.CreateBlockStore(ctx, &models.BlockStoreConfig{
		Name: nfsMemoryStore,
		Kind: models.BlockStoreKindLocal,
		Type: nfsMemoryStore,
	})
	if err != nil {
		return fmt.Errorf("create NFS block store: %w", err)
	}

	if _, err = store.CreateShare(ctx, &models.Share{
		Name:              NFSExport,
		MetadataStoreID:   metadataStoreID,
		LocalBlockStoreID: localBlockStoreID,
		DefaultPermission: "read-write",
		Enabled:           true,
	}); err != nil {
		return fmt.Errorf("create NFS share: %w", err)
	}

	runtime := dittoruntime.New(store)
	if err = runtime.RegisterMetadataStore(nfsMemoryStore, metadatamemory.NewMemoryMetadataStoreWithDefaults()); err != nil {
		return fmt.Errorf("register NFS metadata store: %w", err)
	}

	if err = runtime.AddShare(ctx, &dittoruntime.ShareConfig{
		Name:              NFSExport,
		MetadataStore:     nfsMemoryStore,
		LocalBlockStoreID: localBlockStoreID,
		DefaultPermission: "read-write",
		Enabled:           true,
		// An unset squash mode normalizes to root_to_guest, which maps the client's root to
		// anonymous and leaves the export root (owned by uid 0) unwritable. This server exists to
		// be written to by test workloads, so export it the way a test export is: no_root_squash.
		Squash:          models.SquashNone,
		AllowAuthSys:    true,
		AllowAuthSysSet: true,
	}); err != nil {
		return fmt.Errorf("add NFS export: %w", err)
	}
	defer runtime.RemoveShare(NFSExport) //nolint:errcheck

	server := dittonfs.New(dittonfs.NFSConfig{
		Enabled:        true,
		Port:           port,
		MaxConnections: 64,
	})
	server.Config.BindAddress = bindAddress
	server.SetRuntime(runtime)

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("serve NFS: %w", err)
	}

	return nil
}
