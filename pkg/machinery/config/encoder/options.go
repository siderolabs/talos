// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encoder

// CommentsFlags comments encoding flags type.
type CommentsFlags int

func (f CommentsFlags) enabled(flag CommentsFlags) bool {
	return (f & flag) == flag
}

const (
	// CommentsDisabled renders no comments.
	CommentsDisabled CommentsFlags = 0
	// CommentsExamples enables commented yaml examples rendering.
	CommentsExamples CommentsFlags = 1 << iota
	// CommentsDocs enables rendering each config field short docstring.
	CommentsDocs
	// CommentsAll renders all comments.
	CommentsAll = CommentsExamples | CommentsDocs
)

// Options defines encoder config.
type Options struct {
	Comments CommentsFlags
}

func newOptions(opts ...Option) *Options {
	res := &Options{
		Comments: CommentsAll,
	}

	for _, o := range opts {
		o(res)
	}

	return res
}

// Option gives ability to alter config encoder output settings.
type Option func(*Options)

// WithComments enables comments and examples in the encoder.
func WithComments(flags CommentsFlags) Option {
	return func(o *Options) {
		o.Comments = flags
	}
}
