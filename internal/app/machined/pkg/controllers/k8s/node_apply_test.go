// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"slices"
	"testing"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	v1 "k8s.io/api/core/v1"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

func TestApplyLabels(t *testing.T) { //nolint:dupl
	t.Parallel()

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
			t.Parallel()

			node := &v1.Node{}
			node.Labels = tt.inputLabels

			ownedLabels := xslices.ToSet(tt.ownedLabels)
			if ownedLabels == nil {
				ownedLabels = map[string]struct{}{}
			}

			ctrl.ApplyLabels(logger, node, ownedLabels, tt.labelSpec)

			newOwnedLabels := maps.Keys(ownedLabels)
			if newOwnedLabels == nil {
				newOwnedLabels = []string{}
			}

			slices.Sort(newOwnedLabels)

			assert.Equal(t, tt.expectedLabels, node.Labels)
			assert.Equal(t, tt.expectedOwnedLabels, newOwnedLabels)
		})
	}
}

func TestApplyAnnotations(t *testing.T) { //nolint:dupl
	t.Parallel()

	ctrl := &k8sctrl.NodeApplyController{}
	logger := zaptest.NewLogger(t)

	for _, tt := range []struct {
		name             string
		inputAnnotations map[string]string
		ownedAnnotations []string
		annotationSpec   map[string]string

		expectedAnnotations      map[string]string
		expectedOwnedAnnotations []string
	}{
		{
			name:             "empty",
			inputAnnotations: map[string]string{},
			ownedAnnotations: []string{},
			annotationSpec:   map[string]string{},

			expectedAnnotations:      map[string]string{},
			expectedOwnedAnnotations: []string{},
		},
		{
			name: "initial annotations",
			inputAnnotations: map[string]string{
				"hostname": "foo",
			},
			ownedAnnotations: []string{},
			annotationSpec: map[string]string{
				"talos/foo": "value1",
				"talos/bar": "value2",
			},

			expectedAnnotations: map[string]string{
				"hostname":  "foo",
				"talos/foo": "value1",
				"talos/bar": "value2",
			},
			expectedOwnedAnnotations: []string{
				"talos/bar",
				"talos/foo",
			},
		},
		{
			name: "update owned annotations",
			inputAnnotations: map[string]string{
				"hostname": "foo",
				"label1":   "value1",
				"label2":   "value2",
			},
			ownedAnnotations: []string{
				"label1",
				"label2",
			},
			annotationSpec: map[string]string{
				"label1": "value3",
			},

			expectedAnnotations: map[string]string{
				"hostname": "foo",
				"label1":   "value3",
			},
			expectedOwnedAnnotations: []string{
				"label1",
			},
		},
		{
			name: "ignore not owned annotations",
			inputAnnotations: map[string]string{
				"hostname": "foo",
				"ann1":     "value1",
				"ann2":     "value2",
				"ann3":     "value3",
			},
			ownedAnnotations: []string{},
			annotationSpec: map[string]string{
				"ann1": "value3",
				"ann2": "value2",
			},

			expectedAnnotations: map[string]string{
				"hostname": "foo",
				"ann1":     "value1",
				"ann2":     "value2",
				"ann3":     "value3",
			},
			expectedOwnedAnnotations: []string{
				"ann2",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := &v1.Node{}
			node.Annotations = tt.inputAnnotations

			ownedAnnotations := xslices.ToSet(tt.ownedAnnotations)
			if ownedAnnotations == nil {
				ownedAnnotations = map[string]struct{}{}
			}

			ctrl.ApplyAnnotations(logger, node, ownedAnnotations, tt.annotationSpec)

			newOwnedAnnotations := maps.Keys(ownedAnnotations)
			if newOwnedAnnotations == nil {
				newOwnedAnnotations = []string{}
			}

			slices.Sort(newOwnedAnnotations)

			assert.Equal(t, tt.expectedAnnotations, node.Annotations)
			assert.Equal(t, tt.expectedOwnedAnnotations, newOwnedAnnotations)
		})
	}
}

func TestApplyTaints(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			node := &v1.Node{}
			node.Spec.Taints = tt.inputTaints

			ownedTaints := xslices.ToSet(tt.ownedTaints)
			if ownedTaints == nil {
				ownedTaints = map[string]struct{}{}
			}

			ctrl.ApplyTaints(logger, node, ownedTaints, tt.taintSpec)

			newOwnedTaints := maps.Keys(ownedTaints)
			if newOwnedTaints == nil {
				newOwnedTaints = []string{}
			}

			slices.Sort(newOwnedTaints)

			assert.Equal(t, tt.expectedTaints, node.Spec.Taints)
			assert.Equal(t, tt.expectedOwnedTaints, newOwnedTaints)
		})
	}
}

func TestApplyCordoned(t *testing.T) {
	t.Parallel()

	ctrl := &k8sctrl.NodeApplyController{}
	logger := zaptest.NewLogger(t)

	for _, tt := range []struct {
		name               string
		inputAnnotations   map[string]string
		inputUnschedulable bool
		shouldCordon       bool

		expectedUnschedulable bool
		expectedAnnotations   map[string]string
	}{
		{
			name:               "not cordoned - uncordon",
			inputAnnotations:   nil,
			inputUnschedulable: false,
			shouldCordon:       false,

			expectedUnschedulable: false,
			expectedAnnotations:   nil,
		},
		{
			name:               "not cordoned - cordon",
			inputAnnotations:   nil,
			inputUnschedulable: false,
			shouldCordon:       true,

			expectedUnschedulable: true,
			expectedAnnotations:   map[string]string{constants.AnnotationCordonedKey: constants.AnnotationCordonedValue},
		},
		{
			name:               "cordoned - no annotation - cordon",
			inputAnnotations:   nil,
			inputUnschedulable: true,
			shouldCordon:       true,

			expectedUnschedulable: true,
			expectedAnnotations:   nil,
		},
		{
			name:               "cordoned - with annotation - cordon",
			inputAnnotations:   map[string]string{constants.AnnotationCordonedKey: constants.AnnotationCordonedValue},
			inputUnschedulable: true,
			shouldCordon:       true,

			expectedUnschedulable: true,
			expectedAnnotations:   map[string]string{constants.AnnotationCordonedKey: constants.AnnotationCordonedValue},
		},
		{
			name:               "cordoned - with annotation - uncordon",
			inputAnnotations:   map[string]string{constants.AnnotationCordonedKey: constants.AnnotationCordonedValue},
			inputUnschedulable: true,
			shouldCordon:       false,

			expectedUnschedulable: false,
			expectedAnnotations:   map[string]string{},
		},
		{
			name:               "cordoned - no annotation - uncordon",
			inputAnnotations:   map[string]string{"foo": "bar"},
			inputUnschedulable: true,
			shouldCordon:       false,

			expectedUnschedulable: true,
			expectedAnnotations:   map[string]string{"foo": "bar"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := &v1.Node{}
			node.Annotations = tt.inputAnnotations
			node.Spec.Unschedulable = tt.inputUnschedulable

			ctrl.ApplyCordoned(logger, node, tt.shouldCordon)

			assert.Equal(t, tt.expectedUnschedulable, node.Spec.Unschedulable)
			assert.Equal(t, tt.expectedAnnotations, node.Annotations)
		})
	}
}
