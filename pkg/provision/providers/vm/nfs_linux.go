// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux && (amd64 || arm64 || riscv64)

package vm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	dittonfs "github.com/marmos91/dittofs/pkg/adapter/nfs"
	"github.com/marmos91/dittofs/pkg/controlplane/models"
	dittoruntime "github.com/marmos91/dittofs/pkg/controlplane/runtime"
	dittoshares "github.com/marmos91/dittofs/pkg/controlplane/runtime/shares"
	controlplanestore "github.com/marmos91/dittofs/pkg/controlplane/store"
	metadatamemory "github.com/marmos91/dittofs/pkg/metadata/store/memory"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	nfsPIDFile     = "nfs.pid"
	nfsLogFile     = "nfs.log"
	NFSExport      = "/export"
	nfsMemoryStore = "memory"
	nfsAdapterType = "nfs"
)

// NFSd serves an in-memory NFS export supporting NFSv3 and NFSv4.
func NFSd(ctx context.Context, bindAddress string, port int) error {
	// Every share journals its blocks to disk, so the export needs a journal root even though
	// its metadata store is in-memory. The journal is as ephemeral as that metadata: it is
	// meaningless without it, so it lives in a temporary directory removed when the server exits.
	journalRoot, err := os.MkdirTemp("", "talos-nfsd-")
	if err != nil {
		return fmt.Errorf("create NFS journal root: %w", err)
	}

	defer os.RemoveAll(journalRoot) //nolint:errcheck

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

	if err = disableNFSDelegations(ctx, store, port); err != nil {
		return err
	}

	metadataStoreID, err := store.CreateMetadataStore(ctx, &models.MetadataStoreConfig{
		Name: nfsMemoryStore,
		Type: nfsMemoryStore,
	})
	if err != nil {
		return fmt.Errorf("create NFS metadata store: %w", err)
	}

	// The block store a share points at is its backing tier behind the journal: in-memory here,
	// so nothing but the journal touches the disk.
	blockStoreID, err := store.CreateBlockStore(ctx, &models.BlockStoreConfig{
		Name: nfsMemoryStore,
		Type: nfsMemoryStore,
	})
	if err != nil {
		return fmt.Errorf("create NFS block store: %w", err)
	}

	if _, err = store.CreateShare(ctx, &models.Share{
		Name:              NFSExport,
		MetadataStoreID:   metadataStoreID,
		BlockStoreID:      blockStoreID,
		DefaultPermission: "read-write",
		Enabled:           true,
	}); err != nil {
		return fmt.Errorf("create NFS share: %w", err)
	}

	runtime := dittoruntime.New(store)
	// Must be set before the share is added: AddShare opens the share's journal beneath this root.
	runtime.SetLocalStoreDefaults(&dittoshares.LocalStoreDefaults{
		JournalRoot: journalRoot,
	})

	if err = runtime.RegisterMetadataStore(nfsMemoryStore, metadatamemory.NewMemoryMetadataStoreWithDefaults()); err != nil {
		return fmt.Errorf("register NFS metadata store: %w", err)
	}

	if err = runtime.AddShare(ctx, &dittoruntime.ShareConfig{
		Name:              NFSExport,
		MetadataStore:     nfsMemoryStore,
		BlockStoreID:      blockStoreID,
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

// disableNFSDelegations stores NFS adapter settings with delegations disabled.
//
// The server only recalls NFSv4 delegations on SMB access, never on NFSv3 access, so with
// delegations enabled an NFSv3 client reads stale data for a file an NFSv4 client holds a write
// delegation on: the NFSv4 client doesn't flush its writes on close under such a delegation.
// The settings are picked up when the server is attached to the runtime.
func disableNFSDelegations(ctx context.Context, store controlplanestore.Store, port int) error {
	adapterID, err := store.CreateAdapter(ctx, &models.AdapterConfig{
		Type:    nfsAdapterType,
		Enabled: true,
		Port:    port,
	})
	if err != nil {
		return fmt.Errorf("create NFS adapter: %w", err)
	}

	if err = store.EnsureAdapterSettings(ctx); err != nil {
		return fmt.Errorf("create NFS adapter settings: %w", err)
	}

	settings, err := store.GetNFSAdapterSettings(ctx, adapterID)
	if err != nil {
		return fmt.Errorf("get NFS adapter settings: %w", err)
	}

	settings.DelegationsEnabled = false

	if err = store.UpdateNFSAdapterSettings(ctx, settings); err != nil {
		return fmt.Errorf("update NFS adapter settings: %w", err)
	}

	return nil
}

func (p *Provisioner) startNFSd(state *provision.State, clusterReq provision.ClusterRequest) error {
	if len(clusterReq.Network.GatewayAddrs) == 0 {
		return errors.New("NFS server requires a bridge gateway address")
	}

	gateway := clusterReq.Network.GatewayAddrs[0]

	logFile, err := os.OpenFile(state.GetRelativePath(nfsLogFile), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}
	defer logFile.Close() //nolint:errcheck

	cmd := exec.Command( //nolint:noctx // runs in background
		clusterReq.SelfExecutable,
		"nfs-launch",
		"--bind-address", gateway.String(),
		"--port", "2049",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	setDetachedProcess(cmd)

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(state.GetRelativePath(nfsPIDFile), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		return fmt.Errorf("error writing NFS PID file: %w", err)
	}

	return nil
}

// DestroyNFS stops the development NFS server.
func (p *Provisioner) stopNFSd(state *provision.State) error {
	return StopProcessByPidfile(state.GetRelativePath(nfsPIDFile))
}
