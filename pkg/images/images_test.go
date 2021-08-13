package images

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func Test_UnitPull(t *testing.T) {
	type args struct {
		dir   string
		name  string
		image name.Reference
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
				name: KubeScheduler,
			},
			setup:    func(a *args) error { return nil },
			teardown: func(a *args) error { return nil },
		},
		{
			name: "Pull with nonexistent directory",
			args: args{
				dir:  "/tmp/DEADBEEF",
				name: KubeScheduler,
			},
			setup: func(a *args) error {
				var err error
				a.image, err = getDefaultImage(KubeScheduler)
				return err
			},
			teardown: func(a *args) error {
				return os.RemoveAll(a.dir)
			},

			wantTxtFile: true,
		},
		{
			name: "Pull with no image in directory",
			args: args{
				name: KubeScheduler,
			},
			setup: func(a *args) error {
				var err error
				a.image, err = getDefaultImage(KubeScheduler)
				if err != nil {
					return err
				}
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
				name: "kube-scheduler",
			},
			setup: func(a *args) error {
				var err error
				a.image, err = getDefaultImage(KubeScheduler)
				if err != nil {
					return err
				}
				a.dir, err = os.MkdirTemp("", "*")
				tempImage := a.dir + "/" + a.name + ".image"
				ioutil.WriteFile(tempImage, []byte(a.image.Name()+"\n"), 0644)
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
