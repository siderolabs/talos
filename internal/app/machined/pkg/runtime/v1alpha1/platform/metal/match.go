// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal

import "regexp"

func keyToVar(key string) string {
	return `${` + key + `}`
}

type matcher struct {
	Key    string
	Regexp *regexp.Regexp
}

func newMatcher(key string) *matcher {
	return &matcher{
		Key:    keyToVar(key),
		Regexp: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(keyToVar(key))),
	}
}

type replacer struct {
	original string
	Regexp   *regexp.Regexp
	Matches  [][]int
}

func (m *matcher) process(original string) *replacer {
	var r replacer
	r.Regexp = m.Regexp
	r.original = original

	r.Matches = m.Regexp.FindAllStringIndex(original, -1)

	return &r
}

func (r *replacer) ReplaceMatches(replacement string) string {
	var res string

	if len(r.Matches) < 1 {
		return res
	}

	res += r.original[:r.Matches[0][0]]
	res += replacement

	for i := 0; i < len(r.Matches)-1; i++ {
		res += r.original[r.Matches[i][1]:r.Matches[i+1][0]]
		res += replacement
	}

	res += r.original[r.Matches[len(r.Matches)-1][1]:]

	return res
}
