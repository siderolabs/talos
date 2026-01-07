// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

import (
	"strconv"

	"github.com/containerd/go-cni"
)

const (
	// PATH defines all locations where executables are stored.
	PATH = "/usr/bin:/usr/local/sbin:/usr/local/bin:" + cni.DefaultCNIDir

	// EnvPath is the environment variable to set PATH.
	EnvPath = "PATH=" + PATH

	// EnvPathWithBin is the environment variable to set PATH with /bin prepended.
	EnvPathWithBin = "PATH=/bin:" + PATH

	// EnvTcellMinimizeEnvironment is the environment variable to minimize tcell library memory usage (skips rune width calculation).
	EnvTcellMinimizeEnvironment = "TCELL_MINIMIZE=1"

	// EnvGRPCEnforccceALPNEnabled is the environment variable to disable gRPC ALPN enforcement.
	EnvGRPCEnforccceALPNEnabled = "GRPC_ENFORCE_ALPN_ENABLED=false"

	// EnvTerm is the environment variable to set terminal type.
	EnvTerm = "TERM=linux"

	// EnvGoraceHaltOnError is the environment variable to set GORACE to halt on error.
	EnvGoraceHaltOnError = "GORACE=halt_on_error=1"

	// EnvFIPS140ModeStrict is the environment variable to set Go crypto to FIPS 140 strict mode.
	EnvFIPS140ModeStrict = "GODEBUG=fips140=only"

	// EnvXDGRuntimeDir is a default value for XDG_RUNTIME_DIR for the services running on the host.
	//
	// See https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
	EnvXDGRuntimeDir = "XDG_RUNTIME_DIR=/run"
)

// EnvApidGomemlimit is the environment variable to set GOMEMLIMIT for apid process.
func EnvApidGomemlimit() string {
	return "GOMEMLIMIT=" + strconv.Itoa(CgroupApidMaxMemory/5*4)
}

// EnvDashboardGomemlimit is the environment variable to set GOMEMLIMIT for dashboard process.
func EnvDashboardGomemlimit() string {
	return "GOMEMLIMIT=" + strconv.Itoa(CgroupDashboardMaxMemory/5*4)
}

// EnvTrustdGomemlimit is the environment variable to set GOMEMLIMIT for trustd process.
func EnvTrustdGomemlimit() string {
	return "GOMEMLIMIT=" + strconv.Itoa(CgroupTrustdMaxMemory/5*4)
}
