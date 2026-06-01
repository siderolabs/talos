// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"iter"
	"maps"
)

func findFirstValue[T any, R comparable](documents []T, getter func(T) R) R {
	var zeroR R

	for _, document := range documents {
		if value := getter(document); value != zeroR {
			return value
		}
	}

	return zeroR
}

func aggregateValues[T any, R any](documents []T, getter func(T) []R) []R {
	result := make([]R, 0, len(documents))

	for _, document := range documents {
		result = append(result, getter(document)...)
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func mergeMaps[T any, K comparable, V any](documents []T, getter func(T) iter.Seq2[K, V]) map[K]V {
	result := make(map[K]V)

	for _, document := range documents {
		maps.Insert(result, getter(document))
	}

	return result
}

func filterDocuments[T any, R any](documents []R) []T {
	var result []T

	for _, document := range documents {
		if document, ok := any(document).(T); ok {
			result = append(result, document)
		}
	}

	return result
}
