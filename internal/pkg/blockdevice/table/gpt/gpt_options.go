package gpt

// Options is the functional options struct.
type Options struct {
	PrimaryGPT        bool
	PhysicalBlockSize int
	LogicalBlockSize  int
}

// Option is the functional option func.
type Option func(*Options)

// WithPrimaryGPT sets the contents of offset 24 in the GPT header to the location of the primary header.
func WithPrimaryGPT(o bool) Option {
	return func(args *Options) {
		args.PrimaryGPT = o
	}
}

// WithPhysicalBlockSize sets the physical block size.
func WithPhysicalBlockSize(o int) Option {
	return func(args *Options) {
		args.PhysicalBlockSize = o
	}
}

// WithLogicalBlockSize sets the logical block size.
func WithLogicalBlockSize(o int) Option {
	return func(args *Options) {
		args.LogicalBlockSize = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...interface{}) *Options {
	opts := &Options{
		PrimaryGPT:        true,
		PhysicalBlockSize: 512,
		LogicalBlockSize:  512,
	}

	for _, setter := range setters {
		if s, ok := setter.(Option); ok {
			s(opts)
		}
	}

	return opts
}
