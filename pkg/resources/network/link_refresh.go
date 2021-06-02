// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
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
type LinkRefresh struct {
	md   resource.Metadata
	spec LinkRefreshSpec
}

// LinkRefreshSpec describes status of rendered secrets.
type LinkRefreshSpec struct {
	Generation int `yaml:"generation"`
}

// NewLinkRefresh initializes a LinkRefresh resource.
func NewLinkRefresh(namespace resource.Namespace, id resource.ID) *LinkRefresh {
	r := &LinkRefresh{
		md:   resource.NewMetadata(namespace, LinkRefreshType, id, resource.VersionUndefined),
		spec: LinkRefreshSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *LinkRefresh) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *LinkRefresh) Spec() interface{} {
	return r.spec
}

func (r *LinkRefresh) String() string {
	return fmt.Sprintf("network.LinkRefresh(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *LinkRefresh) DeepCopy() resource.Resource {
	return &LinkRefresh{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *LinkRefresh) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkRefreshType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// Bump performs an update.
func (r *LinkRefresh) Bump() {
	r.spec.Generation++
}
