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

			It("should annotate configured watch kinds in their respective namespaces", func() {
				reconciler := &WatchReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					Audit:  &audit.Console{},
				}
				ns := "dont-watch"
				Expect(k8sClient.Create(ctx, makeNamespace(ns))).To(Succeed())
				// create deployments, svc in 2 namespaces
				deploy1 := makeDeploymentSpec("deploy-1", ns)
				svc1 := makeSvcSpec("svc-1", ns)
				Expect(k8sClient.Create(ctx, deploy1)).To(Succeed())
				Expect(k8sClient.Create(ctx, svc1)).To(Succeed())

				// ns to watch
				deploy2 := makeDeploymentSpec("deploy-2", typeNamespacedName.Namespace)
				svc2 := makeSvcSpec("svc-2", typeNamespacedName.Namespace)
				Expect(k8sClient.Create(ctx, deploy2)).To(Succeed())
				Expect(k8sClient.Create(ctx, svc2)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns}, &v1.Namespace{})).To(Succeed())
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: deploy1.Name, Namespace: deploy1.Namespace}, &appsv1.Deployment{})).To(Succeed())
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: svc1.Name, Namespace: svc1.Namespace}, &v1.Service{})).To(Succeed())
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: deploy2.Name, Namespace: deploy2.Namespace}, &appsv1.Deployment{})).To(Succeed())
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: svc2.Name, Namespace: svc2.Namespace}, &v1.Service{})).To(Succeed())
				}, timeout, interval).Should(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func(g Gomega) {
					// Should not contain annotation
					dep := &appsv1.Deployment{}
					svc := &v1.Service{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: deploy1.Name, Namespace: deploy1.Namespace}, dep)).To(Succeed())
					if dep.Annotations != nil {
						g.Expect(utils.HasWatchManAnnotation(dep.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeFalse())
					}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: svc1.Name, Namespace: svc1.Namespace}, svc)).To(Succeed())
					if svc.Annotations != nil {
						g.Expect(utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeFalse())
					}

					deployments := &appsv1.DeploymentList{}
					if err := reconciler.List(ctx, deployments, client.InNamespace("default")); err != nil {
						fmt.Println("err")
					}

					// Should contain annotation
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: svc2.Name, Namespace: svc2.Namespace}, svc)).To(Succeed())
					g.Expect(svc.Annotations).NotTo(BeNil())
					g.Expect(utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())

					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: deploy2.Name, Namespace: deploy2.Namespace}, dep)).To(Succeed())
					g.Expect(dep.Annotations).NotTo(BeNil())
					g.Expect(utils.HasWatchManAnnotation(dep.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())

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
