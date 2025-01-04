package controller

import (
	"context"
	"fmt"
	"github.com/vandathron/watchman/internal/audit"
	"github.com/vandathron/watchman/internal/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	oldSvcs = map[string]*v1.Service{}
)

func (r *WatchReconciler) unWatchServices(ctx context.Context, serviceList *v1.ServiceList) {
	for _, svc := range serviceList.Items {
		r.unWatchService(ctx, &svc)
	}
}

func (r *WatchReconciler) unWatchService(ctx context.Context, svc *v1.Service) {
	log := log.FromContext(ctx)
	latestSvc := &v1.Service{}

	if !utils.HasWatchManAnnotation(svc.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) {
		return // annotation not found on resource, skip
	}

	if err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, latestSvc); err != nil {
		log.Error(err, "Failed to get service", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
		return
	}

	delete(latestSvc.Annotations, utils.WatchByAnnotationKey) // remove annotation

	if err := r.Update(ctx, latestSvc, &client.UpdateOptions{
		FieldManager: utils.WatchManFieldManager,
	}); err != nil {
		log.Error(err, "Failed to update service resource", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
	}
}

func (r *WatchReconciler) handleService(ctx context.Context, object client.Object) []reconcile.Request {
	log := log.FromContext(ctx)
	svc, ok := object.(*v1.Service)

	if !ok {
		log.Error(fmt.Errorf("object not a service type"), "")
		return nil
	}

	action, ok := svc.Annotations[utils.WatchActionTypeAnnotationKey]
	if !ok {
		log.Error(fmt.Errorf("watch type action label not found"), "No watch type action label specified.")
		return nil
	}

	data := &audit.Data{}
	data.AddField("Kind", "Service")

	switch action {
	case utils.WatchActionTypeCreate:
		r.Audit.Audit(svc.Name, utils.WatchActionTypeCreate, svc.Namespace, *data)

	case utils.WatchActionTypeUpdate:
		if utils.HasWatchManAnnotation(svc.Annotations, utils.WatchUpdateStateKey, utils.WatchUpdateStateOld) {
			log.Info(" old svc, should save temporarily")
			oldSvcs[fmt.Sprintf("%s:%s", svc.Namespace, svc.Name)] = svc.DeepCopy()
			return nil
		} else if utils.HasWatchManAnnotation(svc.Annotations, utils.WatchUpdateStateKey, utils.WatchUpdateStateNew) {
			// find old svc
			log.Info("found new svc, searching for pair/old")
			var oldSvc = oldSvcs[fmt.Sprintf("%s:%s", svc.Namespace, svc.Name)]

			if oldSvc == nil {
				log.Error(fmt.Errorf("old svc not found"), "Old svc not found", "Name", svc.Name, "Namespace", svc.Namespace)
				return nil
			}
			r.recordSvcDiff(ctx, oldSvc, svc, data)
			r.Audit.Audit(svc.Name, utils.WatchActionTypeUpdate, svc.Namespace, *data)
		} else {
			log.Error(fmt.Errorf("annotation not found"), fmt.Sprintf("%s not found", utils.WatchUpdateStateKey))
			return nil
		}

	case utils.WatchActionTypeDelete:
		r.Audit.Audit(svc.Name, utils.WatchActionTypeDelete, svc.Namespace, *data)

	default:
		log.Error(fmt.Errorf("invalid action type"), "Unsupported action type", "Type", action)
	}

	return nil
}

func (r *WatchReconciler) recordSvcDiff(ctx context.Context, old, new *v1.Service, data *audit.Data) {
	// Compare replicas
	log := log.FromContext(ctx)

	if err := utils.RecordChanges(old.Spec, new.Spec, ".spec.selector.", data); err != nil {
		log.Error(err, "record change error")
	}
}

func (r *WatchReconciler) filterServices(e event.TypedUpdateEvent[client.Object]) bool {
	if !utils.HasWatchManAnnotation(e.ObjectOld.GetAnnotations(), utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) {
		return false
	}

	if oldDeployment, ok := e.ObjectOld.(*v1.Service); ok {
		newDeployment := e.ObjectNew.(*v1.Service)
		oldDeployment.Annotations[utils.WatchUpdateStateKey] = utils.WatchUpdateStateOld
		oldDeployment.Annotations[utils.WatchActionTypeAnnotationKey] = utils.WatchActionTypeUpdate

		newDeployment.Annotations[utils.WatchActionTypeAnnotationKey] = utils.WatchActionTypeUpdate
		newDeployment.Annotations[utils.WatchUpdateStateKey] = utils.WatchUpdateStateNew

		return reflect.DeepEqual(oldDeployment.Spec, newDeployment.Spec) == false
	}

	return false
}
