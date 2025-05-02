// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// TaskStatusType is type of TaskStatus resource.
const TaskStatusType = resource.Type("TaskStatuses.runtime.talos.dev")

// TaskStatus resource holds status of watchdog timer.
type TaskStatus = typed.Resource[TaskStatusSpec, TaskStatusExtension]

//go:generate enumer -type=TaskState -linecomment -text

// TaskState describes the task state.
type TaskState int

// Background task state.
//
//structprotogen:gen_enum
const (
	TaskStateCreated   TaskState = iota // created
	TaskStateRunning                    // running
	TaskStateCompleted                  // completed
)

// TaskStatusSpec describes configuration of watchdog timer.
//
//gotagsrewrite:gen
type TaskStatusSpec struct {
	ID         string        `yaml:"id" protobuf:"1"`
	TaskStatus TaskState     `yaml:"taskState" protobuf:"2"`
	ExitCode   int           `yaml:"exitCode" protobuf:"3"`
	Start      time.Time     `yaml:"start" protobuf:"4"`
	Duration   time.Duration `yaml:"duration" protobuf:"5"`
}

// NewTaskStatus initializes a TaskStatus resource.
func NewTaskStatus(id string) *TaskStatus {
	return typed.NewResource[TaskStatusSpec, TaskStatusExtension](
		resource.NewMetadata(NamespaceName, TaskStatusType, id, resource.VersionUndefined),
		TaskStatusSpec{},
	)
}

// TaskStatusExtension is auxiliary resource data for TaskStatus.
type TaskStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (TaskStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TaskStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "ID",
				JSONPath: `{.id}`,
			},
			{
				Name:     "TaskState",
				JSONPath: `{.taskState}`,
			},
			{
				Name:     "ExitCode",
				JSONPath: `{.exitCode}`,
			},
			{
				Name:     "Start",
				JSONPath: `{.start}`,
			},
			{
				Name:     "Duration",
				JSONPath: `{.duration}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[TaskStatusSpec](TaskStatusType, &TaskStatus{})
	if err != nil {
		panic(err)
	}
}
