// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storagehelpers provides validation shared by the documents which name a storage pool.
//
// It exists so that the document declaring a pool and the documents referencing one agree on what a
// pool may be called. They are in different packages, and a reference which the declaring document
// would accept but the referencing one would not is a pool nothing can ever use.
package storagehelpers
