package v1alpha1

import (
	"context"
	"fmt"
	"github.com/vandathron/watchman/internal/utils"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	auditv1alpha1 "github.com/vandathron/watchman/api/v1alpha1"
)

// nolint:unused
var watchlog = logf.Log.WithName("watch-resource")

// SetupWatchWebhookWithManager registers the webhook for Watch in the manager.
func SetupWatchWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&auditv1alpha1.Watch{}).
		WithValidator(&WatchCustomValidator{}).
		WithDefaulter(&WatchCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-audit-my-domain-v1alpha1-watch,mutating=true,failurePolicy=fail,sideEffects=None,groups=audit.my.domain,resources=watches,verbs=create;update,versions=v1alpha1,name=mwatch-v1alpha1.kb.io,admissionReviewVersions=v1

// WatchCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Watch when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type WatchCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Watch.
func (d *WatchCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	watch, ok := obj.(*auditv1alpha1.Watch)

	if !ok {
		return fmt.Errorf("expected a watch object but got %T", obj)
	}
	watchlog.Info("Defaulting for Watch", "name", watch.GetName())

	// selectors with no kinds is assumed to watch all resources in that namespace
	// add all supported resources if kinds is empty
	for i, selector := range watch.Spec.Selectors {
		if len(selector.Kinds) == 0 {
			watch.Spec.Selectors[i].Kinds = []string{utils.SupportedKindService, utils.SupportedKindDeployment}
		}
	}

	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-audit-my-domain-v1alpha1-watch,mutating=false,failurePolicy=fail,sideEffects=None,groups=audit.my.domain,resources=watches,verbs=create;update,versions=v1alpha1,name=vwatch-v1alpha1.kb.io,admissionReviewVersions=v1

// WatchCustomValidator struct is responsible for validating the Watch resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type WatchCustomValidator struct {
	//TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &WatchCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Watch.
func (v *WatchCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	watch, ok := obj.(*auditv1alpha1.Watch)
	if !ok {
		return nil, fmt.Errorf("expected a Watch object but got %T", obj)
	}
	watchlog.Info("Validation for Watch upon creation", "name", watch.GetName())

	if len(watch.Spec.Selectors) == 0 {
		return nil, fmt.Errorf("selector can not be empty. Should contain at least a namespace")
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Watch.
func (v *WatchCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	watch, ok := newObj.(*auditv1alpha1.Watch)
	if !ok {
		return nil, fmt.Errorf("expected a Watch object for the newObj but got %T", newObj)
	}
	watchlog.Info("Validation for Watch upon update", "name", watch.GetName())

	if len(watch.Spec.Selectors) == 0 {
		return nil, fmt.Errorf("selector can not be empty. Should contain at least a namespace")
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Watch.
func (v *WatchCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
