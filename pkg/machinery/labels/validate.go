// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package labels contains adapter label validation functions from Kubernetes.
//
// We want to avoid dependency of machinery on Kubernetes packages.
package labels

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Validate validates that a set of labels are correctly defined.
func Validate(labels map[string]string) error {
	var multiErr *multierror.Error

	keys := maps.Keys(labels)
	slices.Sort(keys)

	for _, k := range keys {
		if err := ValidateQualifiedName(k); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}

		if err := ValidateLabelValue(labels[k]); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	return multiErr.ErrorOrNil()
}

// TotalAnnotationSizeLimitB is the maximum size of all annotations in bytes.
const TotalAnnotationSizeLimitB int64 = 256 * (1 << 10) // 256 kB

// ValidateAnnotations validates that a set of annotations are correctly defined.
func ValidateAnnotations(annotations map[string]string) error {
	var multiErr *multierror.Error

	keys := maps.Keys(annotations)
	slices.Sort(keys)

	var size int64

	for _, k := range keys {
		if err := ValidateQualifiedName(k); err != nil {
			multiErr = multierror.Append(multiErr, err)
		}

		switch k {
		case constants.AnnotationOwnedAnnotations, constants.AnnotationOwnedLabels, constants.AnnotationOwnedTaints:
			multiErr = multierror.Append(multiErr, fmt.Errorf("annotation %q is reserved", k))
		}

		size += int64(len(k)) + int64(len(annotations[k]))
	}

	if size > TotalAnnotationSizeLimitB {
		multiErr = multierror.Append(multiErr, fmt.Errorf("total annotation size exceeds limit of %d bytes", TotalAnnotationSizeLimitB))
	}

	return multiErr.ErrorOrNil()
}

const (
	dns1123LabelFmt           string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	dns1123SubdomainFmt       string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"
	dns1123SubdomainMaxLength int    = 253
)

var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

// ValidateDNS1123Subdomain tests for a string that conforms to the definition of a
// subdomain in DNS (RFC 1123).
func ValidateDNS1123Subdomain(value string) error {
	if len(value) > dns1123SubdomainMaxLength {
		return fmt.Errorf("domain name length exceeds limit of %d: %q", dns1123SubdomainMaxLength, value)
	}

	if !dns1123SubdomainRegexp.MatchString(value) {
		return fmt.Errorf("domain doesn't match required format: %q", value)
	}

	return nil
}

const (
	qnameCharFmt           string = "[A-Za-z0-9]"
	qnameExtCharFmt        string = "[-A-Za-z0-9_.]"
	qualifiedNameFmt       string = "(" + qnameCharFmt + qnameExtCharFmt + "*)?" + qnameCharFmt
	qualifiedNameMaxLength int    = 63
)

var qualifiedNameRegexp = regexp.MustCompile("^" + qualifiedNameFmt + "$")

// ValidateQualifiedName tests whether the value passed is what Kubernetes calls a
// "qualified name".
//
// This is a format used in various places throughout the
// system.  If the value is not valid, a list of error strings is returned.
// Otherwise an empty list (or nil) is returned.
func ValidateQualifiedName(value string) error {
	parts := strings.Split(value, "/")

	var name string

	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		var prefix string

		prefix, name = parts[0], parts[1]

		if len(prefix) == 0 {
			return fmt.Errorf("prefix cannot be empty: %q", value)
		} else if err := ValidateDNS1123Subdomain(prefix); err != nil {
			return fmt.Errorf("prefix %q is invalid: %v", prefix, err)
		}
	default:
		return fmt.Errorf("invalid format: too many slashes: %q", value)
	}

	switch {
	case len(name) == 0:
		return fmt.Errorf("name cannot be empty: %q", value)
	case len(name) > qualifiedNameMaxLength:
		return fmt.Errorf("name is too long: %q (limit is %d)", value, qualifiedNameMaxLength)
	case !qualifiedNameRegexp.MatchString(name):
		return fmt.Errorf("name %q is invalid", name)
	}

	return nil
}

const (
	labelValueFmt       string = "(" + qualifiedNameFmt + ")?"
	labelValueMaxLength int    = 63
)

var labelValueRegexp = regexp.MustCompile("^" + labelValueFmt + "$")

// ValidateLabelValue tests whether the value passed is a valid label value.
//
// If the value is not valid, a list of error strings is returned.
// Otherwise an empty list (or nil) is returned.
func ValidateLabelValue(value string) error {
	if len(value) > labelValueMaxLength {
		return fmt.Errorf("label value length exceeds limit of %d: %q", labelValueMaxLength, value)
	}

	if !labelValueRegexp.MatchString(value) {
		return fmt.Errorf("label value %q is invalid", value)
	}

	return nil
}

// ValidateTaints validates that a set of taints is correctly defined.
func ValidateTaints(taints map[string]string) error {
	var multiErr *multierror.Error

	keys := maps.Keys(taints)
	slices.Sort(keys)

	for _, k := range keys {
		if err := ValidateQualifiedName(k); err != nil {
			multiErr = multierror.Append(multiErr, err)

			continue
		}

		val, effect, found := strings.Cut(taints[k], ":")
		if !found {
			effect = val
		}

		// validate that the taint has a valid effect, which is required to add the taint
		if !slices.Contains(constants.ValidEffects, effect) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("invalid taint effect: %q", effect))

			continue
		}

		if found {
			if err := ValidateLabelValue(val); err != nil {
				multiErr = multierror.Append(multiErr, err)

				continue
			}
		}
	}

	return multiErr.ErrorOrNil()
}
