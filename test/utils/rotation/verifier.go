// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	. "github.com/onsi/gomega"
)

// Verifier does some assertions in different phases of the credentials rotation test.
type Verifier interface {
	// Before is called before the rotation is started.
	Before()
	// ExpectPreparingStatus is called while waiting for the Preparing status.
	ExpectPreparingStatus(g Gomega)
	// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
	ExpectPreparingWithoutWorkersRolloutStatus(g Gomega)
	// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
	ExpectWaitingForWorkersRolloutStatus(g Gomega)
	// AfterPrepared is called when the Shoot is in Prepared status.
	AfterPrepared()
	// ExpectCompletingStatus is called while waiting for the Completing status.
	ExpectCompletingStatus(g Gomega)
	// AfterCompleted is called when the Shoot is in Completed status.
	AfterCompleted()
}

// Verifiers combines multiple Verifier instances and calls them sequentially
type Verifiers []Verifier

var _ Verifier = Verifiers{}
var _ CleanupVerifier = Verifiers{}

// Before is called before the rotation is started.
func (v Verifiers) Before() {
	for _, vv := range v {
		vv.Before()
	}
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v Verifiers) ExpectPreparingStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectPreparingStatus(g)
	}
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v Verifiers) ExpectPreparingWithoutWorkersRolloutStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectPreparingWithoutWorkersRolloutStatus(g)
	}
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v Verifiers) ExpectWaitingForWorkersRolloutStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectWaitingForWorkersRolloutStatus(g)
	}
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v Verifiers) AfterPrepared() {
	for _, vv := range v {
		vv.AfterPrepared()
	}
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v Verifiers) ExpectCompletingStatus(g Gomega) {
	for _, vv := range v {
		vv.ExpectCompletingStatus(g)
	}
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v Verifiers) AfterCompleted() {
	for _, vv := range v {
		vv.AfterCompleted()
	}
}

// CleanupVerifier can be implemented optionally to run cleanup code.
type CleanupVerifier interface {
	// Cleanup is passed to ginkgo.DeferCleanup.
	Cleanup()
}

// Cleanup is passed to ginkgo.DeferCleanup.
func (v Verifiers) Cleanup() {
	for _, vv := range v {
		if cleanup, ok := vv.(CleanupVerifier); ok {
			cleanup.Cleanup()
		}
	}
}
