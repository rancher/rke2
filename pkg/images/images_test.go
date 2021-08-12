package images

import (
	"io/ioutil"
	"os"
	"testing"
)

var testDefaultImages Images

func Test_Unitoverride(t *testing.T) {
	type args struct {
		defaultValue  string
		overrideValue string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "override has only spaces",
			args: args{
				defaultValue:  "defaultCase",
				overrideValue: "    ",
			},

			want: "defaultCase",
		},
		{
			name: "override has extra spaces",
			args: args{
				defaultValue:  "defaultCase",
				overrideValue: " caseWithSpaces   ",
			},

			want: "caseWithSpaces",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := override(tt.args.defaultValue, tt.args.overrideValue); got != tt.want {
				t.Errorf("override() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitPull(t *testing.T) {
	type args struct {
		dir   string
		name  string
		image string
	}
	tests := []struct {
		name        string
		args        args
		setup       func(a *args) error
		teardown    func(a *args) error
		wantTxtFile bool
		wantErr     bool
	}{
		{
			name: "Pull with no directory",
			args: args{
				name:  "kube-scheduler",
				image: testDefaultImages.KubeScheduler,
			},
			setup:    func(a *args) error { return nil },
			teardown: func(a *args) error { return nil },
		},
		{
			name: "Pull with nonexistent directory",
			args: args{
				dir: "/tmp/DEADBEEF",
			},
			setup: func(a *args) error { return nil },
			teardown: func(a *args) error {
				return os.RemoveAll(a.dir)
			},
		},
		{
			name: "Pull with no image in directory",
			args: args{
				name:  "kube-scheduler",
				image: testDefaultImages.KubeScheduler,
			},
			setup: func(a *args) error {
				var err error
				a.dir, err = os.MkdirTemp("", "*")
				return err
			},
			teardown: func(a *args) error {
				return os.RemoveAll(a.dir)
			},

			wantTxtFile: true,
		},
		{
			name: "Pull with fake image in directory",
			args: args{
				name:  "kube-scheduler",
				image: testDefaultImages.KubeScheduler,
			},
			setup: func(a *args) error {
				var err error
				a.dir, err = os.MkdirTemp("", "*")
				tempImage := a.dir + "/" + a.name + ".image"
				ioutil.WriteFile(tempImage, []byte(a.image+"\n"), 0644)
				return err
			},
			teardown: func(a *args) error {
				return os.RemoveAll(a.dir)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := tt.setup(&tt.args); err != nil {
				t.Errorf("Setup for Pull() failed = %v", err)
			}
			if err := Pull(tt.args.dir, tt.args.name, tt.args.image); (err != nil) != tt.wantErr {
				t.Errorf("Pull() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTxtFile {
				fileName := tt.args.name + ".txt"
				if _, err := os.Stat(tt.args.dir + "/" + fileName); os.IsNotExist(err) {
					t.Errorf("File generate by Pull() %s, does not exists, wantFile %v", fileName, tt.wantTxtFile)
				}
			}
			if err := tt.teardown(&tt.args); err != nil {
				t.Errorf("Teardown for Pull() failed = %v", err)
			}
		})
	}
}
