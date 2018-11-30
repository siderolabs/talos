package partition

import (
	"github.com/google/uuid"
)

// Options is the functional options struct.
type Options struct {
	Type uuid.UUID
	Name string
	Test bool
}

// Option is the functional option func.
type Option func(*Options)

// WithPartitionType sets the partition type.
func WithPartitionType(id string) Option {
	return func(args *Options) {
		// TODO: An Option should return an error.
		// nolint: errcheck
		guuid, _ := uuid.Parse(id)
		args.Type = guuid
	}
}

// WithPartitionName sets the partition name.
func WithPartitionName(o string) Option {
	return func(args *Options) {
		args.Name = o
	}
}

func WithPartitionTest(t bool) Option {
	return func(args *Options) {
		args.Test = t
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...interface{}) *Options {
	// Default to data type "af3dc60f-8384-7247-8e79-3d69d8477de4"
	// TODO: An Option should return an error.
	// nolint: errcheck
	guuid, _ := uuid.Parse("af3dc60f-8384-7247-8e79-3d69d8477de4")

	opts := &Options{
		Type: guuid,
		Name: "",
		Test: false,
	}

	for _, setter := range setters {
		if s, ok := setter.(Option); ok {
			s(opts)
		}
	}

	return opts
}
