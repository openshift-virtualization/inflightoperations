//go:build e2e
// +build e2e

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ifo-operator/inflightoperations/test/utils"
)

// namespace where the project is deployed in
const namespace = "inflightoperations-system"

// serviceAccountName created for the project
const serviceAccountName = "inflightoperations-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "inflightoperations-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "inflightoperations-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", managerImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("removing the metrics ClusterRoleBinding")
		cmd = exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=inflightoperations-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", controllerPodName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted, 3*time.Minute, time.Second).Should(Succeed())

			// +kubebuilder:scaffold:e2e-metrics-webhooks-readiness

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable, 2*time.Minute).Should(Succeed())
		})

	})

	Context("OperationRuleSet reconciliation", Ordered, func() {
		const testNS = "e2e-test"

		BeforeAll(func() {
			By("creating test namespace")
			cmd := exec.Command("kubectl", "create", "ns", testNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			By("cleaning up test resources")
			cmd := exec.Command("kubectl", "delete", "ors", "--all", "--ignore-not-found")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "ifo", "--all", "--ignore-not-found")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "ns", testNS, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})

		It("should reconcile an OperationRuleSet to Ready", func() {
			By("applying the deployment OperationRuleSet")
			kubectlApply("rules/deployment_operationrule.yaml")

			By("waiting for the OperationRuleSet to become Ready")
			verifyReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "deployment-rules",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyReady).Should(Succeed())

			By("verifying the watch is active")
			cmd := exec.Command("kubectl", "get", "ors", "deployment-rules",
				"-o", "jsonpath={.status.watchActive}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("true"))

			By("verifying the finalizer is present")
			cmd = exec.Command("kubectl", "get", "ors", "deployment-rules",
				"-o", "jsonpath={.metadata.finalizers[0]}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("ifo.kubevirt.io/finalizer"))
		})

		It("should reject an OperationRuleSet with an invalid CEL expression", func() {
			By("applying an OperationRuleSet with invalid CEL")
			kubectlApply(testdataPath("ors-invalid.yaml"))

			By("waiting for the InvalidRule condition")
			verifyInvalid := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "invalid-rules",
					"-o", "jsonpath={.status.conditions[?(@.type=='InvalidRule')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyInvalid).Should(Succeed())

			By("verifying Ready is not True")
			cmd := exec.Command("kubectl", "get", "ors", "invalid-rules",
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(Equal("True"))

			By("cleaning up invalid ruleset")
			kubectlDelete(testdataPath("ors-invalid.yaml"))
		})

		It("should create an InFlightOperation when a Deployment rolls out", func() {
			By("creating a Deployment that will trigger a Rollout operation")
			kubectlApply(testdataPath("deploy-rollout.yaml"))

			By("waiting for an InFlightOperation to be created for the Deployment")
			verifyIFOCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-rollout,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items[0].spec.operation}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Rollout"))
			}
			Eventually(verifyIFOCreated).Should(Succeed())

			By("verifying IFO labels")
			cmd := exec.Command("kubectl", "get", "ifo",
				"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-rollout,ifo.kubevirt.io/subject-namespace=%s", testNS),
				"-o", "jsonpath={.items[0].metadata.labels.ifo\\.kubevirt\\.io/operation}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("Rollout"))

			By("verifying IFO subject fields")
			cmd = exec.Command("kubectl", "get", "ifo",
				"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-rollout,ifo.kubevirt.io/subject-namespace=%s", testNS),
				"-o", "jsonpath={.items[0].spec.subject.kind}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("Deployment"))
		})

		It("should complete or clean up the IFO when the rollout finishes", func() {
			By("waiting for the Deployment to be fully available")
			verifyAvailable := func(g Gomega) {
				cmd := exec.Command("kubectl", "rollout", "status",
					"deployment/e2e-rollout", "-n", testNS, "--timeout=10s")
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyAvailable, 3*time.Minute, 5*time.Second).Should(Succeed())

			By("waiting for the IFO to be completed or deleted")
			verifyCompleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-rollout,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				// With RetainCompletedIFOs=false (default), the IFO should be deleted.
				// If retained, it should show Completed phase.
				if output == "[]" || output == "" {
					return // deleted — success
				}
				// If still present, it should be Completed
				cmd = exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-rollout,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items[0].status.phase}")
				phase, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(phase).To(Equal("Completed"))
			}
			Eventually(verifyCompleted, 3*time.Minute, 5*time.Second).Should(Succeed())

			By("cleaning up the test Deployment")
			kubectlDelete(testdataPath("deploy-rollout.yaml"))
		})

		It("should apply static labels from the OperationRuleSet to IFOs", func() {
			By("applying an OperationRuleSet with static labels")
			kubectlApply(testdataPath("ors-labeled.yaml"))

			By("waiting for the ruleset to be Ready")
			verifyReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "labeled-rules",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyReady).Should(Succeed())

			By("creating a Deployment to trigger an IFO")
			kubectlApply(testdataPath("deploy-labels.yaml"))

			By("verifying the IFO has the static label")
			verifyLabel := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-labels,ifo.kubevirt.io/ruleset=labeled-rules,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items[0].metadata.labels.e2e-test-label}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("e2e-test-value"))
			}
			Eventually(verifyLabel).Should(Succeed())

			By("cleaning up")
			kubectlDelete(testdataPath("deploy-labels.yaml"))
			kubectlDelete(testdataPath("ors-labeled.yaml"))
		})

		It("should apply dynamic labels from labelExpressions to IFOs", func() {
			By("applying an OperationRuleSet with label expressions")
			kubectlApply(testdataPath("ors-dynamic-labels.yaml"))

			By("waiting for the ruleset to be Ready")
			verifyReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "dynamic-label-rules",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyReady).Should(Succeed())

			By("creating a Deployment to trigger an IFO")
			kubectlApply(testdataPath("deploy-dynlabel.yaml"))

			By("verifying the IFO has the dynamic label")
			verifyLabel := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-dynlabel,ifo.kubevirt.io/ruleset=dynamic-label-rules,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items[0].metadata.labels.e2e-dynamic-ns}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(testNS))
			}
			Eventually(verifyLabel).Should(Succeed())

			By("cleaning up")
			kubectlDelete(testdataPath("deploy-dynlabel.yaml"))
			kubectlDelete(testdataPath("ors-dynamic-labels.yaml"))
		})

		It("should only create IFOs for namespaces specified in the ruleset", func() {
			const includedNS = "e2e-ns-included"
			const excludedNS = "e2e-ns-excluded"

			By("creating test namespaces")
			for _, ns := range []string{includedNS, excludedNS} {
				cmd := exec.Command("kubectl", "create", "ns", ns)
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
			}

			By("applying an OperationRuleSet that only targets the included namespace")
			kubectlApply(testdataPath("ors-ns-filter.yaml"))

			By("waiting for the ruleset to be Ready")
			verifyReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "ns-filter-rules",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyReady).Should(Succeed())

			By("creating Deployments in both namespaces")
			kubectlApply(testdataPath("deploy-nsfilter-included.yaml"))
			kubectlApply(testdataPath("deploy-nsfilter-excluded.yaml"))

			By("verifying an IFO is created for the included namespace")
			verifyIncluded := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-nsfilter,ifo.kubevirt.io/subject-namespace=%s,ifo.kubevirt.io/ruleset=ns-filter-rules", includedNS),
					"-o", "jsonpath={.items[0].spec.operation}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Rollout"))
			}
			Eventually(verifyIncluded).Should(Succeed())

			By("verifying no IFO is created for the excluded namespace")
			Consistently(func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-nsfilter,ifo.kubevirt.io/subject-namespace=%s,ifo.kubevirt.io/ruleset=ns-filter-rules", excludedNS),
					"-o", "jsonpath={.items}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(SatisfyAny(Equal("[]"), BeEmpty()))
			}, 15*time.Second, 3*time.Second).Should(Succeed())

			By("cleaning up")
			kubectlDelete(testdataPath("ors-ns-filter.yaml"))
			kubectlDelete(testdataPath("deploy-nsfilter-included.yaml"))
			kubectlDelete(testdataPath("deploy-nsfilter-excluded.yaml"))
			for _, ns := range []string{includedNS, excludedNS} {
				cmd := exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found")
				_, _ = utils.Run(cmd)
			}
		})

		It("should clean up when an OperationRuleSet is deleted", func() {
			By("applying a temporary OperationRuleSet")
			kubectlApply(testdataPath("ors-temp.yaml"))

			By("waiting for it to be Ready")
			verifyReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "temp-rules",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"))
			}
			Eventually(verifyReady).Should(Succeed())

			By("deleting the OperationRuleSet")
			cmd := exec.Command("kubectl", "delete", "ors", "temp-rules")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying it is fully deleted (finalizer ran)")
			verifyDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ors", "temp-rules",
					"--ignore-not-found", "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(BeEmpty())
			}
			Eventually(verifyDeleted, 30*time.Second, 2*time.Second).Should(Succeed())
		})

		It("should clean up IFOs when the subject is deleted", func() {
			By("creating a Deployment that will trigger an IFO")
			kubectlApply(testdataPath("deploy-delete.yaml"))

			By("waiting for an IFO to be created")
			verifyIFO := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-delete,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items[0].spec.operation}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Rollout"))
			}
			Eventually(verifyIFO).Should(Succeed())

			By("deleting the Deployment")
			cmd := exec.Command("kubectl", "delete", "deployment", "e2e-delete", "-n", testNS)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying all IFOs for this subject are deleted")
			verifyNoIFOs := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "ifo",
					"-l", fmt.Sprintf("ifo.kubevirt.io/subject-name=e2e-delete,ifo.kubevirt.io/subject-namespace=%s", testNS),
					"-o", "jsonpath={.items}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(SatisfyAny(Equal("[]"), BeEmpty()))
			}
			Eventually(verifyNoIFOs).Should(Succeed())
		})
	})
})

// testdataPath returns the path to a fixture file in the testdata directory,
// relative to the project root (since utils.Run sets cwd to the project root).
func testdataPath(name string) string {
	return filepath.Join("test", "e2e", "testdata", name)
}

// kubectlApply applies a YAML file. The path is relative to the project root.
func kubectlApply(path string) {
	cmd := exec.Command("kubectl", "apply", "-f", path)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to apply %s", path)
}

// kubectlDelete deletes resources defined in a YAML file, ignoring not-found errors.
func kubectlDelete(path string) {
	cmd := exec.Command("kubectl", "delete", "-f", path, "--ignore-not-found")
	_, _ = utils.Run(cmd)
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
