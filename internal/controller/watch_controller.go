// SetupWithManager sets up the controller with the Manager.
func (r *WatchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Watch Deployments with watchman annotation
	deployPredicate := predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			if val, ok := e.Object.GetAnnotations()[watchByAnnotation]; ok && val == "watchman" {
				return true
			}
			return false
		},

		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			if val, ok := e.Object.GetAnnotations()[watchByAnnotation]; ok && val == "watchman" {
				return true
			}
			return false
		},

		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			val, hasAnnotation := e.ObjectOld.GetAnnotations()[watchByAnnotation]
			if !(hasAnnotation && val == "watchman") {
				return false
			}

			if oldDeployment, ok := e.ObjectOld.(*appsv1.Deployment); ok {
				newDeployment := e.ObjectNew.(*appsv1.Deployment)
				return hasAnnotation &&
					(reflect.DeepEqual(oldDeployment.Spec, newDeployment.Spec) == false ||
						(reflect.DeepEqual(oldDeployment.ObjectMeta, newDeployment.ObjectMeta) == false))
			}

			return false
		},
	}

	bldr := ctrl.NewControllerManagedBy(mgr)
	bldr.Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: object.GetNamespace(),
					Name:      object.GetName(),
				},
			},
		}
	}), builder.WithPredicates(deployPredicate))

	return bldr.For(&auditv1alpha1.Watch{}).
		Named("watch").
		Complete(r)
}
