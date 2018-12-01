package xfs

// Options is the functional options struct.
type Options struct {
	Label string
	Force bool
}

// Option is the functional option func.
type Option func(*Options)

// WithLabel sets the filesystem label.
func WithLabel(o string) Option {
	return func(args *Options) {
		args.Label = o
	}
}

// WithForce forces the creation of the filesystem.
func WithForce(o bool) Option {
	return func(args *Options) {
		args.Force = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Label: "",
		Force: false,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
