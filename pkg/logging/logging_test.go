package logging

import (
	"fmt"
	"reflect"
	"testing"
)

func Test_UnitExtractFromArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantArgs       []string
		wantLoggerType string
	}{
		{
			name:           "Test 1",
			args:           []string{},
			wantArgs:       []string{},
			wantLoggerType: "*os.File",
		},
		{
			name:           "Test 2",
			args:           []string{"log-file=/dev/null"},
			wantArgs:       []string{},
			wantLoggerType: "*lumberjack.Logger",
		},
		{
			name:           "Test 3",
			args:           []string{"logtostderr=false"},
			wantArgs:       []string{},
			wantLoggerType: "io.discard",
		},
		{
			name:           "Test 4",
			args:           []string{"logtostderr"},
			wantArgs:       []string{},
			wantLoggerType: "*os.File",
		},
		{
			name:           "Test 5",
			args:           []string{"log-file=/dev/null", "alsologtostderr"},
			wantArgs:       []string{},
			wantLoggerType: "*io.multiWriter",
		},
		{
			name:           "Test 6",
			args:           []string{"v=6", "logtostderr=false", "one-output=true", "address=0.0.0.0", "anonymous-auth"},
			wantArgs:       []string{"v=6", "address=0.0.0.0", "anonymous-auth"},
			wantLoggerType: "io.discard",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotLogger := ExtractFromArgs(tt.args)
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("ExtractFromArgs() gotArgs = %+v\nWant = %+v", gotArgs, tt.wantArgs)
			}
			if gotLoggerType := fmt.Sprintf("%T", gotLogger); gotLoggerType != tt.wantLoggerType {
				t.Errorf("ExtractFromArgs() gotLogger = %+v\nWant = %+v", gotLoggerType, tt.wantLoggerType)
			}
		})
	}
}
