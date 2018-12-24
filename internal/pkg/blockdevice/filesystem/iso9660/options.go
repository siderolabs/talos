package iso9660

// Options is the functional options struct.
type Options struct {
}

// Option is the functional option func.
type Option func(*Options)

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
