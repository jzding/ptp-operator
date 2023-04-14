package validation_tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"

	ptpv1 "github.com/openshift/ptp-operator/api/v1"
	testutils "github.com/openshift/ptp-operator/test/pkg"
	testclient "github.com/openshift/ptp-operator/test/pkg/client"
	"github.com/openshift/ptp-operator/test/pkg/ptphelper"
)

var _ = Describe("validation", func() {
	Context("ptp", func() {
		It("should have the ptp namespace", func() {
			_, err := testclient.Client.CoreV1().Namespaces().Get(context.Background(), testutils.PtpNamespace, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the ptp operator deployment in running state", func() {
			Eventually(func() error {
				deploy, err := testclient.Client.Deployments(testutils.PtpNamespace).Get(context.Background(), testutils.PtpOperatorDeploymentName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				pods, err := testclient.Client.CoreV1().Pods(testutils.PtpNamespace).List(context.Background(), metav1.ListOptions{
					LabelSelector: fmt.Sprintf("name=%s", testutils.PtpOperatorDeploymentName)})
				if err != nil {
					return err
				}

				if len(pods.Items) != int(deploy.Status.Replicas) {
					return fmt.Errorf("deployment %s pods are not ready, expected %d replicas got %d pods", testutils.PtpOperatorDeploymentName, deploy.Status.Replicas, len(pods.Items))
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("deployment %s pod %s is not running, expected status %s got %s", testutils.PtpOperatorDeploymentName, pod.Name, corev1.PodRunning, pod.Status.Phase)
					}
				}

				return nil
			}, testutils.TimeoutIn10Minutes, testutils.TimeoutInterval2Seconds).ShouldNot(HaveOccurred())
		})

		It("should have the linuxptp daemonset in running state", func() {
			ptphelper.WaitForPtpDaemonToBeReady()
		})

		It("should have the ptp CRDs available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: testutils.NodePtpDevicesCRD}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: testutils.PtpConfigsCRD}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: testutils.PtpOperatorConfigsCRD}, crd)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should be able to set nodeSelector", func() {
			ptpConfig, err := testclient.Client.PtpV1Interface.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Get(context.Background(), testutils.PtpConfigOperatorName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Set nodeSelector to {}")
			nodeEmpty := make(map[string]string)
			ptpConfig.Spec.DaemonNodeSelector = nodeEmpty
			_, err = testclient.Client.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Update(context.Background(), ptpConfig, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ptpConfig.Spec.DaemonNodeSelector).Should(Equal(nodeEmpty), "empty")

			By("Set nodeSelector to {foo: bar}")
			nodeSelected := map[string]string{"foo": "bar"}
			ptpConfig.Spec.DaemonNodeSelector = nodeSelected
			_, err = testclient.Client.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Update(context.Background(), ptpConfig, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			ptpConfig, err = testclient.Client.PtpV1Interface.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Get(context.Background(), testutils.PtpConfigOperatorName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ptpConfig.Spec.DaemonNodeSelector).Should(Equal(nodeSelected), "foobar")

			By("Set nodeSelector back to {}")
			ptpConfig, err = testclient.Client.PtpV1Interface.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Get(context.Background(), testutils.PtpConfigOperatorName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			ptpConfig.Spec.DaemonNodeSelector = nodeEmpty
			_, err = testclient.Client.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Update(context.Background(), ptpConfig, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ptpConfig.Spec.DaemonNodeSelector).Should(Equal(nodeEmpty), "empty")
		})

		It("should reject event config", func() {
			By("If TransportHost and storageType are not set")

			// wait for k8s to refresh
			time.Sleep(2 * time.Second)

			ptpConfig, err := testclient.Client.PtpV1Interface.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Get(context.Background(), testutils.PtpConfigOperatorName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			ptpConfig.Spec.EventConfig = &ptpv1.PtpEventConfig{
				EnableEventPublisher: true,
			}
			_, err = testclient.Client.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Update(context.Background(), ptpConfig, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
			Expect(err).Should(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("storageType must be set")))
		})

		It("should reject event config", func() {
			By("If TransportHost not set and StorageType are invalid")

			// wait for k8s to refresh
			time.Sleep(2 * time.Second)

			invalidStorageClass := "_invalid_storage_class"
			ptpConfig, err := testclient.Client.PtpV1Interface.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Get(context.Background(), testutils.PtpConfigOperatorName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			ptpConfig.Spec.EventConfig = &ptpv1.PtpEventConfig{
				EnableEventPublisher: true,
				StorageType:          invalidStorageClass,
			}
			_, err = testclient.Client.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Update(context.Background(), ptpConfig, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
			Expect(err).Should(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("storageType is set to StorageClass " + invalidStorageClass + " which does not exist")))
		})

		It("should reject event config", func() {
			By("If TransportHost is http and storageType are not set")

			// wait for k8s to refresh
			time.Sleep(2 * time.Second)

			ptpConfig, err := testclient.Client.PtpV1Interface.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Get(context.Background(), testutils.PtpConfigOperatorName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			ptpConfig.Spec.EventConfig = &ptpv1.PtpEventConfig{
				EnableEventPublisher: true,
				TransportHost:        "http://mock",
			}
			_, err = testclient.Client.PtpOperatorConfigs(testutils.PtpLinuxDaemonNamespace).Update(context.Background(), ptpConfig, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
			Expect(err).Should(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("storageType must be set")))
		})
	})
})
