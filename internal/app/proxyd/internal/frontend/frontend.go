/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package frontend

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/backend"
	tnet "github.com/talos-systems/talos/pkg/net"
)

// ReverseProxy represents a reverse proxy server.
type ReverseProxy struct {
	ConnectTimeout  int
	current         *backend.Backend
	backends        map[string]*backend.Backend
	endpoints       []string
	cancelBootstrap context.CancelFunc
	mux             *sync.Mutex
}

// NewReverseProxy initializes a ReverseProxy.
func NewReverseProxy(endpoints []string, bCancel context.CancelFunc) (r *ReverseProxy, err error) {
	r = &ReverseProxy{
		ConnectTimeout:  100,
		mux:             &sync.Mutex{},
		backends:        map[string]*backend.Backend{},
		endpoints:       endpoints,
		cancelBootstrap: bCancel,
	}

	return r, nil
}

// Listen starts the server on the specified address.
func (r *ReverseProxy) Listen(address string) (err error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	log.Printf("listening on %v", l.Addr())

	for {
		conn, err := l.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok {
				if e.Temporary() {
					continue
				}
			}
		}

		go r.proxyConnection(conn)
	}
}

// AddBackend adds a backend.
func (r *ReverseProxy) AddBackend(uid, addr string) (added bool) {
	r.mux.Lock()
	defer r.mux.Unlock()

	// Check if we already have this backend added
	if _, ok := r.backends[uid]; ok {
		added = false
		return
	}

	// Add new backend
	r.backends[uid] = &backend.Backend{UID: uid, Addr: addr, Connections: 0}
	added = true

	// Update current backend
	r.setCurrent()

	return added
}

// DeleteBackend deletes a backend.
func (r *ReverseProxy) DeleteBackend(uid string) (deleted bool) {
	r.mux.Lock()
	defer r.mux.Unlock()
	if _, ok := r.backends[uid]; ok {
		delete(r.backends, uid)
		deleted = true
	}
	r.setCurrent()

	return deleted
}

// GetBackend gets a backend based on least connections algorithm.
func (r *ReverseProxy) GetBackend() (backend *backend.Backend) {
	return r.current
}

// IncrementBackend increments the connections count of a backend. nolint: dupl
func (r *ReverseProxy) IncrementBackend(uid string) {
	r.mux.Lock()
	defer r.mux.Unlock()
	if _, ok := r.backends[uid]; !ok {
		return
	}
	r.backends[uid].Connections++
	r.setCurrent()
}

// DecrementBackend deccrements the connections count of a backend. nolint: dupl
func (r *ReverseProxy) DecrementBackend(uid string) {
	r.mux.Lock()
	defer r.mux.Unlock()
	if _, ok := r.backends[uid]; !ok {
		return
	}
	// Avoid setting the connections to the max uint32 value.
	if r.backends[uid].Connections == 0 {
		return
	}
	r.backends[uid].Connections--
	r.setCurrent()
}

// Watch uses the Kubernetes informer API to watch events for the API server.
func (r *ReverseProxy) Watch(kubeClient kubernetes.Interface) (err error) {
	// Filter for only apiservers
	labelSelector := labels.FormatLabels(map[string]string{"component": "kube-apiserver"})
	watchlist := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fields.Everything().String()
			options.LabelSelector = labelSelector
			return kubeClient.CoreV1().Pods(metav1.NamespaceSystem).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fields.Everything().String()
			options.LabelSelector = labelSelector
			return kubeClient.CoreV1().Pods(metav1.NamespaceSystem).Watch(options)
		},
	}

	// Use a 1m cache refresh to make sure if there was a network interruption
	// we can quickly :tm: add the backends back once the network has recovered
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		time.Minute*1,
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
		pod, ok := obj.(*v1.Pod)
		if !ok {
			return
		}

		switch {
		case pod.Status.PodIP == "":
			// We need an IP address to register.
			return
		case pod.Status.Phase != v1.PodRunning:
			// Return early if the pod is not running.
			return
		}

		// Return early in the case of any container not being ready.
		for _, status := range pod.Status.ContainerStatuses {
			if !status.Ready {
				return
			}
		}

		if added := r.AddBackend(string(pod.UID), pod.Status.PodIP); added {
			log.Printf("registered API server %s (UID: %q) with IP: %s", pod.Name, pod.UID, pod.Status.PodIP)
		}

		r.stopBootstrapBackends()
	}
}

// UpdateFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) UpdateFunc() func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		newPod, ok := newObj.(*v1.Pod)
		if !ok {
			return
		}

		// Remove the backend if any container is not ready.
		for _, status := range newPod.Status.ContainerStatuses {
			if !status.Ready {
				r.DeleteFunc()(oldObj)
				return
			}
		}

		// Refresh backend
		r.AddFunc()(newObj)
	}
}

// DeleteFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) DeleteFunc() func(obj interface{}) {
	return func(obj interface{}) {
		// nolint: errcheck
		pod := obj.(*v1.Pod)

		if deleted := r.DeleteBackend(string(pod.UID)); deleted {
			log.Printf("deregistered API server %s (UID: %q) with IP: %s", pod.Name, pod.UID, pod.Status.PodIP)
		}
	}
}

func (r *ReverseProxy) setCurrent() {
	least := uint32(math.MaxUint32)
	for _, b := range r.backends {
		switch {
		case b.Connections == 0:
			// If backend has no connections, we don't
			// need to go further
			r.current = b
			return
		case b.Connections < least:
			least = b.Connections
			r.current = b
		}
	}
}

func (r *ReverseProxy) proxyConnection(c1 net.Conn) {
	backend := r.GetBackend()
	if backend == nil {
		log.Printf("no available backend, closing remote connection: %s", c1.RemoteAddr().String())
		// nolint: errcheck
		c1.Close()
		return
	}

	c2, err := net.DialTimeout("tcp", tnet.FormatAddress(backend.Addr)+":6443", time.Duration(r.ConnectTimeout)*time.Millisecond)
	if err != nil {
		log.Printf("dial %v failed, deleting backend: %v", backend.Addr, err)
		r.DeleteBackend(backend.UID)
		r.proxyConnection(c1)
		return
	}

	// Ensure the connections are valid.
	if c1 == nil || c2 == nil {
		return
	}

	r.IncrementBackend(backend.UID)

	r.joinConnections(backend.UID, c1, c2)
}

func (r *ReverseProxy) joinConnections(uid string, c1 net.Conn, c2 net.Conn) {
	defer r.DecrementBackend(uid)

	tcp1, ok := c1.(*net.TCPConn)
	if !ok {
		return
	}
	tcp2, ok := c2.(*net.TCPConn)
	if !ok {
		return
	}

	log.Printf("%s -> %s", c1.RemoteAddr(), c2.RemoteAddr())

	var wg sync.WaitGroup
	join := func(dst *net.TCPConn, src *net.TCPConn) {
		// Close after the copy to avoid a deadlock.
		// nolint: errcheck
		defer dst.CloseRead()
		defer wg.Done()
		_, err := io.Copy(dst, src)
		if err != nil {
			log.Printf("%v", err)
		}
	}

	wg.Add(2)
	go join(tcp1, tcp2)
	go join(tcp2, tcp1)
	wg.Wait()
}

func (r *ReverseProxy) stopBootstrapBackends() {
	if r.endpoints != nil {
		r.cancelBootstrap()
		r.endpoints = nil
	}
}

// Bootstrap handles the initial startup phase of proxyd
// nolint: gocyclo
func (r *ReverseProxy) Bootstrap(ctx context.Context) {
	for idx, endpoint := range r.endpoints {
		go func(c context.Context, i int, e string) {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			uid := fmt.Sprintf("bootstrap-%d", i)
			addr := net.JoinHostPort(e, "6443")
			for {
				select {
				case <-ticker.C:
					conn, err := net.Dial("tcp", addr)
					if err != nil {
						log.Printf("failed to dial bootstrap backend: %+v\n", err)
						// Remove backend.
						if deleted := r.DeleteBackend(uid); deleted {
							log.Printf("deregistered bootstrap backend with IP: %s", e)
						}
						continue
					}
					// Add backend.
					if added := r.AddBackend(uid, e); added {
						log.Printf("registered bootstrap backend with IP: %s", e)
					}

					// We intentionally do not defer closing the connection
					// because that could cause too many file descriptors to be
					// open if the bootstrap phase takes too long.
					if conn != nil {
						if err := conn.Close(); err != nil {
							log.Printf("WARNING: failed to close connection to %s", e)
						}
					}
				case <-c.Done():
					// Transition to kubernetes based health discovery.
					if deleted := r.DeleteBackend(uid); !deleted {
						log.Printf("failed to delete bootstrap backend %q", uid)
					}
					log.Printf("deregistered bootstrap backend with IP: %s", e)
					return
				}
			}
		}(ctx, idx, endpoint)
	}
}

// Backends returns back a copy of the current backends known to proxyd
func (r *ReverseProxy) Backends() map[string]*backend.Backend {
	r.mux.Lock()
	defer r.mux.Unlock()
	backends := make(map[string]*backend.Backend)
	for uid, addr := range r.backends {
		backends[uid] = addr
	}
	return backends
}

// Shutdown initiates a shutdown for the reverse proxy
// TODO fill this out
func (r *ReverseProxy) Shutdown() {
	log.Println("shutdown")
}
