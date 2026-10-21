package rke2

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/executor/staticpod"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	toolscache "k8s.io/client-go/tools/cache"
)

func Test_UnitShouldCleanupStaticPods(t *testing.T) {
	const memberName = "node-1-current-member"
	requestAnnotation := "etcd." + version.Program + ".cattle.io/remove"
	otherRuntime := "k3s"
	if version.Program == otherRuntime {
		otherRuntime = "rke2"
	}
	tests := []struct {
		name        string
		annotations map[string]string
		deleting    bool
		localName   string
		missingName bool
		invalidName bool
		want        bool
		wantErr     bool
	}{
		{name: "no annotations", missingName: true},
		{name: "request only", annotations: map[string]string{requestAnnotation: "true"}, localName: memberName},
		{name: "request without local member", annotations: map[string]string{requestAnnotation: "true"}, missingName: true},
		{name: "empty acknowledgement", annotations: map[string]string{removedNodeNameAnnotation: ""}, localName: memberName},
		{name: "matching acknowledgement without request", annotations: map[string]string{removedNodeNameAnnotation: memberName}, localName: memberName, want: true},
		{name: "matching acknowledgement with request", annotations: map[string]string{removedNodeNameAnnotation: memberName, requestAnnotation: "true"}, localName: memberName, want: true},
		{name: "stale acknowledgement", annotations: map[string]string{removedNodeNameAnnotation: "node-1-old-member", requestAnnotation: "true"}, localName: memberName},
		{name: "node hostname is not member identity", annotations: map[string]string{removedNodeNameAnnotation: "node-1"}, localName: memberName},
		{name: "other runtime acknowledgement", annotations: map[string]string{"etcd." + otherRuntime + ".cattle.io/removed-node-name": memberName}, localName: memberName},
		{name: "member name file newline", annotations: map[string]string{removedNodeNameAnnotation: memberName}, localName: memberName + "\n", want: true},
		{name: "empty member name file", annotations: map[string]string{removedNodeNameAnnotation: memberName}},
		{name: "whitespace member name file", annotations: map[string]string{removedNodeNameAnnotation: memberName}, localName: " \n"},
		{name: "missing member name file", annotations: map[string]string{removedNodeNameAnnotation: memberName}, missingName: true},
		{name: "unreadable member name fails closed", annotations: map[string]string{removedNodeNameAnnotation: memberName}, invalidName: true, wantErr: true},
		{name: "Node deletion without local member", deleting: true, missingName: true, want: true},
		{name: "Node deletion with request", deleting: true, annotations: map[string]string{requestAnnotation: "true"}, localName: memberName, want: true},
		{name: "Node deletion with stale acknowledgement", deleting: true, annotations: map[string]string{removedNodeNameAnnotation: "old-member"}, localName: memberName, want: true},
		{name: "Node deletion does not read member file", deleting: true, invalidName: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			if tt.invalidName {
				// A directory reliably makes ReadFile fail, even when tests run as root.
				if err := os.MkdirAll(etcdNameFile(dataDir), 0700); err != nil {
					t.Fatal(err)
				}
			} else if !tt.missingName {
				writeTestEtcdMemberName(t, dataDir, tt.localName)
			}
			node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Annotations: tt.annotations}}
			if tt.deleting {
				now := metav1.Now()
				node.DeletionTimestamp = &now
			}
			got, err := shouldCleanupStaticPods(node, dataDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("shouldCleanupStaticPods() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("shouldCleanupStaticPods() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_UnitShouldCleanupStaticPodsRejoinedMember(t *testing.T) {
	dataDir := t.TempDir()
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{removedNodeNameAnnotation: "old-member"}}}
	writeTestEtcdMemberName(t, dataDir, "old-member")
	if got, err := shouldCleanupStaticPods(node, dataDir); err != nil || !got {
		t.Fatalf("matching old member: cleanup = %v, err = %v", got, err)
	}
	writeTestEtcdMemberName(t, dataDir, "new-member")
	if got, err := shouldCleanupStaticPods(node, dataDir); err != nil || got {
		t.Fatalf("rejoined member with stale acknowledgement: cleanup = %v, err = %v", got, err)
	}
}

func Test_UnitWaitForSelfDelete(t *testing.T) {
	dataDir := t.TempDir()
	writeTestEtcdMemberName(t, dataDir, "current-member")
	request := &v1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: "node-1", ResourceVersion: "1",
		Annotations: map[string]string{"etcd." + version.Program + ".cattle.io/remove": "true"},
	}}
	stream := watch.NewRaceFreeFake()
	watchStarted := make(chan metav1.ListOptions, 1)
	lw := &toolscache.ListWatch{
		ListFunc: func(metav1.ListOptions) (runtime.Object, error) {
			return &v1.NodeList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}, Items: []v1.Node{*request.DeepCopy()}}, nil
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			watchStarted <- options
			return stream, nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type result struct {
		event *watch.Event
		err   error
	}
	done := make(chan result, 1)
	go func() {
		event, err := waitForSelfDelete(ctx, dataDir, lw)
		done <- result{event, err}
	}()
	select {
	case options := <-watchStarted:
		if options.SendInitialEvents != nil && *options.SendInitialEvents {
			// Newer clients use a streaming initial list and require its
			// end-of-initial-events bookmark before forwarding watch events.
			stream.Add(request.DeepCopy())
			stream.Action(watch.Bookmark, &v1.Node{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "1",
				Annotations:     map[string]string{metav1.InitialEventsAnnotationKey: "true"},
			}})
		}
	case <-ctx.Done():
		t.Fatal("Node watch did not start")
	}

	// Removal may remain requested or carry a stale acknowledgement while the
	// controller is working. Neither should complete the wait.
	stale := request.DeepCopy()
	stale.ResourceVersion = "2"
	stale.Annotations[removedNodeNameAnnotation] = "old-member"
	stream.Modify(stale)
	completed := stale.DeepCopy()
	completed.ResourceVersion = "3"
	delete(completed.Annotations, "etcd."+version.Program+".cattle.io/remove")
	completed.Annotations[removedNodeNameAnnotation] = "current-member"
	stream.Modify(completed)
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.event == nil {
			t.Fatal("wait completed without a Node event")
		}
		node, ok := got.event.Object.(*v1.Node)
		if !ok || node.ResourceVersion != "3" {
			t.Fatalf("wait completed before the matching acknowledgement: %#v", got.event)
		}
	case <-ctx.Done():
		t.Fatal("wait did not complete after member removal acknowledgement")
	}
}

func Test_UnitWaitForSelfDeleteCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lw := &toolscache.ListWatch{
		ListFunc:  func(metav1.ListOptions) (runtime.Object, error) { return &v1.NodeList{}, nil },
		WatchFunc: func(metav1.ListOptions) (watch.Interface, error) { return watch.NewRaceFreeFake(), nil },
	}
	if _, err := waitForSelfDelete(ctx, t.TempDir(), lw); err == nil {
		t.Fatal("cancelled watch must not authorize cleanup")
	}
}

func Test_UnitCleanupStaticPods(t *testing.T) {
	dataDir := t.TempDir()
	manifestDir := staticpod.PodManifestsDir(dataDir)
	if err := os.MkdirAll(manifestDir, 0700); err != nil {
		t.Fatal(err)
	}
	components := []string{"kube-apiserver", "kube-scheduler", "kube-controller-manager", "cloud-controller-manager", "etcd"}
	for _, component := range append(append([]string{}, components...), "unrelated") {
		if err := os.WriteFile(filepath.Join(manifestDir, component+".yaml"), []byte("test manifest\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := cleanupStaticPods(dataDir); err != nil {
			t.Fatal(err)
		}
	}
	for _, component := range components {
		if _, err := os.Stat(filepath.Join(manifestDir, component+".yaml")); !os.IsNotExist(err) {
			t.Errorf("%s manifest was not removed: %v", component, err)
		}
	}
	if _, err := os.Stat(filepath.Join(manifestDir, "unrelated.yaml")); err != nil {
		t.Errorf("unrelated manifest should be preserved: %v", err)
	}
}

func writeTestEtcdMemberName(t *testing.T, dataDir, name string) {
	t.Helper()
	path := etcdNameFile(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name), 0600); err != nil {
		t.Fatal(err)
	}
}
