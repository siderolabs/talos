// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"errors"
	"testing"
	"time"
)

// nolint: scopelint
func Test_retry(t *testing.T) {
	type args struct {
		f RetryableFunc
		d time.Duration
		t Ticker
	}

	tests := []struct {
		name       string
		args       args
		wantString string
	}{
		{
			name: "expected error string",
			args: args{
				f: func() error { return ExpectedError(errors.New("test")) },
				d: 2 * time.Second,
				t: NewConstantTicker(NewDefaultOptions()),
			},
			wantString: "2 error(s) occurred:\n\ttest\n\ttimeout",
		},
		{
			name: "unexpected error string",
			args: args{
				f: func() error { return UnexpectedError(errors.New("test")) },
				d: 2 * time.Second,
				t: NewConstantTicker(NewDefaultOptions()),
			},
			wantString: "1 error(s) occurred:\n\ttest",
		},
		{
			name: "no error string",
			args: args{
				f: func() error { return nil },
				d: 2 * time.Second,
				t: NewConstantTicker(NewDefaultOptions()),
			},
			wantString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := retry(tt.args.f, tt.args.d, tt.args.t); err != nil && tt.wantString != err.Error() {
				t.Errorf("retry() error = %q\nwant:\n%q", err, tt.wantString)
			}
		})
	}
}
