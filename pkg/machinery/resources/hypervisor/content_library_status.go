// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ContentLibraryStatusType is type of ContentLibraryStatus resource.
const ContentLibraryStatusType = resource.Type("ContentLibraryStatuses.hypervisor.talos.dev")

// ContentLibraryStatus resource holds the state of a content library declared via ContentLibraryConfig.
//
// The ID is the content library name.
type ContentLibraryStatus = typed.Resource[ContentLibraryStatusSpec, ContentLibraryStatusExtension]

// ContentLibraryStatusSpec is the spec for ContentLibraryStatus.
//
//gotagsrewrite:gen
type ContentLibraryStatusSpec struct {
	// VolumeID is the ID of the volume backing the library.
	VolumeID string `yaml:"volumeID" protobuf:"1"`
	// Path is the absolute path of the library's contents, the target the backing volume is mounted at.
	//
	// Only meaningful when Ready.
	Path string `yaml:"path,omitempty" protobuf:"2"`
	// Ready is true once the backing volume is mounted.
	Ready bool `yaml:"ready" protobuf:"3"`
	// Error describes why the library is not ready.
	Error string `yaml:"error,omitempty" protobuf:"4"`
}

// NewContentLibraryStatus initializes a ContentLibraryStatus resource.
func NewContentLibraryStatus(namespace resource.Namespace, id resource.ID) *ContentLibraryStatus {
	return typed.NewResource[ContentLibraryStatusSpec, ContentLibraryStatusExtension](
		resource.NewMetadata(namespace, ContentLibraryStatusType, id, resource.VersionUndefined),
		ContentLibraryStatusSpec{},
	)
}

// ContentLibraryStatusExtension is auxiliary resource data for ContentLibraryStatus.
type ContentLibraryStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContentLibraryStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContentLibraryStatusType,
		Aliases:          []resource.Type{"contentlibrary", "contentlibraries", "contentlibrarystatus", "contentlibrarystatuses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Volume",
				JSONPath: `{.volumeID}`,
			},
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
			{
				Name:     "Path",
				JSONPath: `{.path}`,
			},
			{
				Name:     "Error",
				JSONPath: `{.error}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ContentLibraryStatusType, &ContentLibraryStatus{})
	if err != nil {
		panic(err)
	}
}
