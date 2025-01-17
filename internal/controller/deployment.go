package controller

import (
	"context"
	"fmt"
	"github.com/vandathron/watchman/internal/loghandler"
	"github.com/vandathron/watchman/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
)

var (
	oldDeployments = map[string]*appsv1.Deployment{}
)

func (r *WatchReconciler) handleDeployment(ctx context.Context, object client.Object) []reconcile.Request {
	log := log.FromContext(ctx)
	deployment, ok := object.(*appsv1.Deployment)

	if !ok {
		log.Error(fmt.Errorf("object not a deployment type"), "")
		return nil
	}

	action, ok := deployment.Annotations[utils.WatchActionTypeAnnotationKey]
	if !ok {
		log.Error(fmt.Errorf("watch type action label not found"), "No watch type action label specified.")
		return nil
	}

	data := &loghandler.Data{}
	data.AddField("Kind", "Deployment")

	switch action {
	case utils.WatchActionTypeCreate:
		r.Audit.Log("deployment", utils.WatchActionTypeCreate, deployment.Namespace, *data)

	case utils.WatchActionTypeUpdate:
		if utils.HasWatchManAnnotation(deployment.Annotations, utils.WatchUpdateStateKey, utils.WatchUpdateStateOld) {
			log.Info(" old deployment, should save temporarily")
			oldDeployments[fmt.Sprintf("%s:%s", deployment.Namespace, deployment.Name)] = deployment.DeepCopy()
			return nil
		} else if utils.HasWatchManAnnotation(deployment.Annotations, utils.WatchUpdateStateKey, utils.WatchUpdateStateNew) {
			// find old deployment
			log.Info("found new deployment, searching for pair/old")
			var oldDeployment = oldDeployments[fmt.Sprintf("%s:%s", deployment.Namespace, deployment.Name)]
			if oldDeployment == nil {
				log.Error(fmt.Errorf("old deployment not found"), "Old deployment not found for new deployment", "Name", deployment.Name, "Namespace", deployment.Namespace)
				return nil
			}
			r.recordDeploymentDiff(ctx, oldDeployment, deployment, data)
			r.Audit.Log(deployment.Name, utils.WatchActionTypeUpdate, deployment.Namespace, *data)
		} else {
			log.Error(fmt.Errorf("annotation not found"), fmt.Sprintf("%s not found", utils.WatchUpdateStateKey))
			return nil
		}

	case utils.WatchActionTypeDelete:
		r.Audit.Log(deployment.Name, utils.WatchActionTypeDelete, deployment.Namespace, *data)
	default:
		log.Error(fmt.Errorf("invalid action type"), "Unsupported action type", "Type", action)
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

	if utils.HasWatchManAnnotation(deployment.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) { // no need to update deployment with annotation as it already exists
		return
	}

	latestDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, latestDeployment); err != nil {
		log.Error(err, "Failed to get deployment", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
		return
	}

	if latestDeployment.Annotations == nil {
		latestDeployment.Annotations = map[string]string{}
	}
	latestDeployment.Annotations[utils.WatchByAnnotationKey] = utils.WatchByAnnotationKV

	// TODO: Consider patching
	if err := r.Update(ctx, latestDeployment, &client.UpdateOptions{
		FieldManager: utils.WatchManFieldManager,
	}); err != nil {
		log.Error(err, "Failed to update deployment resource", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
	}
}

func (r *WatchReconciler) unWatchDeployments(ctx context.Context, deployment *appsv1.DeploymentList) {
	for _, dep := range deployment.Items {
		r.unWatchDeployment(ctx, &dep)
	}
}

func (r *WatchReconciler) unWatchDeployment(ctx context.Context, deployment *appsv1.Deployment) {
	log := log.FromContext(ctx)

	if !utils.HasWatchManAnnotation(deployment.Annotations, utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) { // annotation not found on resource, skip
		return
	}

	latestDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, latestDeployment); err != nil {
		log.Error(err, "Failed to get deployment", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
		return
	}

	delete(latestDeployment.Annotations, utils.WatchByAnnotationKey)

	// TODO: Consider patching
	if err := r.Update(ctx, latestDeployment, &client.UpdateOptions{
		FieldManager: utils.WatchManFieldManager,
	}); err != nil {
		log.Error(err, "Failed to update deployment resource", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
	}
}

func (r *WatchReconciler) recordDeploymentDiff(ctx context.Context, old, new *appsv1.Deployment, data *loghandler.Data) {
	// Compare replicas
	log := log.FromContext(ctx)
	if old.Spec.Replicas != new.Spec.Replicas {
		data.AddField("replica", strconv.Itoa(int(*new.Spec.Replicas)))
	}
	if old.Spec.Paused != new.Spec.Paused {
		data.AddField("Paused", fmt.Sprintf("%v", new.Spec.Replicas))
	}
	if old.Spec.MinReadySeconds != new.Spec.MinReadySeconds {
		data.AddField("MinReadySeconds", strconv.Itoa(int(new.Spec.MinReadySeconds)))
	}
	if old.Spec.ProgressDeadlineSeconds != new.Spec.ProgressDeadlineSeconds {
		data.AddField("progressDeadlineSeconds", strconv.Itoa(int(*new.Spec.ProgressDeadlineSeconds)))
	}
	if old.Spec.RevisionHistoryLimit != new.Spec.RevisionHistoryLimit {
		data.AddField("revisionHistoryLimit", strconv.Itoa(int(*new.Spec.RevisionHistoryLimit)))
	}

	if err := utils.RecordChanges(*old.Spec.Selector, *new.Spec.Selector, ".spec.selector.", data); err != nil {
		log.Error(err, "record change error")
	}
	if err := utils.RecordChanges(old.Spec.Template, new.Spec.Template, "spec.template.", data); err != nil {
		log.Error(err, "record change error")
	}
	if err := utils.RecordChanges(old.Spec.Template.Spec, new.Spec.Template.Spec, "spec.template.spec.", data); err != nil {
		log.Error(err, "record change error")
	}
}

func (r *WatchReconciler) filterDeployments(e event.TypedUpdateEvent[client.Object]) bool {
	if !utils.HasWatchManAnnotation(e.ObjectOld.GetAnnotations(), utils.WatchByAnnotationKey, utils.WatchByAnnotationKV) {
		return false
	}

	if oldDeployment, ok := e.ObjectOld.(*appsv1.Deployment); ok {
		newDeployment := e.ObjectNew.(*appsv1.Deployment)
		oldDeployment.Annotations[utils.WatchUpdateStateKey] = utils.WatchUpdateStateOld
		oldDeployment.Annotations[utils.WatchActionTypeAnnotationKey] = utils.WatchActionTypeUpdate

		newDeployment.Annotations[utils.WatchActionTypeAnnotationKey] = utils.WatchActionTypeUpdate
		newDeployment.Annotations[utils.WatchUpdateStateKey] = utils.WatchUpdateStateNew

		return reflect.DeepEqual(oldDeployment.Spec, newDeployment.Spec) == false
	}

	return false
}
