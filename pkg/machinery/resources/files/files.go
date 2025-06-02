// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package files provides resources which describe files on disk.
package files

import "github.com/cosi-project/runtime/pkg/resource"

// NamespaceName contains file resources.
const NamespaceName resource.Namespace = "files"

// SourceFileAnnotation is used to annotate a file resource with the source file path(s).
const SourceFileAnnotation = "talos.dev/source-file"
