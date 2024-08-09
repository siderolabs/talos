// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/protoenc"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ResourceSpecToProto converts a resource spec to a proto message.
func ResourceSpecToProto(i resource.Resource, o proto.Message) error {
	marshaled, err := protoenc.Marshal(i.Spec())
	if err != nil {
		return err
	}

	return proto.Unmarshal(marshaled, o)
}
