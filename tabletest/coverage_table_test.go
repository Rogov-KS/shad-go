//go:build !change

package tabletest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	for _, tc := range []struct {
		expected time.Duration
		input    string
		isErr    bool
	}{
		// base
		{expected: 300 * time.Millisecond, input: "300ms", isErr: false},
		{expected: -3 * time.Hour / 2, input: "-1.5h", isErr: false},
		{expected: 165 * time.Minute, input: "2h45m", isErr: false},
		{expected: 165*time.Minute + 30*time.Second, input: "2h45m30s", isErr: false},
		{expected: 166 * time.Minute, input: "2h45m60s", isErr: false},

		// empty
		{expected: time.Second, input: "", isErr: true},
		{expected: 0 * time.Second, input: "0", isErr: false},

		// errors
		{expected: 0 * time.Second, input: "abc12", isErr: true},
		{expected: time.Second, input: "1.0", isErr: true},
		{expected: time.Second, input: "1", isErr: true},
		{expected: time.Second, input: ".s", isErr: true},
		{expected: time.Second, input: "-.s", isErr: true},
		{expected: time.Second, input: "1.6cs", isErr: true},

		// overflow
		{expected: time.Second, input: "9223372036854775811h", isErr: true},
		{expected: time.Second, input: "9223372036854775809h", isErr: true},
		{expected: 1922337203 * time.Nanosecond, input: "1.9223372036899999999999s", isErr: false},
		{expected: 1922337203 * time.Nanosecond, input: "1.9223372036854775809s", isErr: false},
		{expected: time.Second, input: "9999999999h", isErr: true},
		{expected: time.Second, input: "2562047.9999999h", isErr: true},
		{expected: time.Second, input: "2562047h59m", isErr: true},

		// usless below:
		{expected: 11 * time.Nanosecond, input: "11ns", isErr: false},
		{expected: 111 * time.Microsecond, input: "111us", isErr: false},
		{expected: 112 * time.Microsecond, input: "112µs", isErr: false},
		{expected: 5 * time.Millisecond, input: "5ms", isErr: false},
		{expected: 6 * time.Second, input: "6s", isErr: false},
		{expected: 7 * time.Minute, input: "7m", isErr: false},
		{expected: 8 * time.Hour, input: "8h", isErr: false},
	} {
		t.Run(fmt.Sprintf("%s", tc.input), func(t *testing.T) {
			actual, err := ParseDuration(tc.input)
			if tc.isErr {
				require.Error(t, err)
			} else {
				require.Equal(t, tc.expected, actual)
				require.NoError(t, err)
			}
		})
	}

}
