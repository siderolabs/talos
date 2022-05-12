// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// LinkRefreshType is type of LinkRefresh resource.
const LinkRefreshType = resource.Type("LinkRefreshes.net.talos.dev")

// LinkRefresh resource is used to communicate link changes which can't be subscribed to via netlink.
//
// The only usecase for now is the Wireguards, as there's no way subscribe to wireguard updates
// via the netlink API.
//
// Whenever Wireguard interface is updated, LinkRefresh resource is modified to trigger a reconcile
// loop in the LinkStatusController.
type LinkRefresh = typed.Resource[LinkRefreshSpec, LinkRefreshRD]

// LinkRefreshSpec describes status of rendered secrets.
type LinkRefreshSpec struct {
	Generation int `yaml:"generation"`
}

// Bump performs an update.
func (s *LinkRefreshSpec) Bump() {
	s.Generation++
}

// NewLinkRefresh initializes a LinkRefresh resource.
func NewLinkRefresh(namespace resource.Namespace, id resource.ID) *LinkRefresh {
	return typed.NewResource[LinkRefreshSpec, LinkRefreshRD](
		resource.NewMetadata(namespace, LinkRefreshType, id, resource.VersionUndefined),
		LinkRefreshSpec{},
	)
}

// LinkRefreshRD provides auxiliary methods for LinkRefresh.
type LinkRefreshRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (LinkRefreshRD) ResourceDefinition(resource.Metadata, LinkRefreshSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkRefreshType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
