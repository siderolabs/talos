// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// OOMKind is a Out of Memory Handler document kind.
const OOMKind = "OOMConfig"

func init() {
	registry.Register(OOMKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &OOMV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.Validator = &OOMV1Alpha1{}
	_ config.OOMConfig = &OOMV1Alpha1{}
)

// OOMV1Alpha1 is a Out of Memory handler config document.
//
//	examples:
//	  - value: exampleOOMV1Alpha1()
//	alias: OOMConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/OOMConfig
type OOMV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     This expression defines when to trigger OOM action.
	//
	//     The expression must evaluate to a boolean value.
	//     If the expression returns true, then OOM ranking and killing will be handled.
	//   schema:
	//     type: string
	OOMTriggerExpression cel.Expression `yaml:"triggerExpression,omitempty"`
	//   description: |
	//     This expression defines how to rank cgroups for OOM handler.
	//
	//     The cgroup with the highest rank (score) will be evicted first.
	//     The expression must evaluate to a double value.
	//   schema:
	//     type: string
	OOMCgroupRankingExpression cel.Expression `yaml:"cgroupRankingExpression,omitempty"`
}

// NewOOMV1Alpha1 creates a new eventsink config document.
func NewOOMV1Alpha1() *OOMV1Alpha1 {
	return &OOMV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       OOMKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleOOMV1Alpha1() *OOMV1Alpha1 {
	cfg := NewOOMV1Alpha1()
	cfg.OOMCgroupRankingExpression = cel.MustExpression(cel.ParseDoubleExpression(
		`memory_max.hasValue() ? 0.0 :
			{Besteffort : 1.0, Guaranteed: 0.0, Burstable: 0.5}[class] *
			   double(memory_current.orValue(0u)) / double(memory_peak.orValue(0u) - memory_current.orValue(0u))`,
		celenv.OOMCgroupScoring(),
	))

	return cfg
}

// Clone implements config.Document interface.
func (s *OOMV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *OOMV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	if !s.OOMCgroupRankingExpression.IsZero() {
		if err := s.OOMCgroupRankingExpression.ParseDouble(celenv.OOMCgroupScoring()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("OOM cgroup scoring expression is invalid: %w", err))
		}
	}

	return nil, validationErrors
}

// TriggerExpression returns the OOM cgroup ranking expression.
func (s *OOMV1Alpha1) TriggerExpression() optional.Optional[cel.Expression] {
	if s.OOMCgroupRankingExpression.IsZero() {
		return optional.None[cel.Expression]()
	}

	return optional.Some(s.OOMTriggerExpression)
}

// CgroupRankingExpression returns the OOM cgroup ranking expression.
func (s *OOMV1Alpha1) CgroupRankingExpression() optional.Optional[cel.Expression] {
	if s.OOMCgroupRankingExpression.IsZero() {
		return optional.None[cel.Expression]()
	}

	return optional.Some(s.OOMCgroupRankingExpression)
}
