package startup

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/rke2/tests/integration"
	"github.com/sirupsen/logrus"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

var (
	serverLog  *os.File
	serverArgs = []string{"--debug"}
	testLock   int
)

var _ = BeforeSuite(func() {
	var err error
	testLock, err = testutil.AcquireTestLock()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("startup tests", Ordered, func() {
	When("a default server is created", func() {
		It("starts successfully", func() {
			var err error
			serverLog, err = testutil.StartServer(serverArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default components deployed", func() {
			Eventually(func() error {
				err := testutil.ServerReady()
				if err != nil {
					logrus.Info(err)
				}
				return err
			}, "240s", "15s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.KillServer(serverLog)).To(Succeed())
			Expect(testutil.Cleanup(testLock)).To(Succeed())
		})
	})
	When("a server is created with bind-address", func() {
		It("starts successfully", func() {
			hostIP, _ := utilnet.ChooseHostInterface()
			var err error
			serverLog, err = testutil.StartServer(append(serverArgs, "--bind-address", hostIP.String())...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default components deployed", func() {
			Eventually(func() error {
				err := testutil.ServerReady()
				if err != nil {
					logrus.Info(err)
				}
				return err
			}, "240s", "15s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.KillServer(serverLog)).To(Succeed())
			Expect(testutil.Cleanup(testLock)).To(Succeed())
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

func Test_IntegrationStartup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startup Suite")
}
