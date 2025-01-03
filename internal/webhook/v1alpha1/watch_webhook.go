// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Watch.
func (d *WatchCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	watch, ok := obj.(*auditv1alpha1.Watch)

	if !ok {
		return fmt.Errorf("expected an Watch object but got %T", obj)
	}
	watchlog.Info("Defaulting for Watch", "name", watch.GetName())

	// selectors with no kinds is assumed to watch all resources in that namespace
	// add all supported resources if kinds is empty
	for _, selector := range watch.Spec.Selectors {
		if len(selector.Kinds) == 0 {
			selector.Kinds = []string{utils.SupportedKindService, utils.SupportedKindDeployment}
		}
	}

	return nil
}
