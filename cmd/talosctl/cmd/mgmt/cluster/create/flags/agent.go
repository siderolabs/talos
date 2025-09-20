// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags

import "fmt"

// Agent represents the type of the agent to use for cluster management.
type Agent uint8

func (a *Agent) String() string {
	switch *a {
	case 1:
		return "wireguard"
	case 2:
		return "grpc-tunnel"
	case 3:
		return "wireguard+tls"
	case 4:
		return "grpc-tunnel+tls"
	default:
		return "none"
	}
}

// Set implements pflag.Value interface.
func (a *Agent) Set(s string) error {
	switch s {
	case "true", "wireguard":
		*a = 1
	case "tunnel":
		*a = 2
	case "wireguard+tls":
		*a = 3
	case "grpc-tunnel+tls":
		*a = 4
	default:
		return fmt.Errorf("unknown type: %s, possible values: 'true', 'wireguard' for the usual WG; 'tunnel' for WG over GRPC, add '+tls' to enable TLS for API", s)
	}

	return nil
}

// Type implements pflag.Value interface.
func (a *Agent) Type() string { return "agent" }

// IsEnabled returns true if the agent is enabled.
func (a *Agent) IsEnabled() bool { return *a != 0 }

// IsTunnel returns true if the agent is a tunnel.
func (a *Agent) IsTunnel() bool { return *a == 2 || *a == 4 }

// IsTLS returns true if the agent is using TLS.
func (a *Agent) IsTLS() bool { return *a == 3 || *a == 4 }
