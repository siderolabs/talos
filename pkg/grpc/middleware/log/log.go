// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package log provides simple grpc logging middleware
package log

import (
	"context"
	"slices"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"github.com/siderolabs/gen/maps"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Middleware provides grpc logging middleware.
type Middleware struct {
	logger Logger
}

// Logger is the interface that the Middleware expects for logging.
type Logger interface {
	Printf(format string, v ...any)
}

// NewMiddleware creates new logging middleware.
func NewMiddleware(logger Logger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

var sensitiveFields = map[string]struct{}{
	"token": {},
}

// ExtractMetadata formats metadata from incoming grpc context as string for the log.
func ExtractMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}

	// the peer address always comes from the connection itself, never from the client-supplied metadata
	delete(md, "peer")

	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		md["peer"] = []string{p.Addr.String()}
	}

	keys := maps.Keys(md)
	slices.Sort(keys)

	pairs := make([]string, 0, len(keys))

	for _, key := range keys {
		value := strings.Join(md[key], ",")

		if _, sensitive := sensitiveFields[key]; sensitive {
			value = "<hidden>"
		}

		pairs = append(pairs, key+"="+value)
	}

	return strings.Join(pairs, ";")
}

// UnaryInterceptor returns grpc UnaryServerInterceptor.
func (m *Middleware) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startTime := time.Now()

		ctx, annotations := withAnnotations(ctx)

		resp, err := handler(ctx, req)

		duration := time.Since(startTime)
		code := status.Code(err)

		msg := "Success"
		if err != nil {
			msg = err.Error()
		}

		m.logger.Printf("%s [%s] %s unary %s%s (%s)", code, info.FullMethod, duration, msg, annotations, ExtractMetadata(ctx))

		return resp, err
	}
}

// StreamInterceptor returns grpc StreamServerInterceptor.
func (m *Middleware) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()

		ctx, annotations := withAnnotations(stream.Context())

		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = ctx

		err := handler(srv, wrapped)

		duration := time.Since(startTime)
		code := status.Code(err)

		msg := "Success"
		if err != nil {
			msg = err.Error()
		}

		m.logger.Printf("%s [%s] %s stream %s%s (%s)", code, info.FullMethod, duration, msg, annotations, ExtractMetadata(ctx))

		return err
	}
}
