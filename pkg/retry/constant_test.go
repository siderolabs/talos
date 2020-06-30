// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"fmt"
	"testing"
	"time"
)

// nolint: scopelint
func Test_constantRetryer_Retry(t *testing.T) {
	type fields struct {
		retryer retryer
	}

	type args struct {
		f RetryableFunc
	}

	count := 0

	tests := []struct {
		name          string
		fields        fields
		args          args
		expectedCount int
		wantErr       bool
	}{
		{
			name: "test expected number of retries",
			fields: fields{
				retryer: retryer{
					duration: 2500 * time.Millisecond,
					options:  NewDefaultOptions(),
				},
			},
			args: args{
				f: func() error {
					count++
					return ExpectedError(fmt.Errorf("expected"))
				},
			},
			expectedCount: 3,
			wantErr:       true,
		},
		{
			name: "test expected number of retries with units",
			fields: fields{
				retryer: retryer{
					duration: 2250 * time.Millisecond,
					options:  NewDefaultOptions(WithUnits(500 * time.Millisecond)),
				},
			},
			args: args{
				f: func() error {
					count++
					return ExpectedError(fmt.Errorf("expected"))
				},
			},
			expectedCount: 5,
			wantErr:       true,
		},
		{
			name: "test unexpected error",
			fields: fields{
				retryer: retryer{
					duration: 2 * time.Second,
					options:  NewDefaultOptions(),
				},
			},
			args: args{
				f: func() error {
					count++
					return UnexpectedError(fmt.Errorf("unexpected"))
				},
			},
			expectedCount: 1,
			wantErr:       true,
		},
		{
			name: "test conditional unexpected error",
			fields: fields{
				retryer: retryer{
					duration: 10 * time.Second,
					options:  NewDefaultOptions(),
				},
			},
			args: args{
				f: func() error {
					count++
					if count == 2 {
						return UnexpectedError(fmt.Errorf("unexpected"))
					}
					return ExpectedError(fmt.Errorf("unexpected"))
				},
			},
			expectedCount: 2,
			wantErr:       true,
		},
		{
			name: "test conditional no error",
			fields: fields{
				retryer: retryer{
					duration: 10 * time.Second,
					options:  NewDefaultOptions(),
				},
			},
			args: args{
				f: func() error {
					count++
					if count == 2 {
						return nil
					}
					return ExpectedError(fmt.Errorf("unexpected"))
				},
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "no error",
			fields: fields{
				retryer: retryer{
					duration: 1 * time.Second,
					options:  NewDefaultOptions(),
				},
			},
			args: args{
				f: func() error {
					return nil
				},
			},
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name: "test timeout",
			fields: fields{
				retryer: retryer{
					duration: 1 * time.Second,
					options:  NewDefaultOptions(WithUnits(10 * time.Second)),
				},
			},
			args: args{
				f: func() error {
					count++
					return ExpectedError(fmt.Errorf("expected"))
				},
			},
			expectedCount: 1,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := constantRetryer{
				retryer: tt.fields.retryer,
			}
			count = 0
			if err := e.Retry(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("constantRetryer.Retry() error = %v, wantErr %v", err, tt.wantErr)
			}
			if count != tt.expectedCount {
				t.Errorf("expected count of %d, got %d", tt.expectedCount, count)
			}
		})
	}
}
