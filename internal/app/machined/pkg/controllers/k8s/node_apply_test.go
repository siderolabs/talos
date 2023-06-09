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

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

func TestApplyLabels(t *testing.T) {
	ctrl := &k8sctrl.NodeApplyController{}
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
			ownedLabels: []string{},
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

func TestApplyTaints(t *testing.T) {
	ctrl := &k8sctrl.NodeApplyController{}
	logger := zaptest.NewLogger(t)

	for _, tt := range []struct {
		name        string
		inputTaints []v1.Taint
		ownedTaints []string
		taintSpec   []k8s.NodeTaintSpecSpec

		expectedTaints      []v1.Taint
		expectedOwnedTaints []string
	}{
		{
			name:        "empty",
			inputTaints: nil,
			ownedTaints: []string{},
			taintSpec:   nil,

			expectedTaints:      nil,
			expectedOwnedTaints: []string{},
		},
		{
			name: "initial set taints",
			inputTaints: []v1.Taint{
				{
					Key:   "foo",
					Value: "bar",
				},
			},
			ownedTaints: []string{},
			taintSpec: []k8s.NodeTaintSpecSpec{
				{
					Key:    "taint1",
					Value:  "value1",
					Effect: "NoSchedule",
				},
				{
					Key:   "taint2",
					Value: "value2",
				},
			},

			expectedTaints: []v1.Taint{
				{
					Key:   "foo",
					Value: "bar",
				},
				{
					Key:    "taint1",
					Value:  "value1",
					Effect: "NoSchedule",
				},
				{
					Key:   "taint2",
					Value: "value2",
				},
			},
			expectedOwnedTaints: []string{
				"taint1",
				"taint2",
			},
		},
		{
			name: "update owned taints",
			inputTaints: []v1.Taint{
				{
					Key:   "foo",
					Value: "bar",
				},
				{
					Key:    "taint1",
					Value:  "value1",
					Effect: "NoSchedule",
				},
				{
					Key:   "taint2",
					Value: "value2",
				},
			},
			ownedTaints: []string{
				"taint1",
				"taint2",
			},
			taintSpec: []k8s.NodeTaintSpecSpec{
				{
					Key:   "taint1",
					Value: "value3",
				},
			},

			expectedTaints: []v1.Taint{
				{
					Key:   "foo",
					Value: "bar",
				},
				{
					Key:   "taint1",
					Value: "value3",
				},
			},
			expectedOwnedTaints: []string{
				"taint1",
			},
		},
		{
			name: "ignore not owned taints",
			inputTaints: []v1.Taint{
				{
					Key:   "foo",
					Value: "bar",
				},
				{
					Key:    "taint1",
					Value:  "value1",
					Effect: "NoSchedule",
				},
				{
					Key:   "taint2",
					Value: "value2",
				},
			},
			ownedTaints: []string{},
			taintSpec: []k8s.NodeTaintSpecSpec{
				{
					Key:    "taint1",
					Value:  "value1",
					Effect: "NoSchedule",
				},
				{
					Key:   "taint2",
					Value: "value3",
				},
			},

			expectedTaints: []v1.Taint{
				{
					Key:   "foo",
					Value: "bar",
				},
				{
					Key:    "taint1",
					Value:  "value1",
					Effect: "NoSchedule",
				},
				{
					Key:   "taint2",
					Value: "value2",
				},
			},
			expectedOwnedTaints: []string{
				"taint1",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1.Node{}
			node.Spec.Taints = tt.inputTaints

			ownedTaints := slices.ToSet(tt.ownedTaints)
			if ownedTaints == nil {
				ownedTaints = map[string]struct{}{}
			}

			ctrl.ApplyTaints(logger, node, ownedTaints, tt.taintSpec)

			newOwnedTaints := maps.Keys(ownedTaints)
			if newOwnedTaints == nil {
				newOwnedTaints = []string{}
			}

			sort.Strings(newOwnedTaints)

			assert.Equal(t, tt.expectedTaints, node.Spec.Taints)
			assert.Equal(t, tt.expectedOwnedTaints, newOwnedTaints)
		})
	}
}
