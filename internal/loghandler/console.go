package loghandler

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Console struct {
}

func NewConsole() *Console {
	return &Console{}
}

func (c *Console) Audit(resource, action, namespace string, data Data) {
	message := fmt.Sprintf("Resource::%s, action::%s, namespace::%s, %v", resource, action, namespace, data.fields)
	log.FromContext(context.Background()).Info(message)
}
