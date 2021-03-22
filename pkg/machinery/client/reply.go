// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/status"
)

// NodeError is RPC error from some node.
type NodeError struct {
	Node string
	Err  error
}

func (ne *NodeError) Error() string {
	return fmt.Sprintf("%s: %s", ne.Node, ne.Err)
}

// Unwrap implements errors.Unwrap interface.
func (ne *NodeError) Unwrap() error {
	return ne.Err
}

// FilterMessages removes error Messagess from resp and builds multierror.
//
//nolint:gocyclo,cyclop
func FilterMessages(resp interface{}, err error) (interface{}, error) {
	if resp == nil {
		return nil, err
	}

	respStructPtr := reflect.ValueOf(resp)
	if respStructPtr.Kind() != reflect.Ptr {
		panic("response should be pointer to struct")
	}

	if respStructPtr.IsNil() {
		return nil, err
	}

	respStruct := respStructPtr.Elem()
	if respStruct.Kind() != reflect.Struct {
		panic("response should be struct")
	}

	messagesField := respStruct.FieldByName("Messages")
	if !messagesField.IsValid() {
		panic("Messages field missing")
	}

	if messagesField.Kind() != reflect.Slice {
		panic("Messages field should be a slice")
	}

	var multiErr *multierror.Error

	for i := 0; i < messagesField.Len(); {
		MessagesPtr := messagesField.Index(i)
		if MessagesPtr.Kind() != reflect.Ptr {
			panic("Messages slice should container pointers")
		}

		Messages := MessagesPtr.Elem()
		if Messages.Kind() != reflect.Struct {
			panic("Messages slice should container pointers to structs")
		}

		metadataField := Messages.FieldByName("Metadata")
		if !metadataField.IsValid() {
			panic("Messages metadata field missing")
		}

		if metadataField.Kind() != reflect.Ptr {
			panic("Messages metadata field should be a pointer")
		}

		if metadataField.IsNil() {
			// missing metadata, skip the field
			i++

			continue
		}

		metadata := metadataField.Elem()
		if metadata.Kind() != reflect.Struct {
			panic("Messages metadata should be struct")
		}

		errorField := metadata.FieldByName("Error")
		if !errorField.IsValid() {
			panic("metadata.Error field missing")
		}

		if errorField.Kind() != reflect.String {
			panic("metadata.Error should be string")
		}

		if errorField.IsZero() {
			// no error, leave it as is
			i++

			continue
		}

		rpcError := errors.New(errorField.String())

		statusField := metadata.FieldByName("Status")
		if !statusField.IsValid() {
			panic("metadata.Status field missing")
		}

		if statusField.Kind() != reflect.Ptr {
			panic("metadata.Status should be pointer")
		}

		if !statusField.IsZero() {
			statusValue, ok := statusField.Interface().(*rpcstatus.Status)
			if !ok {
				panic("metadata.Status should be of type *status.Status")
			}

			rpcError = status.FromProto(statusValue).Err()
		}

		hostnameField := metadata.FieldByName("Hostname")
		if !hostnameField.IsValid() {
			panic("metadata.Hostname field missing")
		}

		if hostnameField.Kind() != reflect.String {
			panic("metadata.Hostname should be string")
		}

		// extract error
		nodeError := &NodeError{
			Node: hostnameField.String(),
			Err:  rpcError,
		}

		multiErr = multierror.Append(multiErr, nodeError)

		// remove ith Messages
		reflect.Copy(messagesField.Slice(i, messagesField.Len()), messagesField.Slice(i+1, messagesField.Len()))
		messagesField.SetLen(messagesField.Len() - 1)
	}

	// if all the Messagess were error Messagess...
	if multiErr != nil && messagesField.Len() == 0 {
		resp = nil
	}

	return resp, multiErr.ErrorOrNil()
}
