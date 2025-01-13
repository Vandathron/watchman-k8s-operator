func makeDeploymentSpec(name, ns string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app.kubernetes.io/name": name},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/name": name},
				},

				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image:   "busybox",
						Name:    "busybox",
						Command: []string{"sh", "-c", "sleep 1000"},
					}},
				},
			},
		},
	}
}

func makeSvcSpec(name, ns string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
			Ports: []v1.ServicePort{
				{Port: 9090},
			},
		},
	}
}

func makeNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
