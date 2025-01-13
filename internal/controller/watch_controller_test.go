package controller

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	auditv1alpha1 "github.com/vandathron/watchman/api/v1alpha1"
	"github.com/vandathron/watchman/internal/audit"
	"github.com/vandathron/watchman/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"slices"
	"time"
)

var _ = Describe("Watch Controller", func() {

	const (
		resourceName = "test-resource"
		timeout      = time.Second * 10
		duration     = time.Second * 10
		interval     = time.Millisecond * 250
	)
	ctx := context.Background()
	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: "default",
	}

	Describe("Reconciling watch resource", func() {
		Context("A new watch resource is created", func() {
			watch := &auditv1alpha1.Watch{}

			BeforeEach(func() {
				By("Creating the custom resource for the Kinds Watch")
				err := k8sClient.Get(ctx, typeNamespacedName, watch)
				if err != nil && errors.IsNotFound(err) {
					resource := &auditv1alpha1.Watch{
						ObjectMeta: metav1.ObjectMeta{
							Name:      typeNamespacedName.Name,
							Namespace: typeNamespacedName.Namespace,
						},
						Spec: auditv1alpha1.WatchSpec{
							Selectors: []auditv1alpha1.WatchSelector{
								{Namespace: "default", Kinds: []string{"Deployment", "Service"}},
							},
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
					Eventually(ctx, func(g Gomega) {
						Expect(k8sClient.Get(ctx, typeNamespacedName, watch)).To(Succeed())
					}, timeout, interval).Should(Succeed())
				}
			})

			AfterEach(func() {
				By("Deleting existing custom resource for the Kinds Watch")
				Expect(k8sClient.Delete(ctx, watch)).To(Succeed())
			})

			It("should create a config map for the resource", func() {
				reconciler := &WatchReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					Audit:  &audit.Console{},
				}
				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func(g Gomega) {
					cm := &v1.ConfigMap{}
					err := k8sClient.Get(ctx, typeNamespacedName, cm)
					g.Expect(err).NotTo(HaveOccurred())
					selectorMap := utils.ExtractWatchedKindsFromCM(cm.Data)
					kinds, ok := selectorMap[watch.Namespace]
					g.Expect(ok).To(BeTrue())
					g.Expect(len(kinds)).To(Equal(len(watch.Spec.Selectors[0].Kinds)))
					for _, kind := range watch.Spec.Selectors[0].Kinds {
						g.Expect(slices.Index(kinds, kind)).ShouldNot(BeEquivalentTo(-1))
					}
				}, timeout, interval).Should(Succeed())
			})

		})

	})
})

func makeDeploymentSpec(name, ns string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app.kubernetes.io/name": name},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/name": name},
				},

				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image:   "busybox",
						Name:    "busybox",
						Command: []string{"sh", "-c", "sleep 1000"},
					}},
				},
			},
		},
	}
}

func makeSvcSpec(name, ns string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports: []v1.ServicePort{
				{Port: 9090},
			},
		},
	}
}

func makeNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
