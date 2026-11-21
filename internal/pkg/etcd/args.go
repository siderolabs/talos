// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// Values of the etcd `--initial-cluster-state` flag.
const (
	InitialClusterStateNew      = "new"
	InitialClusterStateExisting = "existing"
)

// Names of the etcd flags which can be overridden with the extra arguments, changing the way the etcd member is launched.
const (
	ArgInitialCluster           = "initial-cluster"
	ArgInitialClusterState      = "initial-cluster-state"
	ArgInitialAdvertisePeerURLs = "initial-advertise-peer-urls"
	ArgAdvertiseClientURLs      = "advertise-client-urls"
)

// FormatURLs formats the addresses as the list of etcd URLs with the given port.
func FormatURLs(addrs []netip.Addr, port int) []string {
	return xslices.Map(addrs, func(addr netip.Addr) string {
		return fmt.Sprintf("https://%s", nethelpers.JoinHostPort(addr.String(), port))
	})
}

// FormatInitialCluster formats the value of the `--initial-cluster` flag for a single member with the given peer URLs.
func FormatInitialCluster(name string, peerURLs []string) string {
	return strings.Join(xslices.Map(peerURLs, func(url string) string {
		return fmt.Sprintf("%s=%s", name, url)
	}), ",")
}

// HasExtraArg checks if the etcd spec overrides the given argument via the extra arguments.
func HasExtraArg(spec *etcdresource.SpecSpec, arg string) bool {
	_, ok := spec.ExtraArgs[arg]

	return ok
}

// BuildArgs builds the full list of the etcd command line arguments.
//
// The spec describes the member (name, addresses, extra arguments), while the launch spec
// describes how the member enters the cluster.
func BuildArgs(spec *etcdresource.SpecSpec, launch *etcdresource.LaunchSpecSpec) ([]string, error) {
	joinURLs := func(addrs []netip.Addr, port int) string {
		return strings.Join(FormatURLs(addrs, port), ",")
	}

	denyListArgs := argsbuilder.Args{
		"name":                           {spec.Name},
		"auto-tls":                       {"false"},
		"peer-auto-tls":                  {"false"},
		"data-dir":                       {constants.EtcdDataPath},
		"listen-peer-urls":               {joinURLs(spec.ListenPeerAddresses, constants.EtcdPeerPort)},
		"listen-client-urls":             {joinURLs(spec.ListenClientAddresses, constants.EtcdClientPort)},
		"listen-client-http-urls":        {joinURLs(spec.ListenClientAddresses, constants.EtcdClientHTTPPort)},
		"client-cert-auth":               {"true"},
		"cert-file":                      {constants.EtcdCert},
		"key-file":                       {constants.EtcdKey},
		"trusted-ca-file":                {constants.EtcdCACert},
		"peer-client-cert-auth":          {"true"},
		"peer-cert-file":                 {constants.EtcdPeerCert},
		"peer-key-file":                  {constants.EtcdPeerKey},
		"peer-trusted-ca-file":           {constants.EtcdCACert},
		"feature-gates":                  {"InitialCorruptCheck=true", "CompactHashCheck=true"},
		"watch-progress-notify-interval": {"5s"},
		"tls-min-version":                {"TLS1.3"},
	}

	// the peer URLs should match the initial cluster, so they are fixed at the moment the initial cluster is built
	initialAdvertisePeerURLs := strings.Join(launch.InitialAdvertisePeerURLs, ",")
	if initialAdvertisePeerURLs == "" {
		initialAdvertisePeerURLs = joinURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort)
	}

	extraArgs := make(argsbuilder.Args, len(spec.ExtraArgs))
	for k, v := range spec.ExtraArgs {
		extraArgs[k] = v.Values
	}

	// the arguments below can be overridden with the extra arguments, so they only get into the deny list
	// if they are not overridden
	for _, arg := range []struct {
		key   string
		value string
	}{
		{ArgInitialClusterState, launch.InitialClusterState},
		{ArgInitialCluster, launch.InitialCluster},
		{ArgInitialAdvertisePeerURLs, initialAdvertisePeerURLs},
		{ArgAdvertiseClientURLs, joinURLs(spec.AdvertisedAddresses, constants.EtcdClientPort)},
	} {
		if arg.value == "" || extraArgs.Contains(arg.key) {
			continue
		}

		denyListArgs.Set(arg.key, argsbuilder.Value{arg.value})
	}

	if err := denyListArgs.Merge(extraArgs, argsbuilder.WithDenyList(denyListArgs)); err != nil {
		return nil, err
	}

	return denyListArgs.Args(), nil
}

// RestartArgs returns the etcd arguments which require etcd to be restarted when they change.
//
// The advertised URLs follow the addresses of the machine, and the change of the addresses is not a reason
// to restart etcd: the peer URLs of a running member are updated in the etcd cluster via the API,
// and the flags are picked up on the next start of etcd. The advertised URLs set via the extra arguments
// are kept, as they change only if the machine configuration changes.
func RestartArgs(spec *etcdresource.SpecSpec, args []string) []string {
	var ignored []string

	for _, arg := range []string{ArgInitialAdvertisePeerURLs, ArgAdvertiseClientURLs} {
		if !HasExtraArg(spec, arg) {
			ignored = append(ignored, "--"+arg+"=")
		}
	}

	return xslices.Filter(args, func(arg string) bool {
		return !slices.ContainsFunc(ignored, func(prefix string) bool {
			return strings.HasPrefix(arg, prefix)
		})
	})
}
