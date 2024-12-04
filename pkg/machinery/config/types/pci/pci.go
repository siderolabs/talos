// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pci provides PCI rebind configuration.
package pci

//go:generate docgen -output pci_doc.go pci.go rebind.go

//go:generate deep-copy -type RebindConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
