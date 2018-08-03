package main

import (
	"log"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Watch uses the Kubernetes informer API to watch events for the API server.
func (r *ReverseProxy) Watch() (err error) {
	kubeconfig := "/etc/kubernetes/admin.conf"

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}

	restclient := clientset.CoreV1().RESTClient()
	watchlist := cache.NewListWatchFromClient(restclient, "pods", "kube-system", fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		time.Minute*5,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    r.AddFunc(),
			DeleteFunc: r.DeleteFunc(),
			UpdateFunc: r.UpdateFunc(),
		},
	)
	stop := make(chan struct{})
	controller.Run(stop)

	return nil
}

// AddFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) AddFunc() func(obj interface{}) {
	return func(obj interface{}) {
		pod := obj.(*v1.Pod)

		if !isAPIServer(pod) {
			return
		}

		// We need an IP address to register.
		if pod.Status.PodIP == "" {
			return
		}

		for _, status := range pod.Status.ContainerStatuses {
			if !status.Ready {
				log.Printf("pod %s container %s is not ready", pod.Name, status.Name)
				// Return early in the case of any container not being
				// ready.
				return
			}
		}

		r.AddBackend(pod.Status.PodIP)

		log.Printf("registered API server %s with IP: %s", pod.Name, pod.Status.PodIP)
	}
}

// UpdateFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) UpdateFunc() func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		old := oldObj.(*v1.Pod)
		new := newObj.(*v1.Pod)

		if !isAPIServer(old) {
			return
		}

		for _, status := range new.Status.ContainerStatuses {
			if !status.Ready {
				log.Printf("pod %s container %s is not ready", old.Name, status.Name)
				r.DeleteBackend(old.Status.PodIP)
				log.Printf("deregistered unhealthy API server %s with IP: %s", old.Name, old.Status.PodIP)
				break
			}
		}

		// We need an IP address to register.
		if old.Status.PodIP == "" && new.Status.PodIP != "" {
			r.AddBackend(new.Status.PodIP)
			log.Printf("registered API server %s with IP: %s", new.Name, new.Status.PodIP)
		}
	}
}

// DeleteFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) DeleteFunc() func(obj interface{}) {
	return func(obj interface{}) {
		pod := obj.(*v1.Pod)

		if !isAPIServer(pod) {
			return
		}

		r.DeleteBackend(pod.Status.PodIP)

		log.Printf("deregistered API server %s with IP: %s", pod.Name, pod.Status.PodIP)
	}
}

func isAPIServer(pod *v1.Pod) bool {
	// This is used for non-self-hosted deployments.
	if component, ok := pod.Labels["component"]; ok {
		if component == "kube-apiserver" {
			return true
		}
	}
	// This is used for self-hosted deployments.
	if k8sApp, ok := pod.Labels["k8s-app"]; ok {
		if k8sApp == "self-hosted-kube-apiserver" {
			return true
		}
	}

	return false
}
