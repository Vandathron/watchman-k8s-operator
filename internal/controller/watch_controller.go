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

	for _, selector := range selectors {
		ns := selector.Namespace
		for _, kind := range selector.Kinds {
			if kind == "Deployment" {
				deployments := &appsv1.DeploymentList{}
				err := r.List(ctx, deployments, client.InNamespace(ns))
				if err != nil {
					log.Error(err, "Failed to fetch resource", "Kind", kind)
					continue
				}

				if err := r.watchDeployments(ctx, deployments); err != nil {
					log.Error(err, "Failed to watch deployment")
					continue
				}

				log.Info("Watching deployment resources in namespace", "Namespace", ns)
			} else {
				log.Error(fmt.Errorf("unsupported resource kind to watch"), "Invalid resource", "Kind", kind)
			}
		}
	}

	return nil
}

func (r *WatchReconciler) watchDeployments(ctx context.Context, deployments *appsv1.DeploymentList) error {

	for _, dep := range deployments.Items {
		_ = r.watchDeployment(ctx, &dep)
	}

	return nil
}

func (r *WatchReconciler) watchDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	log := log.FromContext(ctx)

	if HasWatchManAnnotation(deployment.Annotations) { // no need to update deployment with annotation as it already exists
		return nil
	}

	latestDeployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, latestDeployment)

	if err != nil && errors.IsNotFound(err) {
		log.Info("Failed to get deployment. May have been deleted", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
		return nil // no need to error. Resource may have been deleted
	} else if err != nil {
		log.Error(err, "Failed to get deployment", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
		return err
	}

	latestDeployment.Annotations[watchByAnnotation] = "watchman"

	// TODO: Consider patching
	if err = r.Update(ctx, latestDeployment, &client.UpdateOptions{
		FieldManager: "watch-man-controller",
	}); err != nil {
		log.Error(err, "Failed to update deployment resource", "Name", latestDeployment.Name, "Namespace", latestDeployment.Namespace)
		return err
	}

	return nil
}

func (r *WatchReconciler) watchServices(ctx context.Context, services *v1.ServiceList) error {

	for _, svc := range services.Items {
		_ = r.watchService(ctx, &svc)
	}

	return nil
}

func (r *WatchReconciler) watchService(ctx context.Context, s *v1.Service) interface{} {
	log := log.FromContext(ctx)
	latestSvc := &v1.Service{}
	if HasWatchManAnnotation(s.Annotations) {
		return nil // no need to continue
	}

	if err := r.Get(ctx, types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, latestSvc); err != nil && errors.IsNotFound(err) {
		log.Info("Failed to get service. May have been deleted", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
		return nil
	} else if err != nil {
		log.Error(err, "Failed to get service", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
		return err
	}

	latestSvc.Annotations[watchByAnnotation] = "watchman"

	if err := r.Update(ctx, latestSvc, &client.UpdateOptions{
		FieldManager: "watch-man-controller",
	}); err != nil {
		log.Error(err, "Failed to update service resource", "Name", latestSvc.Name, "Namespace", latestSvc.Namespace)
		return err
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

	deployPredicate := predicate.Funcs{
		CreateFunc: createFunc,

		DeleteFunc: deleteFunc,

		UpdateFunc: r.filterDeployment,
	}

	bldr := ctrl.NewControllerManagedBy(mgr)
	bldr.Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(r.handleDeployment), builder.WithPredicates(deployPredicate))
	bldr.Watches(&v1.Service{}, handler.EnqueueRequestsFromMapFunc(r.handleService), builder.WithPredicates(deployPredicate))

	return bldr.For(&auditv1alpha1.Watch{}).
		Named("watch").
		Complete(r)
}
