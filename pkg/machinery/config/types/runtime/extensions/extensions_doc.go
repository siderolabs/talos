// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Code generated by hack/docgen tool. DO NOT EDIT.

package extensions

import (
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

func (ServiceConfigV1Alpha1) Doc() *encoder.Doc {
	doc := &encoder.Doc{
		Type:        "ExtensionServiceConfig",
		Comments:    [3]string{"" /* encoder.HeadComment */, "ExtensionServiceConfig is a extensionserviceconfig document." /* encoder.LineComment */, "" /* encoder.FootComment */},
		Description: "ExtensionServiceConfig is a extensionserviceconfig document.",
		Fields: []encoder.Doc{
			{},
			{
				Name:        "name",
				Type:        "string",
				Note:        "",
				Description: "Name of the extension service.",
				Comments:    [3]string{"" /* encoder.HeadComment */, "Name of the extension service." /* encoder.LineComment */, "" /* encoder.FootComment */},
			},
			{
				Name:        "configFiles",
				Type:        "[]ConfigFile",
				Note:        "",
				Description: "The config files for the extension service.",
				Comments:    [3]string{"" /* encoder.HeadComment */, "The config files for the extension service." /* encoder.LineComment */, "" /* encoder.FootComment */},
			},
			{
				Name:        "environment",
				Type:        "[]string",
				Note:        "",
				Description: "The environment for the extension service.",
				Comments:    [3]string{"" /* encoder.HeadComment */, "The environment for the extension service." /* encoder.LineComment */, "" /* encoder.FootComment */},
			},
		},
	}

	doc.AddExample("", extensionServiceConfigV1Alpha1())

	return doc
}

func (ConfigFile) Doc() *encoder.Doc {
	doc := &encoder.Doc{
		Type:        "ConfigFile",
		Comments:    [3]string{"" /* encoder.HeadComment */, "ConfigFile is a config file for extension services." /* encoder.LineComment */, "" /* encoder.FootComment */},
		Description: "ConfigFile is a config file for extension services.",
		AppearsIn: []encoder.Appearance{
			{
				TypeName:  "ServiceConfigV1Alpha1",
				FieldName: "configFiles",
			},
		},
		Fields: []encoder.Doc{
			{
				Name:        "content",
				Type:        "string",
				Note:        "",
				Description: "The content of the extension service config file.",
				Comments:    [3]string{"" /* encoder.HeadComment */, "The content of the extension service config file." /* encoder.LineComment */, "" /* encoder.FootComment */},
			},
			{
				Name:        "mountPath",
				Type:        "string",
				Note:        "",
				Description: "The mount path of the extension service config file.",
				Comments:    [3]string{"" /* encoder.HeadComment */, "The mount path of the extension service config file." /* encoder.LineComment */, "" /* encoder.FootComment */},
			},
		},
	}

	return doc
}

// GetFileDoc returns documentation for the file extensions_doc.go.
func GetFileDoc() *encoder.FileDoc {
	return &encoder.FileDoc{
		Name:        "extensions",
		Description: "Package extensions provides extensions config documents.\n",
		Structs: []*encoder.Doc{
			ServiceConfigV1Alpha1{}.Doc(),
			ConfigFile{}.Doc(),
		},
	}
}
