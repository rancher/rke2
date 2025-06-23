package defaults

import (
	"reflect"
	"testing"

	"github.com/urfave/cli/v2"
)

func Test_UnitPrependToStringSlice(t *testing.T) {
	type args struct {
		orig    []string
		prepend []string
	}
	tests := []struct {
		name     string
		args     args
		expected []string
	}{
		{
			name: "prepend to non-empty",
			args: args{
				orig:    []string{"b", "c"},
				prepend: []string{"a"},
			},
			expected: []string{"a", "b", "c"},
		},
		{
			name: "prepend to empty",
			args: args{
				orig:    []string{},
				prepend: []string{"x", "y"},
			},
			expected: []string{"x", "y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := cli.NewStringSlice(tt.args.orig...)
			got := PrependToStringSlice(*orig, tt.args.prepend)
			if !reflect.DeepEqual(got.Value(), tt.expected) {
				t.Errorf("PrependToStringSlice() = %v, want %v", got.Value(), tt.expected)
			}
		})
	}
}
