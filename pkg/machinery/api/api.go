// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package api contains API definitions for Talos Linux.
//
//nolint:revive
package api

import (
	cosi "github.com/cosi-project/runtime/api/v1alpha1"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/siderolabs/talos/pkg/machinery/api/cluster"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/inspect"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/security"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/api/time"
)

// TalosAPIdOne2ManyAPIs returns a list of API services that support one-to-many
// communication pattern served by apid.
//
// Note: we are moving to one-to-one APIs, so this list should not grow.
func TalosAPIdOne2ManyAPIs() []protoreflect.FileDescriptor {
	return []protoreflect.FileDescriptor{
		common.File_common_common_proto,
		cluster.File_cluster_cluster_proto,
		inspect.File_inspect_inspect_proto,
		machine.File_machine_machine_proto,
		storage.File_storage_storage_proto,
		time.File_time_time_proto,
	}
}

// TalosAPIdAllAPIs returns a list of all API services served by apid.
//
// This includes legacy one-to-many APIs as well as newer one-to-one APIs.
func TalosAPIdAllAPIs() []protoreflect.FileDescriptor {
	return append(TalosAPIdOne2ManyAPIs(),
		cosi.File_v1alpha1_state_proto,
		machine.File_machine_image_proto,
	)
}

// AllAPIs returns a list of all API services served by Talos components.
//
// This includes Talos apid and trustd APIs.
func AllAPIs() []protoreflect.FileDescriptor {
	return append(TalosAPIdAllAPIs(),
		security.File_security_security_proto,
	)
}
