// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides network machine configuration documents.
package network

//go:generate docgen -output network_doc.go network.go default_action_config.go kubespan_endpoints.go port_range.go rule_config.go

//go:generate deep-copy -type DefaultActionConfigV1Alpha1 -type KubespanEndpointsConfigV1Alpha1 -type RuleConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
