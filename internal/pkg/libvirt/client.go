// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package libvirt composes the host's modular libvirt clients.
package libvirt

import (
	"context"

	"github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage"
)

// The host-local modular daemon endpoints. These are deliberately not the
// go-libvirt library's default libvirtd connection.
const (
	storageSocket = "/run/libvirt/virtstoraged-sock"
	storageURI    = "storage:///system"
	domainSocket  = "/run/libvirt/virtqemud-sock"
	domainURI     = "qemu:///system"
)

// Config identifies the independent libvirt storage and QEMU daemons.
type Config struct {
	StorageSocket string
	StorageURI    string
	DomainSocket  string
	DomainURI     string
}

// Client constructs daemon-specific, independently bounded libvirt sessions.
// The returned clients own their connections; callers must close each one.
type Client struct {
	storage *storage.Connector
	domain  *domain.Connector
}

// New uses the host-local modular daemon endpoints.
func New() *Client {
	return NewWithConfig(Config{
		StorageSocket: storageSocket,
		StorageURI:    storageURI,
		DomainSocket:  domainSocket,
		DomainURI:     domainURI,
	})
}

// NewWithConfig constructs independent connectors for the configured daemons.
// No connection is made until Storage or Domain is called.
func NewWithConfig(config Config) *Client {
	return &Client{
		storage: storage.New(config.StorageSocket, config.StorageURI),
		domain:  domain.New(config.DomainSocket, config.DomainURI),
	}
}

// Storage connects to the modular storage daemon with the caller's context.
func (c *Client) Storage(ctx context.Context) (storage.Client, error) {
	return c.storage.Open(ctx)
}

// Domain connects to the modular QEMU daemon with the caller's context.
func (c *Client) Domain(ctx context.Context) (domain.Client, error) {
	return c.domain.Open(ctx)
}
