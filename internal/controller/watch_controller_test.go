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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"slices"
	"sync"
	"time"
)

const (
	resourceName = "test-resource"
	timeout      = time.Second * 10
	duration     = time.Second * 10
	interval     = time.Millisecond * 250
)

var _ = Describe("Watch Controller", func() {
	ctx := context.Background()
	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: "default",
	}
	ns1 := "ns-1"
	ns2 := "ns-2"

	watch := &auditv1alpha1.Watch{}
	Describe("Reconciling watch resource", Ordered, func() {
		BeforeAll(func() {
			By("Creating namespaces")
			testCreateNamespaces(makeNamespace(ns1), makeNamespace(ns2))

			By("Creating watch resource")
			err := k8sClient.Get(ctx, typeNamespacedName, watch)
			Expect(errors.IsNotFound(err)).To(BeTrue())
			resource := &auditv1alpha1.Watch{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Spec: auditv1alpha1.WatchSpec{
					Selectors: []auditv1alpha1.WatchSelector{
						{Namespace: ns1, Kinds: []string{"Deployment", "Service"}},
					},
				},
			}

			By("Creating the watch resource")
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			Eventually(ctx, func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, watch)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		When("a new watch resource is created", func() {
			It("should handle watch resource spec config", func() {
				r := &WatchReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					Audit:  &audit.Console{},
				}

				By("Expecting watch resource config map not present")
				cm := &v1.ConfigMap{}
				Expect(errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, cm))).To(BeTrue())

				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				By("Creating watch resource config map")
				Eventually(func(g Gomega) {
					cm := &v1.ConfigMap{}
					err := k8sClient.Get(ctx, typeNamespacedName, cm)
					g.Expect(err).NotTo(HaveOccurred())
					selectorMap := utils.ExtractWatchedKindsFromCM(cm.Data)
					kinds, ok := selectorMap[watch.Spec.Selectors[0].Namespace]
					g.Expect(ok).To(BeTrue())
					g.Expect(len(kinds)).To(Equal(len(watch.Spec.Selectors[0].Kinds)))
					for _, kind := range watch.Spec.Selectors[0].Kinds {
						g.Expect(slices.Index(kinds, kind)).ShouldNot(BeEquivalentTo(-1))
					}
				}, timeout, interval).Should(Succeed())

				By("Creating deployments and services for test purposes")
				// create deployments, svc in 2 namespaces. Watching kinds in ns1 according to watch resource spec config
				testCreateDeployments(makeDeploymentSpec("deploy-1", ns1), makeDeploymentSpec("deploy-2", ns2))
				testCreateServices(makeSvcSpec("svc-1", ns1), makeSvcSpec("svc-2", ns2))

				By("Annotating all resources in watch resource selectors")
				_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func(g Gomega) {
					dep := &appsv1.Deployment{}
					svc := &v1.Service{}

					By("having ns2 resources not annotated")
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "deploy-2", Namespace: ns2}, dep)).To(Succeed())
					if dep.Annotations != nil {
						g.Expect(utils.HasWatchManAnnotation(dep.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeFalse())
					}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "svc-2", Namespace: ns2}, svc)).To(Succeed())
					if svc.Annotations != nil {
						g.Expect(utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeFalse())
					}

					By("having ns1 resources annotated with watchman annotation")
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "svc-1", Namespace: ns1}, svc)).To(Succeed())
					g.Expect(svc.Annotations).NotTo(BeNil())
					g.Expect(utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())

					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "deploy-1", Namespace: ns1}, dep)).To(Succeed())
					g.Expect(dep.Annotations).NotTo(BeNil())
					g.Expect(utils.HasWatchManAnnotation(dep.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())

				}, timeout, interval).Should(Succeed())

			})

		})

		When("watch resource spec selector is updated", func() {
			deployResourceCnt := 2
			svcResourceCnt := 2
			var once sync.Once

			BeforeEach(func() {
				By("Setting up deployments and services")
				once.Do(func() {
					testCreateDeployments(
						makeDeploymentSpec("dep-3", ns1),
						makeDeploymentSpec("dep-4", ns1),
						makeDeploymentSpec("dep-5", ns2),
						makeDeploymentSpec("dep-6", ns2))

					testCreateServices(
						makeSvcSpec("svc-3", ns1),
						makeSvcSpec("svc-4", ns1),
						makeSvcSpec("svc-5", ns2),
						makeSvcSpec("svc-6", ns2))

					watch.Spec.Selectors = []auditv1alpha1.WatchSelector{
						{Namespace: ns1, Kinds: []string{"Service"}},               // remove service
						{Namespace: ns2, Kinds: []string{"Service", "Deployment"}}, // add a new namespace to watch for kinds service and deployment
					}
					Expect(k8sClient.Update(ctx, watch)).To(Succeed())
					Eventually(ctx, func(g Gomega) {
						Expect(k8sClient.Get(ctx, typeNamespacedName, watch)).To(Succeed())
						Expect(len(watch.Spec.Selectors[0].Kinds)).To(Equal(1))
					}, timeout, interval).Should(Succeed())
				})
			})

			It("should update all necessary resources as per updated watch spec config", func() {

				r := &WatchReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
					Audit:  &audit.Console{},
				}
				cm := &v1.ConfigMap{}

				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName}) // reconcile to update config map
				Expect(err).To(BeNil())

				By("Updating watch resource config map")
				Eventually(ctx, func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, typeNamespacedName, cm)).To(Succeed())
					watchedKinds := utils.ExtractWatchedKindsFromCM(cm.Data)
					g.Expect(len(watchedKinds)).To(Equal(2)) // test that cm has both namespaces (ns1,ns2)

					// Test that cm contains ns1 and watches only a kind which is Service
					g.Expect(len(watchedKinds[ns1])).To(Equal(1))
					g.Expect(slices.Index(watchedKinds[ns1], "Service")).ShouldNot(BeEquivalentTo(-1))

					// Test that cm contains second namespace and watches two kinds which is Service and Deployment
					g.Expect(len(watchedKinds[ns2])).To(Equal(2))
					g.Expect(slices.Index(watchedKinds[ns2], "Service")).ShouldNot(BeEquivalentTo(-1))
					g.Expect(slices.Index(watchedKinds[ns2], "Deployment")).ShouldNot(BeEquivalentTo(-1))
				})

				By("Removing watch annotation on resources no longer being watched")
				Eventually(ctx, func(g Gomega) {
					Expect(err).To(BeNil())
					deployedList := &appsv1.DeploymentList{}
					svcList := &v1.ServiceList{}

					By("Ensuring deployments in ns1 are not watched")
					Expect(k8sClient.List(ctx, deployedList, client.InNamespace(ns1))).To(Succeed())
					Expect(deployedList.Items).NotTo(BeNil())
					Expect(len(deployedList.Items)).Should(BeNumerically(">=", deployResourceCnt))
					for _, deployment := range deployedList.Items {
						Expect(utils.HasWatchManAnnotation(deployment.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeFalse())
					}

					By("Ensuring services in ns1 are watched")
					Expect(k8sClient.List(ctx, svcList, client.InNamespace(ns1))).To(Succeed())
					Expect(svcList.Items).NotTo(BeNil())
					Expect(len(svcList.Items)).Should(BeNumerically(">=", svcResourceCnt))
					for _, svc := range svcList.Items {
						Expect(utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())
					}

					By("Ensuring both services and deployments in ns2 are watched")
					deployedList = &appsv1.DeploymentList{}
					svcList = &v1.ServiceList{}
					Expect(k8sClient.List(ctx, deployedList, client.InNamespace(ns2))).To(Succeed())
					Expect(deployedList.Items).NotTo(BeNil())
					Expect(len(deployedList.Items)).Should(BeNumerically(">=", deployResourceCnt))
					for _, deployment := range deployedList.Items {
						Expect(utils.HasWatchManAnnotation(deployment.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())
					}
					Expect(k8sClient.List(ctx, svcList, client.InNamespace(ns2))).To(Succeed())
					Expect(svcList.Items).NotTo(BeNil())
					for _, svc := range svcList.Items {
						Expect(utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV)).To(BeTrue())
					}
				}, timeout, interval).Should(Succeed())
			})
		})
	})
})

func testCreateDeployments(deployments ...*appsv1.Deployment) {
	for _, deployment := range deployments {
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())
		Eventually(ctx, func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, &appsv1.Deployment{})).To(Succeed())
		}, timeout, interval).Should(Succeed())
	}
}

func testCreateServices(services ...*v1.Service) {
	for _, svc := range services {
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		Eventually(ctx, func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, &v1.Service{})).To(Succeed())
		}, timeout, interval).Should(Succeed())
	}
}

func testCreateNamespaces(namespaces ...*v1.Namespace) {
	for _, ns := range namespaces {
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		Eventually(ctx, func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name}, &v1.Namespace{})).To(Succeed())
		}, timeout, interval).Should(Succeed())
	}
}

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
