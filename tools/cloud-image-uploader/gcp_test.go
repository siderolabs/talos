// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"net/http"
	"testing"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func TestGCPOperationError(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		op          *compute.Operation
		expectedErr string
	}{
		{
			name: "success",
			op: &compute.Operation{
				ServerResponse: googleapi.ServerResponse{
					HTTPStatusCode: http.StatusOK,
				},
			},
		},
		{
			name: "http error",
			op: &compute.Operation{
				HttpErrorMessage: "not found",
				ServerResponse: googleapi.ServerResponse{
					HTTPStatusCode: http.StatusNotFound,
				},
			},
			expectedErr: "gcp: operation failed with http error message: not found",
		},
		{
			name: "operation error",
			op: &compute.Operation{
				Error: &compute.OperationError{
					Errors: []*compute.OperationErrorErrors{
						{
							Message: "quota exceeded",
						},
					},
				},
				ServerResponse: googleapi.ServerResponse{
					HTTPStatusCode: http.StatusOK,
				},
			},
			expectedErr: "gcp: operation failed with error message: quota exceeded",
		},
		{
			name: "operation error without details",
			op: &compute.Operation{
				Error: &compute.OperationError{},
				ServerResponse: googleapi.ServerResponse{
					HTTPStatusCode: http.StatusOK,
				},
			},
			expectedErr: "gcp: operation failed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := gcpOperationError(test.op)
			if test.expectedErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %q", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected error %q, got nil", test.expectedErr)
			}

			if err.Error() != test.expectedErr {
				t.Fatalf("expected error %q, got %q", test.expectedErr, err)
			}
		})
	}
}
