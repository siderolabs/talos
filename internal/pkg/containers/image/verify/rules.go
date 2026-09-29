// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package verify

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/ryanuber/go-glob"

	"github.com/siderolabs/talos/internal/pkg/containers/image/imageref"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

// RuleMatchFunc matches an image reference against the image verification rules.
//
// The image reference is normalized before it is matched, and the returned rule is the first one
// whose (also normalized) pattern matches; a nil rule means no rule matched.
type RuleMatchFunc func(imageRef string) (*security.ImageVerificationRule, error)

// NewRuleMatcher builds a matcher for the image verification rules present in the state.
//
// Both the image reference and the rule patterns are normalized with [imageref], so that a rule is
// matched against the very reference which is going to be pulled: matching a reference in one
// spelling while pulling another would let a rule be evaded by, say, upper-casing the registry
// domain.
func NewRuleMatcher(ctx context.Context, st state.State) (RuleMatchFunc, error) {
	rules, err := safe.StateListAll[*security.ImageVerificationRule](ctx, st)
	if err != nil {
		return nil, fmt.Errorf("failed to list image verification rules: %w", err)
	}

	type normalizedRule struct {
		rule    *security.ImageVerificationRule
		pattern string
	}

	normalizedRules := make([]normalizedRule, 0, rules.Len())

	for rule := range rules.All() {
		if rule.TypedSpec().ImagePattern == "" {
			continue
		}

		normalizedRules = append(normalizedRules, normalizedRule{
			rule:    rule,
			pattern: imageref.NormalizePattern(rule.TypedSpec().ImagePattern),
		})
	}

	return func(imageRef string) (*security.ImageVerificationRule, error) {
		namedRef, err := imageref.Parse(imageRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %q: %w", imageRef, err)
		}

		// rules match on the registry and the repository only, never on the tag or the digest
		repositoryKey := imageref.RepositoryKey(namedRef)

		for _, normalized := range normalizedRules {
			if glob.Glob(normalized.pattern, repositoryKey) {
				return normalized.rule, nil
			}
		}

		return nil, nil
	}, nil
}
