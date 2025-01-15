package etcdsnapshot

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/rke2/tests/integration"
	"github.com/sirupsen/logrus"
)

var serverArgs = []string{""}
var serverLog *os.File
var testLock int

var _ = BeforeSuite(func() {
	var err error
	testLock, err = testutil.AcquireTestLock()
	Expect(err).ToNot(HaveOccurred())
	serverLog, err = testutil.StartServer(serverArgs...)
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("etcd snapshots", Ordered, func() {
	When("a new etcd snapshot is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				err := testutil.ServerReady()
				if err != nil {
					logrus.Info(err)
				}
				return err
			}, "240s", "15s").Should(Succeed())
		})
		It("saves an etcd snapshot", func() {
			Expect(testutil.RKE2Cmd("etcd-snapshot", "save")).
				To(And(ContainSubstring("Snapshot on-demand-"), ContainSubstring(" saved.")))
		})
		It("list snapshots", func() {
			Expect(testutil.RKE2Cmd("etcd-snapshot", "ls")).
				To(MatchRegexp(`:///var/lib/rancher/rke2/server/db/snapshots/on-demand`))
		})
		It("deletes a snapshot", func() {
			lsResult, err := testutil.RKE2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`on-demand[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(testutil.RKE2Cmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring(fmt.Sprintf("Snapshot %s deleted", snapshotName)))
		})
	})
	When("saving a custom name", func() {
		It("saves an etcd snapshot with a custom name", func() {
			Expect(testutil.RKE2Cmd("etcd-snapshot", "save", "--name", "ALIVEBEEF")).
				To(And(ContainSubstring("Snapshot ALIVEBEEF-"), ContainSubstring(" saved.")))
		})
		It("deletes that snapshot", func() {
			lsResult, err := testutil.RKE2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`ALIVEBEEF[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(testutil.RKE2Cmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Snapshot %s deleted.", snapshotName))
		})
	})
	When("using etcd snapshot prune", func() {
		It("saves 3 different snapshots", func() {
			Expect(testutil.RKE2Cmd("etcd-snapshot", "save", "--name", "PRUNE_TEST")).
				To(And(ContainSubstring("Snapshot PRUNE_TEST-"), ContainSubstring(" saved.")))
			time.Sleep(1 * time.Second)
			Expect(testutil.RKE2Cmd("etcd-snapshot", "save", "--name", "PRUNE_TEST")).
				To(And(ContainSubstring("Snapshot PRUNE_TEST-"), ContainSubstring(" saved.")))
			time.Sleep(1 * time.Second)
			Expect(testutil.RKE2Cmd("etcd-snapshot", "save", "--name", "PRUNE_TEST")).
				To(And(ContainSubstring("Snapshot PRUNE_TEST-"), ContainSubstring(" saved.")))
			time.Sleep(1 * time.Second)
		})
		It("lists all 3 snapshots", func() {
			lsResult, err := testutil.RKE2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`:///var/lib/rancher/rke2/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(3))
		})
		It("prunes snapshots down to 2", func() {
			Expect(testutil.RKE2Cmd("etcd-snapshot", "prune", "--snapshot-retention", "2", "--name", "PRUNE_TEST")).
				To(ContainSubstring(" deleted."))
			lsResult, err := testutil.RKE2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`:///var/lib/rancher/rke2/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(2))
		})
		It("cleans up remaining snapshots", func() {
			Eventually(func(g Gomega) {
				lsResult, err := testutil.RKE2Cmd("etcd-snapshot", "ls")
				g.Expect(err).ToNot(HaveOccurred())
				reg, err := regexp.Compile(`PRUNE_TEST[^\s]+`)
				g.Expect(err).ToNot(HaveOccurred())
				for _, snapshotName := range reg.FindAllString(lsResult, -1) {
					g.Expect(testutil.RKE2Cmd("etcd-snapshot", "delete", snapshotName)).
						To(ContainSubstring("Snapshot %s deleted.", snapshotName))
				}
			}, "20s", "5s").Should(Succeed())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		testutil.SaveLog(serverLog, false)
		serverLog = nil
	}
	Expect(testutil.KillServer(serverLog)).To(Succeed())
	Expect(testutil.Cleanup(testLock)).To(Succeed())
})

func Test_IntegrationEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Suite")
}
