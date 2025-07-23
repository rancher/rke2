package podtemplate

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/yaml"
)

// OnlyExisting filters out paths from the list that cannot be accessed
func OnlyExisting(paths []string) []string {
	existing := []string{}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

// After calls a function after a message is received from a channel.
// If the function returns an error, a fatal error is logged.
func After(after <-chan struct{}, f func() error) error {
	go func() {
		<-after
		if err := f(); err != nil {
			logrus.Fatal(err)
		}
	}()
	return nil
}

func WriteDefaultPolicyFile(policyFilePath string) error {
	auditPolicy := auditv1.Policy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Policy",
			APIVersion: "audit.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Rules: []auditv1.PolicyRule{
			{
				Level: "None",
			},
		},
	}
	bytes, err := yaml.Marshal(auditPolicy)
	if err != nil {
		return err
	}
	return writeIfNotExists(policyFilePath, bytes)
}

// writeIfNotExists writes content to a file at a given path, but only if the file does not already exist
func writeIfNotExists(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	_, err = file.Write(content)
	return err
}
