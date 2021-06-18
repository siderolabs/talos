// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lint_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/talos-systems/talos/pkg/machinery/api/cluster"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/inspect"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/api/resource"
	"github.com/talos-systems/talos/pkg/machinery/api/security"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/api/time"
)

// TODO https://github.com/talos-systems/talos/issues/3760
// Check messages, hook into build.

func TestProto(t *testing.T) {
	var protoreflectMethods, grpcServiceDescMethods []string

	for _, services := range []protoreflect.ServiceDescriptors{
		common.File_common_common_proto.Services(),
		cluster.File_cluster_cluster_proto.Services(),
		inspect.File_inspect_inspect_proto.Services(),
		machine.File_machine_machine_proto.Services(),
		network.File_network_network_proto.Services(),
		resource.File_resource_resource_proto.Services(),
		security.File_security_security_proto.Services(),
		storage.File_storage_storage_proto.Services(),
		time.File_time_time_proto.Services(),
	} {
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			methods := service.Methods()

			for j := 0; j < methods.Len(); j++ {
				s := fmt.Sprintf("/%s/%s", service.FullName(), methods.Get(j).Name())
				protoreflectMethods = append(protoreflectMethods, s)
			}
		}
	}

	for _, service := range []grpc.ServiceDesc{
		// no common
		cluster.ClusterService_ServiceDesc,
		inspect.InspectService_ServiceDesc,
		machine.MachineService_ServiceDesc,
		network.NetworkService_ServiceDesc,
		resource.ResourceService_ServiceDesc,
		security.SecurityService_ServiceDesc,
		storage.StorageService_ServiceDesc,
		time.TimeService_ServiceDesc,
	} {
		for _, method := range service.Methods {
			s := fmt.Sprintf("/%s/%s", service.ServiceName, method.MethodName)
			grpcServiceDescMethods = append(grpcServiceDescMethods, s)
		}

		for _, stream := range service.Streams {
			s := fmt.Sprintf("/%s/%s", service.ServiceName, stream.StreamName)
			grpcServiceDescMethods = append(grpcServiceDescMethods, s)
		}
	}

	sort.Strings(protoreflectMethods)
	sort.Strings(grpcServiceDescMethods)

	for _, s := range protoreflectMethods {
		t.Log(s)
	}

	require.Equal(t, protoreflectMethods, grpcServiceDescMethods)
}
