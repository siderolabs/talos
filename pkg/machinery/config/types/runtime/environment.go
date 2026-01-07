// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// EnvironmentConfigKind is an environment config document kind.
const EnvironmentConfigKind = "EnvironmentConfig"

func init() {
	registry.Register(EnvironmentConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &EnvironmentV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EnvironmentConfig = &EnvironmentV1Alpha1{}
	_ config.Validator         = &EnvironmentV1Alpha1{}
)

// EnvironmentV1Alpha1 is an environment config document.
//
//	examples:
//	  - value: exampleEnvironmentV1Alpha1()
//	alias: EnvironmentConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/EnvironmentConfig
type EnvironmentV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     This field allows for the addition of environment variables.
	//     All environment variables are set on PID 1 in addition to every service.
	//     Propagation of environment variables to services is done only at initial service start time.
	//     To modify environment variables for services, the node must be restarted.
	//     Multiple values for the same environment variable (in multiple documents) will replace previous values, with the last one taking precedence.
	//     Fully removing an environment variable can only be achieved by removing it from the document and restarting the machine.
	//     Environment variable names are validated, and should:
	//       - start with an uppercase letter, lowercase letter, or an underscore (_) character, and
	//       - contain only uppercase and lowercase letters, underscore (_) characters, and numbers.
	//   values:
	//     - "`GRPC_GO_LOG_VERBOSITY_LEVEL`"
	//     - "`GRPC_GO_LOG_SEVERITY_LEVEL`"
	//     - "`http_proxy`"
	//     - "`https_proxy`"
	//     - "`no_proxy`"
	//   examples:
	//     - name: Environment variables definition examples.
	//       value: exampleEnvVars0()
	//     - value: exampleEnvVars1()
	//     - value: exampleEnvVars2()
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	EnvironmentVariables Env `yaml:"variables"`
}

// Env represents a set of environment variables.
type Env = map[string]string

// NewEnvironmentV1Alpha1 creates a new Environment config document.
func NewEnvironmentV1Alpha1() *EnvironmentV1Alpha1 {
	return &EnvironmentV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       EnvironmentConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleEnvironmentV1Alpha1() *EnvironmentV1Alpha1 {
	cfg := NewEnvironmentV1Alpha1()
	cfg.EnvironmentVariables = exampleEnvVars0()

	return cfg
}

func exampleEnvVars0() Env {
	return Env{
		"GRPC_GO_LOG_VERBOSITY_LEVEL": "99",
		"GRPC_GO_LOG_SEVERITY_LEVEL":  "info",
		"https_proxy":                 "http://SERVER:PORT/",
	}
}

func exampleEnvVars1() Env {
	return Env{
		"GRPC_GO_LOG_SEVERITY_LEVEL": "error",
		"https_proxy":                "https://USERNAME:PASSWORD@SERVER:PORT/",
	}
}

func exampleEnvVars2() Env {
	return Env{
		"https_proxy": "http://DOMAIN\\USERNAME:PASSWORD@SERVER:PORT/",
	}
}

// Clone implements config.Document interface.
func (s *EnvironmentV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Variables implements config.EnvironmentConfig interface.
func (s *EnvironmentV1Alpha1) Variables() Env {
	return s.EnvironmentVariables
}

// POSIX1EnvKeyRegex is a regex for validating POSIX.1 environment variable names.
var POSIX1EnvKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Validate implements config.Validator interface.
func (s *EnvironmentV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var err error

	for key := range s.EnvironmentVariables {
		if !POSIX1EnvKeyRegex.MatchString(key) {
			err = errors.Join(err, fmt.Errorf("invalid environment variable name: %q", key))
		}
	}

	return nil, err
}
