// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// TaskType is type of Task resource.
const TaskType = resource.Type("Tasks.runtime.talos.dev")

// Task resource holds configuration for a task.
type Task = typed.Resource[TaskSpec, TaskExtension]

// TaskID is a resource ID for Task.
const TaskID resource.ID = "task"

// TaskSpec describes a background task to be run by a schedule.
//
//gotagsrewrite:gen
type TaskSpec struct {
	ID       string   `yaml:"id" protobuf:"1"`
	TaskName string   `yaml:"taskName" protobuf:"2"`
	Args     []string `yaml:"args" protobuf:"3"`
}

// NewTask initializes a Task resource.
func NewTask() *Task {
	return typed.NewResource[TaskSpec, TaskExtension](
		resource.NewMetadata(NamespaceName, TaskType, TaskID, resource.VersionUndefined),
		TaskSpec{},
	)
}

// TaskExtension is auxiliary resource data for Task.
type TaskExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (TaskExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TaskType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "ID",
				JSONPath: `{.id}`,
			},
			{
				Name:     "TaskName",
				JSONPath: `{.taskName}`,
			},
			{
				Name:     "Args",
				JSONPath: `{.args}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[TaskSpec](TaskType, &Task{})
	if err != nil {
		panic(err)
	}
}
