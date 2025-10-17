// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"time"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	//
	//     This expression receives the following parameters:
	//     - memory_{some,full}_{avg10,avg60,avg300,total} - double, representing PSI values
	//     - time_since_trigger - duration since the last OOM handler trigger event
	//   schema:
	//     type: string
	OOMTriggerExpression cel.Expression `yaml:"triggerExpression,omitempty"`
	//   description: |
	//     This expression defines how to rank cgroups for OOM handler.
	//
	//     The cgroup with the highest rank (score) will be evicted first.
	//     The expression must evaluate to a double value.
	//
	//     This expression receives the following parameters:
	//     - memory_max - Optional<uint> - in bytes
	//     - memory_current - Optional<uint> - in bytes
	//     - memory_peak - Optional<uint> - in bytes
	//     - path - string, path to the cgroup
	//     - class - int. This represents cgroup QoS class, and matches one of the constants, which are also provided: Besteffort, Burstable, Guaranteed, Podruntime, System
	//   schema:
	//     type: string
	OOMCgroupRankingExpression cel.Expression `yaml:"cgroupRankingExpression,omitempty"`
	//   description: |
	//     How often should the trigger expression be evaluated.
	//
	//     This interval determines how often should the OOM controller
	//     check for the OOM condition using the provided expression.
	//     Adjusting it can help tune the reactivity of the OOM handler.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	OOMSampleInterval time.Duration `yaml:"sampleInterval,omitempty"`
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
	cfg.OOMSampleInterval = 100 * time.Millisecond
	cfg.OOMTriggerExpression = cel.MustExpression(cel.ParseBooleanExpression(
		constants.DefaultOOMTriggerExpression,
		celenv.OOMTrigger(),
	))
	cfg.OOMCgroupRankingExpression = cel.MustExpression(cel.ParseDoubleExpression(
		constants.DefaultOOMCgroupRankingExpression,
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

	if !s.OOMTriggerExpression.IsZero() {
		if err := s.OOMTriggerExpression.ParseBool(celenv.OOMTrigger()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("OOM trigger expression is invalid: %w", err))
		}
	}

	if !s.OOMCgroupRankingExpression.IsZero() {
		if err := s.OOMCgroupRankingExpression.ParseDouble(celenv.OOMCgroupScoring()); err != nil {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("OOM cgroup scoring expression is invalid: %w", err))
		}
	}

	if s.OOMSampleInterval < 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("OOM sample interval must be longer than 0"))
	}

	return nil, validationErrors
}

// TriggerExpression returns the OOM trigger expression.
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

// SampleInterval returns the OOM sampling interval.
func (s *OOMV1Alpha1) SampleInterval() optional.Optional[time.Duration] {
	if s.OOMSampleInterval == 0 {
		return optional.None[time.Duration]()
	}

	return optional.Some(s.OOMSampleInterval)
}
