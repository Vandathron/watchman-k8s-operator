package controller

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *WatchReconciler) handleDeployment(ctx context.Context, object client.Object) []reconcile.Request {
	return nil
}

func (r *WatchReconciler) diff(ctx context.Context, old, new *appsv1.Deployment, data *audit.Data) {
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
