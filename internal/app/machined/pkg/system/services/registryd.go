// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/namespaces"
	criconstants "github.com/containerd/containerd/pkg/cri/constants"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/hashicorp/go-cleanhttp"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/slices"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

type registrydService struct {
	logger     *zap.Logger
	resources  state.State
	client     *containerdapi.Client
	httpClient *http.Client
}

// Main is an entrypoint to the API service.
func (s *registrydService) Main(ctx context.Context, r runtime.Runtime, logWriter io.Writer) error {
	s.logger = logging.ZapLogger(
		logging.NewLogDestination(logWriter, zapcore.DebugLevel, logging.WithColoredLevels()),
	)
	s.resources = r.State().V1Alpha2().Resources()
	s.httpClient = cleanhttp.DefaultPooledClient()

	var err error

	s.client, err = containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer s.client.Close()

	server := http.Server{
		Addr:    ":3172",
		Handler: s,
	}

	go func() {
		server.ListenAndServe() //nolint:errcheck
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCtxCancel()

	return server.Shutdown(shutdownCtx)
}

func (s *registrydService) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	isProxied := req.Header.Get("X-Talos-Registry-Proxy") == "true"

	logger := s.logger.With(
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.Bool("proxied", isProxied),
		zap.String("remote_addr", req.RemoteAddr),
	)

	logger.Info("got request")

	switch req.Method {
	case http.MethodGet, http.MethodHead:
		// accepted
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	requestPath := path.Clean(req.URL.Path)

	// quickly respond to health check
	switch requestPath {
	case "/v2", "/healthz":
		w.WriteHeader(http.StatusOK)

		return
	}

	registry := req.URL.Query().Get("ns")
	if registry == "" {
		logger.Error("no registry specified")
		w.WriteHeader(http.StatusNotFound)

		return
	}

	parts := strings.Split(requestPath, "/")
	if len(parts) < 5 {
		logger.Error("wrong paths count")
		w.WriteHeader(http.StatusNotFound)

		return
	}

	l := len(parts)

	var (
		name, digest string
		isBlob       bool
	)

	switch {
	case parts[1] == "v2" && parts[l-2] == "manifests":
		name = strings.Join(parts[2:l-2], "/")
		digest = parts[l-1]
	case parts[1] == "v2" && parts[l-2] == "blobs":
		name = strings.Join(parts[2:l-2], "/")
		digest = parts[l-1]

		isBlob = true
	default:
		logger.Error("wrong path")
		w.WriteHeader(http.StatusNotFound)

		return
	}

	if !reference.NameRegexp.MatchString(name) {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	if !reference.DigestRegexp.MatchString(digest) {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	logger.Info("image request", zap.String("name", name), zap.String("digest", digest), zap.Bool("is_blob", isBlob), zap.String("registry", registry))

	ref, err := reference.Parse(fmt.Sprintf("%s/%s@%s", registry, name, digest))
	if err != nil {
		s.logger.Error("failed to parse reference", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	canonical, ok := ref.(reference.Canonical)
	if !ok {
		logger.Error("not a canonical reference")
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	logger.Info("canonical reference", zap.String("canonical", canonical.String()))

	if isProxied {
		s.handleLocal(logger.With(zap.String("handler", "local")), w, req, canonical, isBlob)
	} else {
		s.handleFanout(logger.With(zap.String("handler", "fanout")), w, req, canonical, isBlob)
	}
}

func (s *registrydService) handleLocal(logger *zap.Logger, w http.ResponseWriter, req *http.Request, canonical reference.Canonical, isBlob bool) {
	var (
		ctx  context.Context
		info content.Info
		err  error
	)

	for _, namespace := range []string{constants.SystemContainerdNamespace, criconstants.K8sContainerdNamespace} {
		ctx = namespaces.WithNamespace(req.Context(), namespace)

		info, err = s.client.ContentStore().Info(ctx, canonical.Digest())
		if err != nil {
			if errdefs.IsNotFound(err) {
				continue
			}

			logger.Error("failed to get content info", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		break
	}

	if err != nil {
		logger.Error("failed to find content info", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("Docker-Content-Digest", canonical.Digest().String())

	// for manifestBlob requests, we need to set the content type and read the manifestBlob immediately
	var manifestBlob []byte

	if !isBlob {
		manifestBlob, err = content.ReadBlob(ctx, s.client.ContentStore(), ocispec.Descriptor{Digest: canonical.Digest()})
		if err != nil {
			logger.Error("failed to read content blob", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		var manifest struct {
			MediaType string `json:"mediaType"`
		}

		if err = json.Unmarshal(manifestBlob, &manifest); err != nil {
			logger.Error("failed to unmarshal manifest", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		if manifest.MediaType == "" {
			logger.Error("failed to get media type", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", manifest.MediaType)
	}

	logger.Info("response headers set", zap.Stringer("digest", canonical.Digest()), zap.Int64("size", info.Size))

	if req.Method == http.MethodHead {
		// all done
		w.WriteHeader(http.StatusOK)

		return
	}

	if !isBlob {
		w.WriteHeader(http.StatusOK)
		w.Write(manifestBlob) //nolint:errcheck

		logger.Info("manifest sent")

		return
	}

	reader, err := s.client.ContentStore().ReaderAt(ctx, ocispec.Descriptor{Digest: info.Digest})
	if err != nil {
		logger.Error("failed to get content reader", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}
	defer reader.Close() //nolint:errcheck

	io.Copy(w, content.NewReader(reader)) //nolint:errcheck

	logger.Info("stream sent")
}

func (s *registrydService) handleFanout(logger *zap.Logger, w http.ResponseWriter, req *http.Request, canonical reference.Canonical, isBlob bool) {
	ctx := req.Context()

	members, err := safe.StateList[*cluster.Member](ctx, s.resources, resource.NewMetadata(cluster.NamespaceName, cluster.MemberType, "", resource.VersionUndefined))
	if err != nil {
		logger.Error("failed to list members", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	var endpoints []string

	for iter := safe.IteratorFromList(members); iter.Next(); {
		endpoints = append(endpoints, slices.Map(iter.Value().TypedSpec().Addresses, func(addr netip.Addr) string {
			return net.JoinHostPort(addr.String(), "3172")
		})...)
	}

	if len(endpoints) == 0 {
		logger.Error("no endpoints found")
		w.WriteHeader(http.StatusNotFound)

		return
	}

	logger.Info("fan out", zap.Strings("endpoints", endpoints), zap.String("canonical", canonical.String()), zap.Bool("is_blob", isBlob))

	fanoutCtx, fanoutCancel := context.WithTimeout(ctx, 15*time.Second)
	defer fanoutCancel()

	result := make(chan string)

	for _, endpoint := range endpoints {
		go func(endpoint string) {
			channel.SendWithContext(fanoutCtx, result, func() string {
				fanoutURL := url.URL{
					Scheme:   "http",
					Host:     endpoint,
					Path:     req.URL.Path,
					RawQuery: req.URL.RawQuery,
				}

				fanoutReq, err := http.NewRequestWithContext(fanoutCtx, http.MethodHead, fanoutURL.String(), nil)
				if err != nil {
					logger.Error("failed to create fanout request", zap.Error(err), zap.String("endpoint", endpoint))

					return ""
				}

				fanoutReq.Header.Set("X-Talos-Registry-Proxy", "true")

				resp, err := s.httpClient.Do(fanoutReq)
				if err != nil {
					logger.Error("failed to fanout request", zap.Error(err), zap.String("endpoint", endpoint))

					return ""
				}

				if resp.Body != nil {
					resp.Body.Close() //nolint:errcheck
				}

				if resp.StatusCode != http.StatusOK {
					logger.Error("fanout request failed", zap.Int("status", resp.StatusCode), zap.String("endpoint", endpoint))

					return ""
				}

				logger.Info("fanout request succeeded", zap.String("endpoint", endpoint))

				return endpoint
			}())
		}(endpoint)
	}

	var (
		goodEndpoint string
		responses    int
	)

collectLoop:
	for {
		select {
		case <-fanoutCtx.Done():
			logger.Error("fanout timed out")
			w.WriteHeader(http.StatusNotFound)

			return
		case endpoint := <-result:
			logger.Info("fanout response", zap.String("endpoint", endpoint))
			responses++

			if endpoint != "" {
				goodEndpoint = endpoint

				fanoutCancel()

				break collectLoop
			}

			if responses == len(endpoints) {
				logger.Error("no good endpoints found")
				w.WriteHeader(http.StatusNotFound)

				return
			}
		}
	}

	logger.Info("good endpoint", zap.String("endpoint", goodEndpoint))

	if req.Method == http.MethodHead {
		// all done
		w.WriteHeader(http.StatusOK)

		return
	}

	// we have a good endpoint, proxy the request
	req.Header.Set("X-Talos-Registry-Proxy", "true")

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: goodEndpoint})
	proxy.Transport = s.httpClient.Transport

	proxy.ServeHTTP(w, req)
}

var _ system.HealthcheckedService = (*Registryd)(nil)

// Registryd implements the Service interface. It serves as the concrete type with
// the required methods.
type Registryd struct {
	Controller runtime.Controller
}

// ID implements the Service interface.
func (m *Registryd) ID(r runtime.Runtime) string {
	return "registryd"
}

// PreFunc implements the Service interface.
func (m *Registryd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (m *Registryd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (m *Registryd) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (m *Registryd) DependsOn(r runtime.Runtime) []string {
	return []string{"cri"}
}

// Runner implements the Service interface.
func (m *Registryd) Runner(r runtime.Runtime) (runner.Runner, error) {
	svc := &registrydService{}

	return goroutine.NewRunner(r, "registryd", svc.Main, runner.WithLoggingManager(r.Logging())), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (m *Registryd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		// TODO: implement me
		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (m *Registryd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
