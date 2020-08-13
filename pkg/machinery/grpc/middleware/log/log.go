// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package log provides simple grpc logging middleware
package log

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	metadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Middleware provides grpc logging middleware.
type Middleware struct {
	logger *log.Logger
}

// NewMiddleware creates new logging middleware.
func NewMiddleware(logger *log.Logger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

var sensitiveFields = map[string]struct{}{
	"token": {},
}

// ExtractMetadata formats metadata from incoming grpc context as string for the log.
func ExtractMetadata(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	keys := make([]string, 0, len(md))

	for key := range md {
		keys = append(keys, key)
	}

	sort.Strings(keys)

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
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(startTime)
		code := status.Code(err)

		msg := "Success"
		if err != nil {
			msg = err.Error()
		}

		m.logger.Printf("%s [%s] %s unary %s (%s)", code, info.FullMethod, duration, msg, ExtractMetadata(ctx))

		return resp, err
	}
}

// StreamInterceptor returns grpc StreamServerInterceptor.
func (m *Middleware) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()

		err := handler(srv, stream)

		duration := time.Since(startTime)
		code := status.Code(err)

		msg := "Success"
		if err != nil {
			msg = err.Error()
		}

		m.logger.Printf("%s [%s] %s stream %s (%s)", code, info.FullMethod, duration, msg, ExtractMetadata(stream.Context()))

		return err
	}
}
