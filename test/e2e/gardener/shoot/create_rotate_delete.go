// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/rotation"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

func testCredentialRotation(s *ShootContext, v rotationutils.Verifiers, startRotationAnnotation, completeRotationAnnotation string) {
	v.Before()

	if startRotationAnnotation != "" {
		ItShouldAnnotateShoot(s, map[string]string{
			v1beta1constants.GardenerOperation: startRotationAnnotation,
		})

		itShouldEventuallyNotHaveOperationAnnotation(s)
		v.ExpectPreparingStatus()
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		v.AfterPrepared()
	}

	testCredentialRotationComplete(s, v, completeRotationAnnotation)
}

func testCredentialRotationComplete(s *ShootContext, v rotationutils.Verifiers, completeRotationAnnotation string) {
	if completeRotationAnnotation != "" {
		ItShouldAnnotateShoot(s, map[string]string{
			v1beta1constants.GardenerOperation: completeRotationAnnotation,
		})

		itShouldEventuallyNotHaveOperationAnnotation(s)
		v.ExpectCompletingStatus()
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		// renew shoot clients after rotation
		ItShouldInitializeShootClient(s)
		v.AfterCompleted()
	}

	v.Cleanup()
}

func testCredentialRotationWithoutWorkersRollout(s *ShootContext, v rotationutils.Verifiers) {
	v.Before()

	beforeStartMachinePodNames := sets.New[string]()

	It("Find all machine pods to ensure later that they weren't rolled out", func(ctx SpecContext) {
		beforeStartMachinePodList := &corev1.PodList{}
		Eventually(ctx, s.SeedKomega.List(beforeStartMachinePodList, client.InNamespace(s.Shoot.Status.TechnicalID), client.MatchingLabels{
			"app":              "machine",
			"machine-provider": "local",
		})).Should(Succeed())

		for _, item := range beforeStartMachinePodList.Items {
			beforeStartMachinePodNames.Insert(item.Name)
		}
	}, SpecTimeout(time.Minute))

	ItShouldAnnotateShoot(s, map[string]string{
		v1beta1constants.GardenerOperation: v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
	})

	itShouldEventuallyNotHaveOperationAnnotation(s)
	v.ExpectPreparingWithoutWorkersRolloutStatus()
	ItShouldWaitForShootToBeReconciledAndHealthy(s)
	v.ExpectWaitingForWorkersRolloutStatus()

	It("Compare machine pod names", func(ctx SpecContext) {
		afterStartMachinePodList := &corev1.PodList{}
		Eventually(ctx, s.SeedKomega.List(afterStartMachinePodList, client.InNamespace(s.Shoot.Status.TechnicalID), client.MatchingLabels{
			"app":              "machine",
			"machine-provider": "local",
		})).Should(Succeed())

		afterStartMachinePodNames := sets.New[string]()
		for _, item := range afterStartMachinePodList.Items {
			afterStartMachinePodNames.Insert(item.Name)
		}

		Expect(beforeStartMachinePodNames.Equal(afterStartMachinePodNames)).To(BeTrue())
	}, SpecTimeout(time.Minute))

	It("Ensure all worker pools are marked as 'pending for roll out'", func() {
		for _, worker := range s.Shoot.Spec.Provider.Workers {
			Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
				return rollout.Name == worker.Name
			})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in CA rotation status")

			Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
				return rollout.Name == worker.Name
			})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in service account key rotation status")
		}
	})

	var lastWorkerPoolName string
	It("Remove last worker pool from spec", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			lastWorkerPoolName = s.Shoot.Spec.Provider.Workers[len(s.Shoot.Spec.Provider.Workers)-1].Name
			s.Shoot.Spec.Provider.Workers = slices.DeleteFunc(s.Shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
				return worker.Name == lastWorkerPoolName
			})
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(s)

	It("Last worker pool no longer pending rollout", func() {
		Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == lastWorkerPoolName
		})).To(BeFalse())
		Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == lastWorkerPoolName
		})).To(BeFalse())
	})

	It("Trigger rollout of pending worker pools", func(ctx SpecContext) {
		workerNames := sets.New[string]()
		for _, rollout := range s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts {
			workerNames.Insert(rollout.Name)
		}
		for _, rollout := range s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts {
			workerNames.Insert(rollout.Name)
		}

		// as this annotation is computed dynamically, we can't use the "ItShouldAnnotateShoot" function
		// this is because the ginkgo tree construction would just pass the empty output string to the annotate function
		rolloutWorkersAnnotation := v1beta1constants.OperationRotateRolloutWorkers + "=" + strings.Join(workerNames.UnsortedList(), ",")
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			metav1.SetMetaDataAnnotation(&s.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, rolloutWorkersAnnotation)
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(s)

	It("Credential rotation in status prepared", func() {
		Expect(s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
		Expect(s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
	})

	v.AfterPrepared()

	testCredentialRotationComplete(s, v, v1beta1constants.OperationRotateCredentialsComplete)
}

func itShouldEventuallyNotHaveOperationAnnotation(s *ShootContext) {
	It("Should not have operation annotation", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Object(s.Shoot)).WithPolling(2 * time.Second).Should(
			HaveField("ObjectMeta.Annotations", Not(HaveKey(v1beta1constants.GardenerOperation))))
	}, SpecTimeout(time.Minute))
}

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create Shoot, Rotate Credentials and Delete Shoot", Label("credentials-rotation"), func() {
		test := func(s *ShootContext, withoutWorkersRollout bool) {
			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			// isolated test for ssh key rotation (does not trigger node rolling update)
			if !v1beta1helper.IsWorkerless(s.Shoot) && !withoutWorkersRollout {
				testCredentialRotation(s, rotationutils.Verifiers{&rotation.SSHKeypairVerifier{ShootContext: s}}, v1beta1constants.ShootOperationRotateSSHKeypair, "")
			}

			v := rotationutils.Verifiers{
				// basic verifiers checking secrets
				&rotation.CAVerifier{ShootContext: s},
				&rotation.ShootAccessVerifier{ShootContext: s},
				&rotationutils.ObservabilityVerifier{
					GetObservabilitySecretFunc: func(ctx context.Context) (*corev1.Secret, error) {
						secret := &corev1.Secret{}
						return secret, s.GardenClient.Get(ctx, client.ObjectKey{Namespace: s.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(s.Shoot.Name, "monitoring")}, secret)
					},
					GetObservabilityEndpoint: func(secret *corev1.Secret) string {
						return secret.Annotations["plutono-url"]
					},
					GetObservabilityRotation: func() *gardencorev1beta1.ObservabilityRotation {
						return s.Shoot.Status.Credentials.Rotation.Observability
					},
				},
				&rotationutils.ETCDEncryptionKeyVerifier{
					GetETCDSecretNamespace: func() string {
						return s.Shoot.Status.TechnicalID
					},
					ListETCDEncryptionSecretsFunc: func(ctx context.Context, namespace client.InNamespace, matchLabels client.MatchingLabels) (*corev1.SecretList, error) {
						secretList := &corev1.SecretList{}
						return secretList, s.SeedClient.List(ctx, secretList, namespace, matchLabels)
					},
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
						return s.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey
					},
					EncryptionKey:  v1beta1constants.SecretNameETCDEncryptionKey,
					RoleLabelValue: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration,
				},
				&rotationutils.ServiceAccountKeyVerifier{
					GetServiceAccountKeySecretNamespace: func() string {
						return s.Shoot.Status.TechnicalID
					},
					ListServiceAccountKeySecretsFunc: func(ctx context.Context, namespace client.InNamespace, matchLabels client.MatchingLabels) (*corev1.SecretList, error) {
						secretList := &corev1.SecretList{}
						return secretList, s.SeedClient.List(ctx, secretList, namespace, matchLabels)
					},
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetServiceAccountKeyRotation: func() *gardencorev1beta1.ServiceAccountKeyRotation {
						return s.Shoot.Status.Credentials.Rotation.ServiceAccountKey
					},
				},
				// advanced verifiers testing things from the user's perspective
				&rotationutils.EncryptedDataVerifier{
					TargetClientFunc: func() client.Client {
						return s.ShootClient
					},
					Resources: []rotationutils.EncryptedResource{
						{
							NewObject: func() client.Object {
								return &corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
									StringData: map[string]string{"content": "foo"},
								}
							},
							NewEmptyList: func() client.ObjectList { return &corev1.SecretList{} },
						},
					},
				},
			}

			if !v1beta1helper.IsWorkerless(s.Shoot) && !withoutWorkersRollout {
				v = append(v, &rotation.SSHKeypairVerifier{ShootContext: s})
			}

			if !withoutWorkersRollout {
				// test rotation for every rotation type
				testCredentialRotation(s, v, v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsComplete)
			} else {
				testCredentialRotationWithoutWorkersRollout(s, v)
			}

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		}

		Context("Shoot with workers", Label("basic"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-rotate")), false)

			Context("without workers rollout", Label("without-workers-rollout"), Ordered, func() {
				var s *ShootContext
				BeforeTestSetup(func() {
					shoot := DefaultShoot("e2e-rotate")
					shoot.Name = "e2e-rot-noroll"
					// Add a second worker pool when worker rollout should not be performed such that we can make proper
					// assertions of the shoot status
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers[0])
					shoot.Spec.Provider.Workers[len(shoot.Spec.Provider.Workers)-1].Name += "2"

					s = NewTestContext().ForShoot(shoot)
				})

				test(s, true)
			})

		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-rotate")), false)
		})
	})
})
