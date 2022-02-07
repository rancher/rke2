package tests

import (
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/util"
)

var serverArgs = []string{"server"}

var _ = Describe("etcd snapshots", func() {
	BeforeEach(func() {
		if !util.ServerArgsPresent(serverArgs) {
			Skip("Test needs rke2 server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("a new etcd is created", func() {
		It("starts up with no problems", func() {
			Eventually(util.Rke2Ready(), "90s", "1s").Should(BeTrue())
		})
		It("saves an etcd snapshot", func() {
			Expect(util.Rke2Cmd("etcd-snapshot", "save")).
				To(ContainSubstring("Saving current etcd snapshot set to rke2-etcd-snapshots"))
		})
		It("list snapshots", func() {
			Expect(util.Rke2Cmd("etcd-snapshot", "ls")).
				To(MatchRegexp(`:///var/lib/rancher/rke2/server/db/snapshots/on-demand`))
		})
		It("deletes a snapshot", func() {
			lsResult, err := util.Rke2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`on-demand[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(util.Rke2Cmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("saving a custom name", func() {
		It("saves an etcd snapshot with a custom name", func() {
			Expect(util.Rke2Cmd("etcd-snapshot", "save", "--name", "ALIVEBEEF")).
				To(ContainSubstring("Saving etcd snapshot to /var/lib/rancher/rke2/server/db/snapshots/ALIVEBEEF"))
		})
		It("deletes that snapshot", func() {
			lsResult, err := util.Rke2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`ALIVEBEEF[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(util.Rke2Cmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("using etcd snapshot prune", func() {
		It("saves 3 different snapshots", func() {
			Expect(util.Rke2Cmd("etcd-snapshot", "save", "--name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to rke2-etcd-snapshots"))
			time.Sleep(1 * time.Second)
			Expect(util.Rke2Cmd("etcd-snapshot", "save", "--name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to rke2-etcd-snapshots"))
			time.Sleep(1 * time.Second)
			Expect(util.Rke2Cmd("etcd-snapshot", "save", "--name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to rke2-etcd-snapshots"))
			time.Sleep(1 * time.Second)
		})
		It("lists all 3 snapshots", func() {
			lsResult, err := util.Rke2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`:///var/lib/rancher/rke2/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(3))
		})
		It("prunes snapshots down to 2", func() {
			Expect(util.Rke2Cmd("etcd-snapshot", "prune", "--snapshot-retention", "2", "--name", "PRUNE_TEST")).
				To(BeEmpty())
			lsResult, err := util.Rke2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`:///var/lib/rancher/rke2/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(2))
		})
		It("cleans up remaining snapshots", func() {
			lsResult, err := util.Rke2Cmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`PRUNE_TEST[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			for _, snapshotName := range reg.FindAllString(lsResult, -1) {
				Expect(util.Rke2Cmd("etcd-snapshot", "delete", snapshotName)).
					To(ContainSubstring("Removing the given locally stored etcd snapshot"))
			}
		})
	})
})

func Test_IntegrationEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Suite")
}
