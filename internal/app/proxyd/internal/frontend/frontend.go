/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package frontend

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/backend"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ReverseProxy represents a reverse proxy server.
type ReverseProxy struct {
	ConnectTimeout int
	Config         *tls.Config
	Context        context.Context
	current        *backend.Backend
	backends       map[string]*backend.Backend
	mux            *sync.Mutex
	ctxCancel      context.CancelFunc
}

// Exposed for testing purposes
var caCertFile = "/etc/kubernetes/pki/ca.crt"

// NewReverseProxy initializes a ReverseProxy.
// nolint: interfacer
func NewReverseProxy(ip net.IP) (r *ReverseProxy, err error) {
	config, err := tlsConfig()
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	r = &ReverseProxy{
		ConnectTimeout: 100,
		Config:         config,
		mux:            &sync.Mutex{},
		backends: map[string]*backend.Backend{
			"bootstrap": {
				UID:         "bootstrap",
				Addr:        ip.String(),
				Connections: 0,
			},
		},
		Context:   ctx,
		ctxCancel: cancel,
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
func (r *ReverseProxy) AddBackend(uid, addr string) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.backends[uid] = &backend.Backend{UID: uid, Addr: addr, Connections: 0}
	r.setCurrent()
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
func (r *ReverseProxy) Watch(kubeClient kubernetes.Interface) {
	informers := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := informers.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc:    r.AddFunc(),
		DeleteFunc: r.DeleteFunc(),
		UpdateFunc: r.UpdateFunc(),
	})

	// Make sure informers are running.
	informers.Start(r.Context.Done())

	// This is not required in tests, but it serves as a proof-of-concept by
	// ensuring that the informer goroutine have warmed up and called List before
	// we send any events to it.
	cache.WaitForCacheSync(r.Context.Done(), podInformer.HasSynced)

	<-r.Context.Done()
}

// Shutdown initiates a shutdown for the reverse proxy
func (r *ReverseProxy) Shutdown() {
	r.ctxCancel()
}

// AddFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) AddFunc() func(obj interface{}) {
	return func(obj interface{}) {
		// nolint: errcheck
		pod := obj.(*v1.Pod)

		if !isAPIServer(pod) {
			return
		}

		// We need an IP address to register.
		if pod.Status.PodIP == "" {
			return
		}

		// Return early in the case of any container not being ready.
		for _, status := range pod.Status.ContainerStatuses {
			if !status.Ready {
				return
			}
		}

		r.AddBackend(string(pod.UID), pod.Status.PodIP)
		log.Printf("registered API server %s (UID: %q) with IP: %s", pod.Name, pod.UID, pod.Status.PodIP)
		if deleted := r.DeleteBackend("bootstrap"); deleted {
			log.Println("deregistered bootstrap backend")
		}
	}
}

// UpdateFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) UpdateFunc() func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		r.AddFunc()(newObj)
		r.DeleteFunc()(oldObj)
	}
}

// DeleteFunc is a ResourceEventHandlerFunc.
func (r *ReverseProxy) DeleteFunc() func(obj interface{}) {
	return func(obj interface{}) {
		// nolint: errcheck
		pod := obj.(*v1.Pod)

		if !isAPIServer(pod) {
			return
		}

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

	uid := backend.UID
	addr := backend.Addr

	c2, err := net.DialTimeout("tcp", addr+":6443", time.Duration(r.ConnectTimeout)*time.Millisecond)
	if err != nil {
		log.Printf("dial %v: %v", addr, err)
		// nolint: errcheck
		c1.Close()
		return
	}

	// Ensure the connections are valid.
	if c1 == nil || c2 == nil {
		return
	}

	r.IncrementBackend(uid)
	r.joinConnections(c1, c2)
	r.DecrementBackend(uid)
}

func (r *ReverseProxy) joinConnections(c1 net.Conn, c2 net.Conn) {
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

	// Wait for connection to terminate
	wg.Wait()
}

func tlsConfig() (config *tls.Config, err error) {
	if caCertFile == "" {
		return &tls.Config{}, err
	}

	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return config, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	config = &tls.Config{RootCAs: certPool}

	return config, err
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

// Backends returns back a copy of the current backends known to proxyd
func (r *ReverseProxy) Backends() map[string]*backend.Backend {
	r.mux.Lock()
	defer r.mux.Unlock()
	backends := make(map[string]*backend.Backend)
	for name, be := range r.backends {
		backends[name] = be
	}
	return backends
}
