package rke2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	toolscache "k8s.io/client-go/tools/cache"
)

// Test_UnitWaitForSelfDelete exercises self-delete cleanup through the Node
// watch, the way the control-plane startup hook runs it.
// Each case replays an ordered sequence of Node events.
func Test_UnitWaitForSelfDeleteWatcher(t *testing.T) {
	// The fake client does not honor deletion propagation policies, so a Node
	// deletion is modeled as the object carrying a deletionTimestamp (plus a
	// finalizer), which the watch observes as a single event.
	deletionFinalizer := "etcd." + version.Program + ".cattle.io/deletion-test"
	// this annotation refers to the deletion request
	requestAnnotation := "etcd." + version.Program + ".cattle.io/remove"

	otherRuntime := "example"

	type nodeStep struct {
		annotations map[string]string
		deleting    bool
	}

	tests := []struct {
		name        string
		steps       []nodeStep // each event
		wantTrigger bool       // whether the deletion should happen or not
		wantRV      int        // 1-based index of the triggering step
		localName   string
	}{
		// Triggers: the wait must complete on the listed event, no earlier.
		{
			name:        "initial node with matching acknowledgement",
			localName:   "current-member",
			steps:       []nodeStep{{annotations: map[string]string{removedNodeNameAnnotation: "current-member"}}},
			wantTrigger: true,
			wantRV:      1,
		},
		{
			name:      "request then matching acknowledgement",
			localName: "current-member",
			steps: []nodeStep{
				{annotations: map[string]string{requestAnnotation: "true"}},
				{annotations: map[string]string{removedNodeNameAnnotation: "current-member"}},
			},
			wantTrigger: true,
			wantRV:      2,
		},
		{
			name:      "stale acknowledgement then matching acknowledgement",
			localName: "current-member",
			steps: []nodeStep{
				{annotations: map[string]string{requestAnnotation: "true"}},
				{annotations: map[string]string{requestAnnotation: "true", removedNodeNameAnnotation: "old-member"}},
				{annotations: map[string]string{removedNodeNameAnnotation: "current-member"}},
			},
			wantTrigger: true,
			wantRV:      3,
		},
		{
			name:      "member name file with trailing newline",
			localName: "current-member\n",
			steps: []nodeStep{
				{annotations: map[string]string{removedNodeNameAnnotation: "current-member"}},
			},
			wantTrigger: true,
			wantRV:      1,
		},
		{
			name:      "node deletion without local member",
			localName: "", // missing name!
			steps: []nodeStep{
				{deleting: true, annotations: map[string]string{requestAnnotation: "true"}},
			},
			wantTrigger: true,
			wantRV:      1,
		},
		{
			name:        "node deletion despite stale acknowledgement",
			localName:   "current-member",
			steps:       []nodeStep{{deleting: true, annotations: map[string]string{removedNodeNameAnnotation: "old-member"}}},
			wantTrigger: true,
			wantRV:      1,
		},
		// Non-triggers: the wait must stay silent until the test context ends (deliveryMargin).
		// This is not ideal, but should not cause problems.
		{
			name:      "request only",
			localName: "current-member",
			steps:     []nodeStep{{annotations: map[string]string{requestAnnotation: "true"}}},
		},
		{
			name:      "acknowledgement without local member",
			localName: "", // missing name!
			steps:     []nodeStep{{annotations: map[string]string{removedNodeNameAnnotation: "current-member"}}},
		},
		{
			name:      "no annotations without local member",
			localName: "", // missing name!
			steps:     []nodeStep{{}},
		},
		{
			name:      "stale acknowledgement",
			localName: "current-member",
			steps:     []nodeStep{{annotations: map[string]string{removedNodeNameAnnotation: "old-member"}}},
		},
		{
			name:      "node hostname is not the member identity",
			localName: "current-member",
			steps:     []nodeStep{{annotations: map[string]string{removedNodeNameAnnotation: "node-1"}}},
		},
		{
			name:      "acknowledgement for another runtime",
			localName: "current-member",
			steps:     []nodeStep{{annotations: map[string]string{"etcd." + otherRuntime + ".cattle.io/removed-node-name": "current-member"}}},
		},
	}

	// delivery margin for the informer to deliver a step's event
	// used for the non-trigger tests.
	const deliveryMargin = 200 * time.Millisecond

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()

			// validate cases with missing names
			if tt.localName != "" {
				writeTestEtcdMemberName(t, dataDir, tt.localName)
			}

			// Fake API server with clean state
			cs := fake.NewSimpleClientset()

			watchStarted := make(chan struct{})
			var watchStartedOnce sync.Once

			// ListWatch using the fake client calls
			lw := &toolscache.ListWatch{
				ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
					return cs.CoreV1().Nodes().List(t.Context(), opts)
				},
				WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
					w, err := cs.CoreV1().Nodes().Watch(t.Context(), opts)
					if err == nil {
						// inform the watch has started
						watchStartedOnce.Do(func() { close(watchStarted) })
					}
					return w, err
				},
			}

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			type result struct {
				event *watch.Event
				err   error
			}

			done := make(chan result, 1)
			go func() {
				event, err := waitForSelfDelete(ctx, dataDir,
					toolscache.ToListWatcherWithWatchListSemantics(lw, cs),
				)
				done <- result{event, err}
			}()

			// wait for the watcher to start, or capture a failure
			select {
			case <-watchStarted:
				// The initial list is complete and the watch can receive all Node events.
			case got := <-done:
				t.Fatalf("wait returned before watch registration: event=%#v, err=%v", got.event, got.err)
			case <-ctx.Done():
				t.Fatal("Node watch did not start")
			}

			nodes := cs.CoreV1().Nodes()

			// Replay the steps in order. The fake tracker does not rewrite
			// object resource versions, so step i is observed by the watch
			// with the resource version i+1 assigned here.
			var node *v1.Node
			for i, step := range tt.steps {
				rv := fmt.Sprint(i + 1)
				if node == nil {
					node = &v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "node-1",
							ResourceVersion: rv,
							Annotations:     step.annotations,
						},
					}
					if step.deleting {
						now := metav1.Now()
						node.DeletionTimestamp = &now
						node.Finalizers = []string{deletionFinalizer}
					}
					created, err := nodes.Create(ctx, node, metav1.CreateOptions{})
					if err != nil {
						t.Fatalf("creating the node at step %s: %v", rv, err)
					}
					node = created
				} else {
					node = node.DeepCopy()
					node.ResourceVersion = rv
					node.Annotations = step.annotations
					if step.deleting {
						now := metav1.Now()
						node.DeletionTimestamp = &now
						node.Finalizers = []string{deletionFinalizer}
					}
					updated, err := nodes.Update(ctx, node, metav1.UpdateOptions{})
					if err != nil {
						t.Fatalf("updating the node at step %s: %v", rv, err)
					}
					node = updated
				}

				// The wait must not complete after a step that does not
				// satisfy the cleanup condition; allow the informer to
				// deliver the event first.
				if !tt.wantTrigger || i+1 != tt.wantRV {
					select {
					case got := <-done:
						t.Fatalf("wait completed at step %s, expected step %d: event=%#v, err=%v", rv, tt.wantRV, got.event, got.err)
					case <-time.After(deliveryMargin):
					}
				}
			}

			if tt.wantTrigger {
				select {
				case got := <-done:
					if got.err != nil {
						t.Fatal("wait returned an error: ", got.err)
					}
					if got.event == nil {
						t.Fatal("wait completed without a Node event")
					}
					node, ok := got.event.Object.(*v1.Node)
					if !ok || node.ResourceVersion != fmt.Sprint(tt.wantRV) {
						t.Fatalf("wait completed before the matching acknowledgement: %#v", got.event)
					}
					wantKind := watch.Added
					if tt.wantRV > 1 {
						wantKind = watch.Modified
					}
					if got.event.Type != wantKind {
						t.Fatalf("wait completed on an event of kind %v, want %v", got.event.Type, wantKind)
					}
				case <-ctx.Done():
					t.Fatal("wait did not complete after the triggering Node event")
				}
			} else {
				select {
				case got := <-done:
					t.Fatalf("wait completed although no step should trigger cleanup: event=%#v, err=%v", got.event, got.err)
				case <-time.After(deliveryMargin):
				}
			}
		})
	}
}

func writeTestEtcdMemberName(t *testing.T, dataDir, name string) {
	t.Helper()
	path := etcdNameFile(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}
}
