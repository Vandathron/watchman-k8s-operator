package v1alpha1

import (
	"errors"
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vandathron/watchman/internal/utils"
	"k8s.io/utils/strings/slices"

	auditv1alpha1 "github.com/vandathron/watchman/api/v1alpha1"
)

var _ = Describe("Watch Webhook", func() {
	var (
		watch     *auditv1alpha1.Watch
		oldWatch  *auditv1alpha1.Watch
		validator WatchCustomValidator
		defaulter WatchCustomDefaulter
	)

	BeforeEach(func() {
		watch = &auditv1alpha1.Watch{}
		oldWatch = &auditv1alpha1.Watch{}
		validator = WatchCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = WatchCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldWatch).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(watch).NotTo(BeNil(), "Expected obj to be initialized")
	})

	When("creating Watch under Defaulting Webhook", func() {
		It("Should apply defaults when kinds in a selector is empty", func() {
			By("simulating a scenario where defaults should be applied")
			watch.Spec.Selectors = append(watch.Spec.Selectors, auditv1alpha1.WatchSelector{Namespace: "ns-1"}, auditv1alpha1.WatchSelector{Namespace: "ns-2"})

			By("calling the Default method to apply defaults")
			Expect(defaulter.Default(ctx, watch)).To(Succeed())

			By("checking that the default values are set")
			Expect(len(watch.Spec.Selectors)).To(Equal(2))
			for _, selector := range watch.Spec.Selectors {
				Expect(len(selector.Kinds)).To(Equal(2)) // Supports just 2 kinds yet (Svc, Deploy)
				Expect(slices.Index(selector.Kinds, utils.SupportedKindDeployment)).NotTo(Equal(-1))
				Expect(slices.Index(selector.Kinds, utils.SupportedKindService)).NotTo(Equal(-1))
			}
		})
	})

	Describe("Validation webhook", func() {
		When("creating Watch resource by providing unsupported kinds", func() {
			It("Should fail validation", func() {
				By("Providing unsupported kind to watch")
				watch.Spec.Selectors = append(watch.Spec.Selectors, auditv1alpha1.WatchSelector{Namespace: "ns-1", Kinds: []string{"Deployment", "ConfigMap"}})

				By("calling ValidateCreate method to validate object")
				_, err := validator.ValidateCreate(ctx, watch)
				Expect(err).To(Equal(fmt.Errorf("unsupported kind(s) in namespace %v", "ns-1")))
			})
		})

		When("creating Watch resource with empty selector", func() {
			It("Should fail validation", func() {
				By("Setting empty selector")
				watch.Spec = auditv1alpha1.WatchSpec{Selectors: make([]auditv1alpha1.WatchSelector, 0)}

				By("calling ValidateCreate method to validate object")
				_, err := validator.ValidateCreate(ctx, watch)
				Expect(err).To(Equal(errors.New("selector can not be empty. Should contain at least a namespace")))
			})
		})

		When("updating Watch resource with empty selector", func() {
			It("Should fail validation", func() {
				By("Setting empty selector")
				watch.Spec = auditv1alpha1.WatchSpec{Selectors: make([]auditv1alpha1.WatchSelector, 0)}

				By("calling ValidateUpdate method to validate object")
				_, err := validator.ValidateUpdate(ctx, oldWatch, watch)
				Expect(err).To(Equal(errors.New("selector can not be empty. Should contain at least a namespace")))
			})
		})

		When("updating Watch resource by providing unsupported kinds", func() {
			It("Should fail validation", func() {
				By("Providing unsupported kind to watch")
				watch.Spec.Selectors = append(watch.Spec.Selectors, auditv1alpha1.WatchSelector{Namespace: "ns-1", Kinds: []string{"Deployment", "ConfigMap"}})

				By("calling ValidateUpdate method to validate object")
				_, err := validator.ValidateUpdate(ctx, oldWatch, watch)
				Expect(err).To(Equal(fmt.Errorf("unsupported kind(s) in namespace %v", "ns-1")))
			})
		})

		When("deleting watch resource", func() {
			It("Should do nothing", func() {
				By("calling ValidateDelete method to validate object")
				_, err := validator.ValidateDelete(ctx, watch)

				By("and doing nothing")
				Expect(err).To(Succeed())
			})
		})
	})
})
