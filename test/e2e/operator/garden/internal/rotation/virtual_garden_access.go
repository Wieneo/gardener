// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/access"
)

type clients struct {
	accessSecret, clientCert, serviceAccountDynamic kubernetes.Interface
}

// VirtualGardenAccessVerifier uses the various access methods to access the virtual garden.
type VirtualGardenAccessVerifier struct {
	*GardenContext
	Namespace string

	clientsBefore, clientsPrepared, clientsAfter clients
}

// Before is called before the rotation is started.
func (v *VirtualGardenAccessVerifier) Before() {
	It("Request new client certificate and using it to access virtual garden", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			var err error
			v.clientsBefore.accessSecret, err = kubernetes.NewClientFromSecret(ctx, v.GardenClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
			g.Expect(err).NotTo(HaveOccurred())

			virtualGardenClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsBefore.accessSecret, "e2e-rotate-csr-before")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsBefore.clientCert = virtualGardenClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new dynamic token for a ServiceAccount and using it to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			virtualGardenClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsBefore.accessSecret, "e2e-rotate-sa-dynamic-before")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsBefore.serviceAccountDynamic = virtualGardenClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *VirtualGardenAccessVerifier) ExpectPreparingStatus() {}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *VirtualGardenAccessVerifier) ExpectPreparingWithoutWorkersRolloutStatus() {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *VirtualGardenAccessVerifier) ExpectWaitingForWorkersRolloutStatus() {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *VirtualGardenAccessVerifier) AfterPrepared() {
	It("Use client certificate from before rotation to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use dynamic ServiceAccount token from before rotation to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new client certificate and using it to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			var err error
			v.clientsPrepared.accessSecret, err = kubernetes.NewClientFromSecret(ctx, v.GardenClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
			g.Expect(err).NotTo(HaveOccurred())

			virtualGardenClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsPrepared.accessSecret, "e2e-rotate-csr-prepared")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsPrepared.clientCert = virtualGardenClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new dynamic token for a ServiceAccount and using it to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			virtualGardenClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsPrepared.accessSecret, "e2e-rotate-sa-dynamic-prepared")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsPrepared.serviceAccountDynamic = virtualGardenClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *VirtualGardenAccessVerifier) ExpectCompletingStatus() {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *VirtualGardenAccessVerifier) AfterCompleted() {
	It("Use client certificate from before rotation to access target cluster", func(ctx SpecContext) {
		Consistently(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.clientCert.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use dynamic ServiceAccount token from before rotation to access target cluster", func(ctx SpecContext) {
		Consistently(ctx, func(g Gomega) {
			g.Expect(v.clientsBefore.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).NotTo(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use client certificate from after preparation to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsPrepared.clientCert.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Use dynamic ServiceAccount token from after preparation to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.clientsPrepared.serviceAccountDynamic.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new client certificate and using it to access target cluster", func(ctx SpecContext) {
		Eventually(func(g Gomega) {
			var err error
			v.clientsAfter.accessSecret, err = kubernetes.NewClientFromSecret(ctx, v.GardenClient, v.Namespace, "gardener", kubernetes.WithDisabledCachedClient())
			Expect(err).NotTo(HaveOccurred())

			virtualGardenClient, err := access.CreateTargetClientFromCSR(ctx, v.clientsAfter.accessSecret, "e2e-rotate-csr-after")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsAfter.clientCert = virtualGardenClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Request new dynamic token for a ServiceAccount and using it to access target cluster", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			virtualGardenClient, err := access.CreateTargetClientFromDynamicServiceAccountToken(ctx, v.clientsAfter.accessSecret, "e2e-rotate-sa-dynamic-after")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(virtualGardenClient.Client().List(ctx, &corev1.NamespaceList{})).To(Succeed())

			v.clientsAfter.serviceAccountDynamic = virtualGardenClient
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v *VirtualGardenAccessVerifier) Cleanup() {
	It("Clean up objects in virtual garden from client certificate access", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(access.CleanupObjectsFromCSRAccess(ctx, v.VirtualClusterClientSet)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Clean up objects in virtual garden from dynamic ServiceAccount token access", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(access.CleanupObjectsFromDynamicServiceAccountTokenAccess(ctx, v.VirtualClusterClientSet)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
