package controller

import (
	"context"
	"fmt"
	auditv1alpha1 "github.com/vandathron/watchman/api/v1alpha1"
	"github.com/vandathron/watchman/internal/audit"
	"github.com/vandathron/watchman/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	audit2 "k8s.io/apiserver/pkg/audit"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	watchByAnnotation = "audit.my.domain/watch-by"
	watchActionType   = "audit.my.domain/watch-action"
)

// WatchReconciler reconciles a Watch object
type WatchReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Audit  audit.Provider
}

// +kubebuilder:rbac:groups=audit.my.domain,resources=watches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=audit.my.domain,resources=watches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=audit.my.domain,resources=watches/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;update;patch;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update

func (r *WatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	watch := &auditv1alpha1.Watch{}
	err := r.Get(ctx, req.NamespacedName, watch)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Watch resource was not found", "Namespace", req.Namespace, "Name", req.Name)
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Fails to get resource", "Namespace", req.Namespace, "Name", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.reconcileWatchManResource(ctx, watch)
}

func (r *WatchReconciler) reconcileWatchManResource(ctx context.Context, watch *auditv1alpha1.Watch) error {
	log := log.FromContext(ctx)
	selectors := watch.Spec.Selectors
	a := audit2.AuditContextFrom(ctx)
	log.Info("Audit", "Audit", a)

	for _, selector := range selectors {
		ns := selector.Namespace
		for _, kind := range selector.Kinds {
			if kind == "Deployment" {
				deployments := &appsv1.DeploymentList{}
				if err := r.List(ctx, deployments, client.InNamespace(ns)); err != nil {
					log.Error(err, "Failed to fetch deployments")
					continue
				}

				r.watchDeployments(ctx, deployments)
				continue
			}

			if kind == "Service" {
				services := &v1.ServiceList{}
				err := r.List(ctx, services, client.InNamespace(ns))

				if err != nil {
					log.Error(err, "Failed to fetch services")
				}
				continue
			}

			log.Error(fmt.Errorf("invalid kind"), "Unsupported kind", "Kind", kind)
		}
	}

	return nil
}

func (r *WatchReconciler) watchDeployments(ctx context.Context, deployments *appsv1.DeploymentList) {
	for _, dep := range deployments.Items {
		r.watchDeployment(ctx, &dep)
	}
}

func (r *WatchReconciler) watchDeployment(ctx context.Context, deployment *appsv1.Deployment) {
	log := log.FromContext(ctx)

	if utils.HasWatchManAnnotation(deployment.Annotations, watchByAnnotation, utils.WatchByAnnotationKV) { // no need to update deployment with annotation as it already exists
		return
	}

	latestDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, latestDeployment); err != nil {
		log.Error(err, "Failed to get deployment", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
		return
	}

	latestDeployment.Annotations[watchByAnnotation] = utils.WatchByAnnotationKV

	// TODO: Consider patching
	if err := r.Update(ctx, latestDeployment, &client.UpdateOptions{
		FieldManager: utils.WatchManFieldManager,
	}); err != nil {
		log.Error(err, "Failed to update deployment resource", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
	}

}

func (r *WatchReconciler) watchServices(ctx context.Context, services *v1.ServiceList) {
	for _, svc := range services.Items {
		r.watchService(ctx, &svc)
	}
}

func (r *WatchReconciler) watchService(ctx context.Context, s *v1.Service) {
	log := log.FromContext(ctx)
	latestSvc := &v1.Service{}

	if utils.HasWatchManAnnotation(s.Annotations, watchByAnnotation, utils.WatchByAnnotationKV) {
		return // no need to continue
	}

	if err := r.Get(ctx, types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, latestSvc); err != nil {
		log.Error(err, "Failed to get service", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
		return
	}

	latestSvc.Annotations[watchByAnnotation] = utils.WatchByAnnotationKV

	if err := r.Update(ctx, latestSvc, &client.UpdateOptions{
		FieldManager: utils.WatchManFieldManager,
	}); err != nil {
		log.Error(err, "Failed to update service resource", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Watch Deployments with watchman annotation
	createFunc := func(e event.TypedCreateEvent[client.Object]) bool {
		annotations := e.Object.GetAnnotations()
		if utils.HasWatchManAnnotation(annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) {
			annotations[utils.WatchActionTypeAnnotationKey] = utils.WatchActionTypeCreate
			e.Object.SetAnnotations(annotations)
			return true
		}
		return false
	}

	deleteFunc := func(e event.TypedDeleteEvent[client.Object]) bool {
		annotations := e.Object.GetAnnotations()

		if utils.HasWatchManAnnotation(annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) {
			annotations[utils.WatchActionTypeAnnotationKey] = utils.WatchActionTypeUpdate
			e.Object.SetAnnotations(annotations)
			return true
		}
		return false
	}

	deployPredicate := predicate.Funcs{CreateFunc: createFunc, DeleteFunc: deleteFunc, UpdateFunc: r.filterDeployments}
	svcPredicate := predicate.Funcs{CreateFunc: createFunc, DeleteFunc: deleteFunc, UpdateFunc: r.filterServices}

	bldr := ctrl.NewControllerManagedBy(mgr)
	bldr.Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.handleDeployment), builder.WithPredicates(deployPredicate))
	bldr.Watches(&v1.Service{}, handler.EnqueueRequestsFromMapFunc(r.handleService), builder.WithPredicates(svcPredicate))

	return bldr.For(&auditv1alpha1.Watch{}).
		Named("watch").
		Complete(r)
}
