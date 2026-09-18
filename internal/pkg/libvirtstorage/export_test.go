// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package libvirtstorage

var OpenConn = openConn

// NewTestClient exposes only the RPC seam to external-package tests.
func NewTestClient(rpc poolRPC) Client { return &client{rpc: rpc} }
