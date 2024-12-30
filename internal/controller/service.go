package controller

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *WatchReconciler) handleService(ctx context.Context, object client.Object) []reconcile.Request {

	return nil
}
