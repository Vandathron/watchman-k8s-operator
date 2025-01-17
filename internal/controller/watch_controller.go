package controller

import (
	"context"
	"fmt"
	auditv1alpha1 "github.com/vandathron/watchman/api/v1alpha1"
	"github.com/vandathron/watchman/internal/loghandler"
	"github.com/vandathron/watchman/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"strings"
)

// WatchReconciler reconciles a Watch object
type WatchReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Audit  loghandler.Provider
}

// +kubebuilder:rbac:groups=audit.my.domain,resources=watches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=audit.my.domain,resources=watches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=audit.my.domain,resources=watches/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;update;patch;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;create

func (r *WatchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	watch := &auditv1alpha1.Watch{}
	err := r.Get(ctx, req.NamespacedName, watch)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Watch resource deleted", "Namespace", req.Namespace, "Name", req.Name)
		return ctrl.Result{}, r.cleanUp(ctx)
	} else if err != nil {
		log.Error(err, "Fails to get resource", "Namespace", req.Namespace, "Name", req.Name)
		return ctrl.Result{}, err
	}

	cm := &v1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: watch.Name, Namespace: watch.Namespace}, cm)

	if err != nil && errors.IsNotFound(err) { // Watch cm deleted or may not have been created
		cm, err = r.prepareConfigMapForResource(ctx, watch)

		if err != nil {
			log.Error(err, "Failed to prepare config map for watch resource")
			return ctrl.Result{}, err
		}

		if err = r.Create(ctx, cm); err != nil {
			log.Error(err, "Failed to create watch man config map", "Name", cm.Name, "Namespace", cm.Namespace)
			// TODO: Track status
			return ctrl.Result{}, err
		}

	} else if err != nil {
		log.Error(err, "Failed to fetch watch man config map", "Name", watch.Name, "Namespace", watch.Namespace)
		return ctrl.Result{}, err
	}

	if err := r.reconcileWatchManResource(ctx, watch, utils.ExtractWatchedKindsFromCM(cm.Data)); err != nil {
		log.Error(err, "Failed to update watch resource")
		return ctrl.Result{}, err
	}

	if cm.Data == nil {
		cm.Data = map[string]string{}
	}

	for _, selector := range watch.Spec.Selectors {
		cm.Data[selector.Namespace] = strings.Join(selector.Kinds, ",")
	}

	if err = r.Update(ctx, cm); err != nil {
		log.Error(err, "Failed to update watch config map resource", "Name", cm.Name, "Namespace", cm.Namespace)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WatchReconciler) prepareConfigMapForResource(ctx context.Context, watch *auditv1alpha1.Watch) (*v1.ConfigMap, error) {

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watch.Name,
			Namespace: watch.Namespace,
		},
		Data: map[string]string{},
	}

	if err := ctrl.SetControllerReference(watch, cm, r.Scheme); err != nil {
		return nil, err
	}
	return cm, nil
}

func (r *WatchReconciler) reconcileWatchManResource(ctx context.Context, watch *auditv1alpha1.Watch, watching map[string][]string) error {
	log := log.FromContext(ctx)
	toUnWatch := map[string][]string{}
	toWatch := map[string][]string{}

	a := audit2.AuditContextFrom(ctx)
	for _, selector := range watch.Spec.Selectors {
		toWatch[selector.Namespace] = append([]string{}, selector.Kinds...)
	}

	log.Info("Audit", "Audit", a)

	for ns, kinds := range watching { // loop through resources being watched
		toWatchKinds, found := toWatch[ns]
		if !found { // unwatch all resources in namespace ns if ns no longer present in latest crd/toWatch
			toUnWatch[ns] = watching[ns]
			continue
		}

		toUnWatch[ns] = []string{}
		toWatchKindSet := map[string]struct{}{}
		for _, kind := range toWatchKinds {
			toWatchKindSet[kind] = struct{}{}
		}

		for _, kind := range kinds {
			if _, ok := toWatchKindSet[kind]; !ok { // compare kinds, unwatch kinds no longer present in latest crd/toWatch in namespace ns
				toUnWatch[ns] = append(toUnWatch[ns], kind)
				continue
			}

			// if kind is present in both kinds to watch and kinds already being watched, remove it from to watch
			// to avoid making unnecessary api calls
			//kindIdx := slices.Index(toWatch[ns], kind)
			//if kindIdx == -1 {
			//	err := fmt.Errorf("incorrect implementation")
			//	log.Error(err, "Incorrect implementation. Kind is expected to be present in slice")
			//	return err
			//}
			//
			//toWatch[ns] = append(toWatch[ns][:kindIdx], toWatch[ns][kindIdx+1:]...)
		}
	}

	for ns, kinds := range toWatch {
		for _, kind := range kinds {
			if kind == "Deployment" {
				deploymentList := &appsv1.DeploymentList{}
				if err := r.List(ctx, deploymentList, client.InNamespace(ns)); err != nil || deploymentList.Items == nil {
					log.Error(err, "Failed to fetch deployments. No deployment perhaps")
					continue
				}

				r.watchDeployments(ctx, deploymentList)
				continue
			}

			if kind == "Service" {
				svcList := &v1.ServiceList{}
				if err := r.List(ctx, svcList, client.InNamespace(ns)); err != nil || svcList.Items == nil {
					log.Error(err, "Failed to fetch services. No services perhaps")
					continue
				}
				r.watchServices(ctx, svcList)
				continue
			}

			log.Error(fmt.Errorf("invalid kind"), "Unsupported kind", "Kind", kind)
		}
	}

	for ns, kinds := range toUnWatch {
		for _, kind := range kinds {
			if kind == "Deployment" {
				deployments := &appsv1.DeploymentList{}
				if err := r.List(ctx, deployments, client.InNamespace(ns)); err != nil {
					log.Error(err, "Failed to fetch deployments")
					continue
				}

				r.unWatchDeployments(ctx, deployments)
				continue
			}

			if kind == "Service" {
				services := &v1.ServiceList{}
				err := r.List(ctx, services, client.InNamespace(ns))

				if err != nil {
					log.Error(err, "Failed to fetch services")
				}
				r.unWatchServices(ctx, services)
				continue
			}
		}
	}

	return nil
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
