/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package frontend

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/proxyd/internal/backend"
	pkgnet "github.com/talos-systems/talos/internal/pkg/net"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// ReverseProxy represents a reverse proxy server.
type ReverseProxy struct {
	ConnectTimeout int
	Config         *tls.Config
	current        *backend.Backend
	backends       map[string]*backend.Backend
	mux            *sync.Mutex
}

// NewReverseProxy initializes a ReverseProxy.
func NewReverseProxy() (r *ReverseProxy, err error) {
	config, err := tlsConfig()
	if err != nil {
		return
	}

	r = &ReverseProxy{
		ConnectTimeout: 100,
		Config:         config,
		mux:            &sync.Mutex{},
		backends:       map[string]*backend.Backend{},
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
func (r *ReverseProxy) Watch() (err error) {
	kubeconfig := "/etc/kubernetes/admin.conf"

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return
	}
	ips, err := pkgnet.IPAddrs()
	if err != nil {
		return
	}
	if len(ips) == 0 {
		return errors.New("no IP address found for bootstrap backend")
	}
	ip := ips[0]
	// Update the host to the node's IP.
	config.Host = ip.String() + ":6443"
	// Add the node for the purposes of bootstrapping. If we don't do this, the
	// kubelet won't be able reach the API server before it becomes healthy.
	r.AddBackend("bootstrap", ip.String())
	log.Printf("registered bootstrap backend with IP: %s", ip.String())

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
		// nolint: errcheck
		old := oldObj.(*v1.Pod)
		// nolint: errcheck
		new := newObj.(*v1.Pod)

		if !isAPIServer(old) {
			return
		}

		// Remove the backend if any container is not ready.
		for _, status := range new.Status.ContainerStatuses {
			if !status.Ready {
				if deleted := r.DeleteBackend(old.Status.PodIP); deleted {
					log.Printf("deregistered unhealthy API server %s (UID: %q) with IP: %s", old.Name, old.UID, old.Status.PodIP)
				}
				break
			}
		}

		// We need an IP address to register.
		if old.Status.PodIP == "" && new.Status.PodIP != "" {
			r.AddBackend(string(new.UID), new.Status.PodIP)
			log.Printf("registered API server %s (UID: %q) with IP: %s", new.Name, new.UID, new.Status.PodIP)
			if deleted := r.DeleteBackend("bootstrap"); deleted {
				log.Println("deregistered bootstrap backend")
			}
		}
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

		if deleted := r.DeleteBackend(pod.Status.PodIP); deleted {
			log.Printf("deregistered API server %s (UID: %q) with IP: %s", pod.Name, pod.UID, pod.Status.PodIP)
		}
	}
}

func (r *ReverseProxy) setCurrent() {
	least := uint32(math.MaxUint32)
	for _, b := range r.backends {
		switch {
		case b.Connections == 0:
			fallthrough
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

	r.joinConnections(uid, c1, c2)
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

func tlsConfig() (config *tls.Config, err error) {
	caCert, err := ioutil.ReadFile("/etc/kubernetes/pki/ca.crt")
	if err != nil {
		return
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	config = &tls.Config{RootCAs: certPool}

	return config, nil
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
