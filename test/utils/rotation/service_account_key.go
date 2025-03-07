// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ServiceAccountKeyVerifier verifies the service account key rotation.
type ServiceAccountKeyVerifier struct {
	SecretsManagerLabelSelector         client.MatchingLabels
	GetServiceAccountKeyRotation        func() *gardencorev1beta1.ServiceAccountKeyRotation
	ListServiceAccountKeySecretsFunc    func(ctx context.Context, namespace client.InNamespace, matchLabels client.MatchingLabels) (*corev1.SecretList, error)
	GetServiceAccountKeySecretNamespace func() string

	secretsBefore   SecretConfigNamesToSecrets
	secretsPrepared SecretConfigNamesToSecrets
}

const (
	serviceAccountKey       = "service-account-key"
	serviceAccountKeyBundle = "service-account-key-bundle"
)

// Before is called before the rotation is started.
func (v *ServiceAccountKeyVerifier) Before() {
	It("Verify old service account key secret", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			secretList, err := v.ListServiceAccountKeySecretsFunc(ctx, client.InNamespace(v.GetServiceAccountKeySecretNamespace()), v.SecretsManagerLabelSelector)
			g.Expect(err).NotTo(HaveOccurred())

			grouped := GroupByName(secretList.Items)
			g.Expect(grouped[serviceAccountKey]).To(HaveLen(1), "service account key secret should get created, but not rotated yet")
			g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1), "service account key bundle secret should get created, but not rotated yet")
			v.secretsBefore = grouped
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ServiceAccountKeyVerifier) ExpectPreparingStatus() {
	It("expect preparing status", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
			g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
			g.Expect(time.Now().UTC().Sub(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
			g.Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
			g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ServiceAccountKeyVerifier) ExpectPreparingWithoutWorkersRolloutStatus() {
	It("expect preparing without workers rollout", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
			g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPreparingWithoutWorkersRollout))
			g.Expect(time.Now().UTC().Sub(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
			g.Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
			g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ServiceAccountKeyVerifier) ExpectWaitingForWorkersRolloutStatus() {
	It("expect waiting for workers rollout", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
			g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			g.Expect(serviceAccountKeyRotation.LastInitiationTime).NotTo(BeNil())
			g.Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
			g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ServiceAccountKeyVerifier) AfterPrepared() {
	It("rotation phase should be 'Prepared'", func() {
		serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
		Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
		Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).NotTo(BeNil())
		Expect(serviceAccountKeyRotation.LastInitiationFinishedTime.After(serviceAccountKeyRotation.LastInitiationTime.Time)).To(BeTrue())
	})

	It("Verify service account key bundle secret", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			secretList, err := v.ListServiceAccountKeySecretsFunc(ctx, client.InNamespace(v.GetServiceAccountKeySecretNamespace()), v.SecretsManagerLabelSelector)
			g.Expect(err).NotTo(HaveOccurred())

			grouped := GroupByName(secretList.Items)
			g.Expect(grouped[serviceAccountKey]).To(HaveLen(2), "service account key secret should get rotated, but old service account key is kept")
			g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1))

			g.Expect(grouped[serviceAccountKey]).To(ContainElement(v.secretsBefore[serviceAccountKey][0]), "old service account key secret should be kept")
			g.Expect(grouped[serviceAccountKeyBundle]).To(Not(ContainElement(v.secretsBefore[serviceAccountKeyBundle][0])), "service account key bundle should have changed")
			v.secretsPrepared = grouped
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ServiceAccountKeyVerifier) ExpectCompletingStatus() {
	It("expect completing status", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
			g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationCompleting))
			g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).NotTo(BeNil())
			g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime.Time.Equal(serviceAccountKeyRotation.LastInitiationFinishedTime.Time) ||
				serviceAccountKeyRotation.LastCompletionTriggeredTime.After(serviceAccountKeyRotation.LastInitiationFinishedTime.Time)).To(BeTrue())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ServiceAccountKeyVerifier) AfterCompleted() {
	It("rotation phase should be 'Completed'", func() {
		serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
		Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationCompleted))
		Expect(serviceAccountKeyRotation.LastCompletionTime.Time.UTC().After(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeTrue())
		Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
		Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
	})

	It("Verify new service account key secret", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			secretList, err := v.ListServiceAccountKeySecretsFunc(ctx, client.InNamespace(v.GetServiceAccountKeySecretNamespace()), v.SecretsManagerLabelSelector)
			g.Expect(err).NotTo(HaveOccurred())

			grouped := GroupByName(secretList.Items)
			g.Expect(grouped[serviceAccountKey]).To(HaveLen(1), "old service account key secret should get cleaned up")
			g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1))

			g.Expect(grouped[serviceAccountKey]).To(ContainElement(v.secretsPrepared[serviceAccountKey][1]), "new service account key secret should be kept")
			g.Expect(grouped[serviceAccountKeyBundle]).To(Not(ContainElement(v.secretsPrepared[serviceAccountKeyBundle][0])), "service account key bundle should have changed")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}
