package resource

import "fmt"

var (
	ErrDeps = fmt.Errorf("dependencies not yet available")
	ErrInvalidConfig = fmt.Errorf("configuration validation failed")
)

// Class defines the broad functional class to which a resource belongs.
type Class int

const (
	ClassUnknown = iota
	ClassInterface
	ClassAddress
	ClassRoute
)

// Controller manages a network resource.
type Controller interface {
	Enable() error
	Disable() error
	Status() Status
}

// Status describes the most basic status of a resource.
// It can be type-asserted for more specific details.
type Status interface {
	Class() Class
	Kind() string

	Enabled() bool
	Up() bool
}
