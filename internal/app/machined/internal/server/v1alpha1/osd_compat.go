// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/os"
)

type osdServer struct {
	*Server
}

// Dmesg implements the osapi.OsServer interface.
func (s *osdServer) Dmesg(req *machine.DmesgRequest, srv os.OSService_DmesgServer) error {
	return s.Server.Dmesg(req, machine.MachineService_DmesgServer(srv))
}
