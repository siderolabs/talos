// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

func (p *Provisioner) stopDNSd(_ *State) error {
	return nil
}

// StartDNSd on darwin is a no-op since DNSd is not used.
func (p *Provisioner) StartDNSd(_ *State) error {
	return nil
}
