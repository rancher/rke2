package images

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func Pull(dir, name, image string) error {
	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	dest := filepath.Join(dir, name+".txt")
	if err := ioutil.WriteFile(dest, []byte(image+"\n"), 0644); err != nil {
		return err
	}

	return nil
}
