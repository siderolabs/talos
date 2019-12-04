// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
)

// NodeError is RPC error from some node
type NodeError struct {
	Node string
	Err  string
}

func (ne *NodeError) Error() string {
	return fmt.Sprintf("%s: %s", ne.Node, ne.Err)
}

// FilterReply removes error responses from reply and builds multierror.
//
//nolint: gocyclo
func FilterReply(reply interface{}, err error) (interface{}, error) {
	if reply == nil {
		return nil, err
	}

	replyStructPtr := reflect.ValueOf(reply)
	if replyStructPtr.Kind() != reflect.Ptr {
		panic("reply should be pointer to struct")
	}

	replyStruct := replyStructPtr.Elem()
	if replyStruct.Kind() != reflect.Struct {
		panic("reply should be struct")
	}

	responseField := replyStruct.FieldByName("Response")
	if !responseField.IsValid() {
		panic("Response field missing")
	}

	if responseField.Kind() != reflect.Slice {
		panic("Response field should be a slice")
	}

	var multiErr *multierror.Error

	for i := 0; i < responseField.Len(); {
		responsePtr := responseField.Index(i)
		if responsePtr.Kind() != reflect.Ptr {
			panic("response slice should container pointers")
		}

		response := responsePtr.Elem()
		if response.Kind() != reflect.Struct {
			panic("response slice should container pointers to structs")
		}

		metadataField := response.FieldByName("Metadata")
		if !metadataField.IsValid() {
			panic("response metadata field missing")
		}

		if metadataField.Kind() != reflect.Ptr {
			panic("response metadata field should be a pointer")
		}

		if metadataField.IsNil() {
			// missing metadata, skip the field
			i++
			continue
		}

		metadata := metadataField.Elem()
		if metadata.Kind() != reflect.Struct {
			panic("response metadata should be struct")
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
			Err:  errorField.String(),
		}

		multiErr = multierror.Append(multiErr, nodeError)

		// remove ith response
		reflect.Copy(responseField.Slice(i, responseField.Len()), responseField.Slice(i+1, responseField.Len()))
		responseField.SetLen(responseField.Len() - 1)
	}

	// if all the responses were error responses...
	if multiErr != nil && responseField.Len() == 0 {
		reply = nil
	}

	return reply, multiErr.ErrorOrNil()
}
