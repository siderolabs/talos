// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package log_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/log"
)

type testLogger struct {
	lines []string
}

func (l *testLogger) Printf(format string, v ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, v...))
}

type testStream struct {
	grpc.ServerStream

	ctx context.Context //nolint:containedctx
}

func (s *testStream) Context() context.Context {
	return s.ctx
}

func TestAnnotateUnary(t *testing.T) {
	logger := &testLogger{}
	middleware := log.NewMiddleware(logger)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("foo", "bar"))

	_, err := middleware.UnaryInterceptor()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		func(ctx context.Context, _ any) (any, error) {
			log.Annotatef(ctx, "authorized based on PID (%d)", 42)
			log.Annotatef(ctx, "authorized (%v includes %v)", []string{"os:admin"}, []string{"os:admin"})

			return nil, nil
		})
	require.NoError(t, err)

	require.Len(t, logger.lines, 1)
	assert.Regexp(t,
		regexp.MustCompile(`^OK \[/test\.Service/Method\] \S+ unary Success \{authorized based on PID \(42\); `+
			`authorized \(\[os:admin\] includes \[os:admin\]\)\} \(foo=bar\)$`),
		logger.lines[0])
}

func TestAnnotateStream(t *testing.T) {
	logger := &testLogger{}
	middleware := log.NewMiddleware(logger)

	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("foo", "bar"))

	err := middleware.StreamInterceptor()(nil, &testStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/test.Service/Method"},
		func(_ any, stream grpc.ServerStream) error {
			log.Annotatef(stream.Context(), "not authorized")

			return nil
		})
	require.NoError(t, err)

	require.Len(t, logger.lines, 1)
	assert.Regexp(t,
		regexp.MustCompile(`^OK \[/test\.Service/Method\] \S+ stream Success \{not authorized\} \(foo=bar\)$`),
		logger.lines[0])
}

func TestAnnotateNoAnnotations(t *testing.T) {
	logger := &testLogger{}
	middleware := log.NewMiddleware(logger)

	_, err := middleware.UnaryInterceptor()(t.Context(), nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
		func(context.Context, any) (any, error) {
			return nil, nil
		})
	require.NoError(t, err)

	require.Len(t, logger.lines, 1)
	assert.Regexp(t, regexp.MustCompile(`^OK \[/test\.Service/Method\] \S+ unary Success \(\)$`), logger.lines[0])
}

func TestAnnotateNoMiddleware(t *testing.T) {
	// should be a no-op, not a panic
	log.Annotatef(t.Context(), "no logging middleware installed")
}
