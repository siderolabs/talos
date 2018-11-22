package blockdevice

// Options is the functional options struct.
type Options struct {
	CreateGPT bool
}

// Option is the functional option func.
type Option func(*Options)

// WithNewGPT opens the blockdevice with a new GPT.
func WithNewGPT(o bool) Option {
	return func(args *Options) {
		args.CreateGPT = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		CreateGPT: false,
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
