// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"sort"
	"testing"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/slices"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
)

func TestApplyLabels(t *testing.T) {
	ctrl := &k8s.NodeLabelsApplyController{}
	logger := zaptest.NewLogger(t)

	for _, tt := range []struct {
		name        string
		inputLabels map[string]string
		ownedLabels []string
		labelSpec   map[string]string

		expectedLabels      map[string]string
		expectedOwnedLabels []string
	}{
		{
			name:        "empty",
			inputLabels: map[string]string{},
			ownedLabels: []string{},
			labelSpec:   map[string]string{},

			expectedLabels:      map[string]string{},
			expectedOwnedLabels: []string{},
		},
		{
			name: "initial set labels",
			inputLabels: map[string]string{
				"hostname": "foo",
			},
			ownedLabels: []string{},
			labelSpec: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},

			expectedLabels: map[string]string{
				"hostname": "foo",
				"label1":   "value1",
				"label2":   "value2",
			},
			expectedOwnedLabels: []string{
				"label1",
				"label2",
			},
		},
		{
			name: "update owned labels",
			inputLabels: map[string]string{
				"hostname": "foo",
				"label1":   "value1",
				"label2":   "value2",
			},
			ownedLabels: []string{
				"label1",
				"label2",
			},
			labelSpec: map[string]string{
				"label1": "value3",
			},

			expectedLabels: map[string]string{
				"hostname": "foo",
				"label1":   "value3",
			},
			expectedOwnedLabels: []string{
				"label1",
			},
		},
		{
			name: "ignore not owned labels",
			inputLabels: map[string]string{
				"hostname": "foo",
				"label1":   "value1",
				"label2":   "value2",
				"label3":   "value3",
			},
			ownedLabels: []string{
				"label2",
			},
			labelSpec: map[string]string{
				"label1": "value3",
				"label2": "value2",
			},

			expectedLabels: map[string]string{
				"hostname": "foo",
				"label1":   "value1",
				"label2":   "value2",
				"label3":   "value3",
			},
			expectedOwnedLabels: []string{
				"label2",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1.Node{}
			node.Labels = tt.inputLabels

			ownedLabels := slices.ToSet(tt.ownedLabels)
			if ownedLabels == nil {
				ownedLabels = map[string]struct{}{}
			}

			ctrl.ApplyLabels(logger, node, ownedLabels, tt.labelSpec)

			newOwnedLabels := maps.Keys(ownedLabels)
			if newOwnedLabels == nil {
				newOwnedLabels = []string{}
			}

			sort.Strings(newOwnedLabels)

			assert.Equal(t, tt.expectedLabels, node.Labels)
			assert.Equal(t, tt.expectedOwnedLabels, newOwnedLabels)
		})
	}
}
