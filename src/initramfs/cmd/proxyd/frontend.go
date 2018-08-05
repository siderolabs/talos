package main

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
)

// ReverseProxy represents a reverse proxy server.
type ReverseProxy struct {
	ConnectTimeout int
	Config         *tls.Config
	current        *Backend
	backends       map[string]*Backend
	mux            *sync.Mutex
}

// Backend represents a backend.
type Backend struct {
	addr        string
	connections uint32
}

// NewReverseProxy initializes a ReverseProxy.
func NewReverseProxy() (r *ReverseProxy, err error) {
	config, err := tlsConfig()
	if err != nil {
		return
	}

	r = &ReverseProxy{
		ConnectTimeout: 50,
		Config:         config,
		mux:            &sync.Mutex{},
		backends:       map[string]*Backend{},
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
func (r *ReverseProxy) AddBackend(addr string) {
	r.mux.Lock()
	r.backends[addr] = &Backend{addr: addr}
	r.setCurrent()
	r.mux.Unlock()
}

// DeleteBackend deletes a backend.
func (r *ReverseProxy) DeleteBackend(addr string) {
	r.mux.Lock()
	if _, ok := r.backends[addr]; ok {
		delete(r.backends, addr)
	}
	r.setCurrent()
	r.mux.Unlock()
}

// GetBackend gets a backend based on least connections algorithm.
func (r *ReverseProxy) GetBackend() (addr string) {
	if r.current != nil {
		return r.current.addr
	}

	return ""
}

// IncrementBackend increments the connections count of a backend.
// nolint: dupl
func (r *ReverseProxy) IncrementBackend(addr string) {
	r.mux.Lock()
	if _, ok := r.backends[addr]; ok {
		r.backends[addr].connections++
	}
	r.setCurrent()
	r.mux.Unlock()
}

// DecrementBackend deccrements the connections count of a backend.
// nolint: dupl
func (r *ReverseProxy) DecrementBackend(addr string) {
	r.mux.Lock()
	if _, ok := r.backends[addr]; ok {
		r.backends[addr].connections--
	}
	r.setCurrent()
	r.mux.Unlock()
}

func (r *ReverseProxy) setCurrent() {
	least := uint32(math.MaxUint32)
	for _, b := range r.backends {
		switch {
		case b.connections == 0:
			fallthrough
		case b.connections < least:
			r.current = b
		}
	}
}

func (r *ReverseProxy) proxyConnection(c net.Conn) {
	addr := r.GetBackend()
	if addr == "" {
		log.Printf("no available backend, closing remote connection: %s", c.RemoteAddr().String())
		// nolint: errcheck
		c.Close()
		return
	}

	upConn, err := net.DialTimeout("tcp", addr+":6443", time.Duration(r.ConnectTimeout)*time.Millisecond)
	if err != nil {
		log.Printf("dial %v: %v", addr, err)
		// nolint: errcheck
		c.Close()
	}
	r.IncrementBackend(addr)
	defer r.DecrementBackend(addr)

	r.joinConnections(c, upConn)
}

func (r *ReverseProxy) joinConnections(c1 net.Conn, c2 net.Conn) {
	// nolint: errcheck
	defer c1.Close()
	// nolint: errcheck
	defer c2.Close()

	var wg sync.WaitGroup
	join := func(dst net.Conn, src net.Conn) {
		defer wg.Done()
		n, err := io.Copy(dst, src)
		if err != nil {
			log.Printf("copy from %v to %v failed after %d bytes with error %v", src.RemoteAddr(), dst.RemoteAddr(), n, err)
		}
	}

	wg.Add(2)
	go join(c1, c2)
	go join(c2, c1)
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
