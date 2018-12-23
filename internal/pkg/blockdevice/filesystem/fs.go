package filesystem

// SuperBlocker describes the requirements for file system super blocks.
type SuperBlocker interface {
	Is() bool
	Offset() int64
	Type() string
}
