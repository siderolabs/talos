package mount

// Options is the functional options struct.
type Options struct {
	Prefix   string
	ReadOnly bool
	Shared   bool
}

// Option is the functional option func.
type Option func(*Options)

// WithPrefix is a functional option for setting the mount point prefix.
func WithPrefix(o string) Option {
	return func(args *Options) {
		args.Prefix = o
	}
}

// WithReadOnly is a functional option for setting the mount point as readonly.
func WithReadOnly(o bool) Option {
	return func(args *Options) {
		args.ReadOnly = o
	}
}

// WithShared is a functional option for setting the mount point as shared.
func WithShared(o bool) Option {
	return func(args *Options) {
		args.Shared = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Prefix:   "",
		ReadOnly: false,
		Shared:   false,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
