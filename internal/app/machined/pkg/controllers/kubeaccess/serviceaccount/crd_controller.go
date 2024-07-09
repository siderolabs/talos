// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package serviceaccount

import (
	"bytes"
	"context"
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	taloskubernetes "github.com/siderolabs/go-kubernetes/kubernetes"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/dynamiclister"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/connrotation"
	"k8s.io/client-go/util/workqueue"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

const (
	certTTL            = time.Hour * 6
	certRenewThreshold = time.Hour * 1

	successResourceSynced = "Synced"
	messageResourceSynced = "Synced successfully"

	errResourceExists     = "ErrResourceExists"
	messageResourceExists = "%s already exists and is not managed by controller: %s"

	errRolesNotFound     = "ErrRolesNotFound"
	messageRolesNotFound = "Roles not found"

	errNamespaceNotAllowed     = "ErrNamespaceNotAllowed"
	messageNamespaceNotAllowed = "Namespace is not allowed: %s"

	errRolesNotAllowed     = "ErrRolesNotAllowed"
	messageRolesNotAllowed = "Roles not allowed: %v"

	controllerAgentName  = "talos-sa-controller"
	informerResyncPeriod = time.Minute * 1

	talosconfigContextName = "default"
	endpoint               = constants.KubernetesTalosAPIServiceName + "." + constants.KubernetesTalosAPIServiceNamespace

	kindSecret = "Secret"
)

var (
	talosSAGV = schema.GroupVersion{
		Group:   constants.ServiceAccountResourceGroup,
		Version: constants.ServiceAccountResourceVersion,
	}

	talosSAGVR = talosSAGV.WithResource(constants.ServiceAccountResourcePlural)
	talosSAGVK = talosSAGV.WithKind(constants.ServiceAccountResourceKind)
)

// CRDController is the controller implementation for TalosServiceAccount resources.
type CRDController struct {
	talosCA *x509.PEMEncodedCertificateAndKey

	allowedNamespaces []string
	allowedRoles      map[string]struct{}

	queue workqueue.TypedRateLimitingInterface[string]

	kubeInformerFactory    kubeinformers.SharedInformerFactory
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory

	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	dialer        *connrotation.Dialer

	secretsSynced  cache.InformerSynced
	talosSAsSynced cache.InformerSynced

	secretsLister corelisters.SecretLister
	dynamicLister dynamiclister.Lister

	eventRecorder record.EventRecorder

	logger *zap.Logger
}

// NewCRDController creates a new CRD controller.
func NewCRDController(
	talosCA *x509.PEMEncodedCertificateAndKey,
	kubeconfig *rest.Config,
	allowedNamespaces []string,
	allowedRoles []string,
	logger *zap.Logger,
) (*CRDController, error) {
	dialer := taloskubernetes.NewDialer()
	kubeconfig.Dial = dialer.DialContext

	kubeCli, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	dynCli, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynCli, informerResyncPeriod)
	resourceInformer := dynamicInformerFactory.ForResource(talosSAGVR)
	informer := resourceInformer.Informer()

	indexer := informer.GetIndexer()
	lister := dynamiclister.New(indexer, talosSAGVR)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeCli, informerResyncPeriod)
	secrets := kubeInformerFactory.Core().V1().Secrets()

	logger.Debug("creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeCli.CoreV1().Events("")})

	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := CRDController{
		talosCA:                talosCA,
		allowedNamespaces:      allowedNamespaces,
		allowedRoles:           xslices.ToSet(allowedRoles),
		dynamicInformerFactory: dynamicInformerFactory,
		kubeInformerFactory:    kubeInformerFactory,
		kubeClient:             kubeCli,
		dynamicClient:          dynCli,
		dialer:                 dialer,
		dynamicLister:          lister,
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[string](),
		),
		logger:         logger,
		secretsSynced:  secrets.Informer().HasSynced,
		talosSAsSynced: informer.HasSynced,
		eventRecorder:  recorder,
		secretsLister:  secrets.Lister(),
	}

	if _, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueTalosSA,
		UpdateFunc: func(oldTalosSA, newTalosSA any) {
			controller.enqueueTalosSA(newTalosSA)
		},
	}); err != nil {
		return nil, err
	}

	if _, err = secrets.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleSecret,
		UpdateFunc: func(oldSec, newSec any) {
			newSecret := newSec.(*corev1.Secret) //nolint:errcheck
			oldSecret := oldSec.(*corev1.Secret) //nolint:errcheck

			if newSecret.ResourceVersion == oldSecret.ResourceVersion {
				return
			}

			controller.handleSecret(newSec)
		},
		DeleteFunc: controller.handleSecret,
	}); err != nil {
		return nil, err
	}

	return &controller, nil
}

// Run starts the CRD controller.
func (t *CRDController) Run(ctx context.Context, workers int) error {
	var wg sync.WaitGroup

	defer func() {
		t.queue.ShutDown()
		t.dialer.CloseAll()

		wg.Wait()
		t.logger.Debug("all workers have shut down")
	}()

	t.kubeInformerFactory.Start(ctx.Done())
	t.dynamicInformerFactory.Start(ctx.Done())

	t.logger.Sugar().Debugf("starting %s controller", constants.ServiceAccountResourceKind)

	t.logger.Debug("waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), t.secretsSynced, t.talosSAsSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	t.logger.Debug("starting workers")

	wg.Add(workers)

	for range workers {
		go func() {
			wait.Until(func() { t.runWorker(ctx) }, time.Second, ctx.Done())
			wg.Done()
		}()
	}

	t.logger.Debug("started workers")

	<-ctx.Done()

	t.logger.Debug("shutting down workers")

	t.kubeInformerFactory.Shutdown()

	return nil
}

func (t *CRDController) runWorker(ctx context.Context) {
	for t.processNextWorkItem(ctx) {
	}
}

func (t *CRDController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := t.queue.Get()

	if shutdown {
		return false
	}

	err := func(obj string) error {
		defer t.queue.Done(obj)

		if err := t.syncHandler(ctx, obj); err != nil {
			t.queue.AddRateLimited(obj)

			return fmt.Errorf("error syncing '%s': %s, requeuing", obj, err.Error())
		}

		t.queue.Forget(obj)
		t.logger.Sugar().Debugf("successfully synced '%s'", obj)

		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)

		return true
	}

	return true
}

//nolint:gocyclo,cyclop
func (t *CRDController) syncHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))

		return nil //nolint:nilerr
	}

	talosSA, err := t.dynamicLister.Namespace(namespace).Get(name)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("talosSA '%s' in work queue no longer exists", key))

			return nil
		}

		return err
	}

	secret, err := t.secretsLister.Secrets(namespace).Get(name)
	secretNotFound := kubeerrors.IsNotFound(err)

	if err != nil && !secretNotFound {
		return err
	}

	if !secretNotFound && !metav1.IsControlledBy(secret, talosSA) {
		msg := fmt.Sprintf(messageResourceExists, kindSecret, key)

		err = t.updateTalosSAStatus(ctx, talosSA, msg)
		if err != nil {
			return err
		}

		t.eventRecorder.Event(talosSA, corev1.EventTypeWarning, errResourceExists, msg)

		return errors.New(msg)
	}

	desiredRoles, found, err := unstructured.NestedStringSlice(talosSA.UnstructuredContent(), "spec", "roles")
	if err != nil || !found {
		msg := messageRolesNotFound

		updateErr := t.updateTalosSAStatus(ctx, talosSA, msg)
		if updateErr != nil {
			return updateErr
		}

		t.eventRecorder.Event(talosSA, corev1.EventTypeWarning, errRolesNotFound, messageRolesNotFound)

		if err != nil {
			return fmt.Errorf("%s: %w", msg, err)
		}

		return errors.New(msg)
	}

	desiredRoleSet, _ := role.Parse(desiredRoles)

	if !slices.ContainsFunc(t.allowedNamespaces, func(allowedNS string) bool {
		return allowedNS == namespace
	}) {
		msg := fmt.Sprintf(messageNamespaceNotAllowed, namespace)

		err = t.updateTalosSAStatus(ctx, talosSA, msg)
		if err != nil {
			return err
		}

		t.eventRecorder.Event(talosSA, corev1.EventTypeWarning, errNamespaceNotAllowed, msg)

		return nil
	}

	var unallowedRoles []string

	for _, desiredRole := range desiredRoles {
		_, allowed := t.allowedRoles[desiredRole]
		if !allowed {
			unallowedRoles = append(unallowedRoles, desiredRole)
		}
	}

	if len(unallowedRoles) > 0 {
		msg := fmt.Sprintf(messageRolesNotAllowed, unallowedRoles)

		err = t.updateTalosSAStatus(ctx, talosSA, msg)
		if err != nil {
			return err
		}

		t.eventRecorder.Event(talosSA, corev1.EventTypeWarning, errRolesNotAllowed, msg)

		return nil
	}

	if secretNotFound {
		var newSecret *corev1.Secret

		newSecret, err = t.newSecret(talosSA, desiredRoleSet)
		if err != nil {
			return err
		}

		_, err = t.kubeClient.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if t.needsUpdate(secret, desiredRoleSet.Strings()) {
		var newTalosconfigBytes []byte

		newTalosconfigBytes, err = t.generateTalosconfig(desiredRoleSet)
		if err != nil {
			return err
		}

		secret.Data[constants.TalosconfigFilename] = newTalosconfigBytes

		_, err = t.kubeClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	err = t.updateTalosSAStatus(ctx, talosSA, "")
	if err != nil {
		return err
	}

	t.eventRecorder.Event(talosSA, corev1.EventTypeNormal, successResourceSynced, messageResourceSynced)

	return nil
}

func (t *CRDController) enqueueTalosSA(obj any) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)

		return
	}

	t.queue.Add(key)
}

func (t *CRDController) handleSecret(obj any) {
	var object metav1.Object

	var ok bool

	if object, ok = obj.(metav1.Object); !ok {
		tombstone, tombstoneOK := obj.(cache.DeletedFinalStateUnknown)
		if !tombstoneOK {
			utilruntime.HandleError(errors.New("error decoding object, invalid type"))

			return
		}

		object, tombstoneOK = tombstone.Obj.(metav1.Object)
		if !tombstoneOK {
			utilruntime.HandleError(errors.New("error decoding object tombstone, invalid type"))

			return
		}

		t.logger.Sugar().Debugf("recovered deleted object '%s' from tombstone", object.GetName())
	}

	t.logger.Sugar().Debugf("processing object: %s", object.GetName())

	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.Kind != constants.ServiceAccountResourceKind {
			return
		}

		talosSA, err := t.dynamicLister.Namespace(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			t.logger.Sugar().Debugf("ignoring orphaned object '%s/%s' of %s '%s'",
				object.GetNamespace(), object.GetName(), constants.ServiceAccountResourceKind, ownerRef.Name)

			return
		}

		t.enqueueTalosSA(talosSA)

		return
	}
}

func (t *CRDController) updateTalosSAStatus(
	ctx context.Context,
	talosSA *unstructured.Unstructured,
	failureReason string,
) error {
	var err error

	talosSACopy := talosSA.DeepCopy()

	if failureReason == "" {
		unstructured.RemoveNestedField(talosSACopy.UnstructuredContent(), "status", "failureReason")
	} else {
		err = unstructured.SetNestedField(talosSACopy.UnstructuredContent(), failureReason, "status", "failureReason")
		if err != nil {
			return err
		}
	}

	_, err = t.dynamicClient.Resource(talosSAGVR).
		Namespace(talosSACopy.GetNamespace()).
		Update(ctx, talosSACopy, metav1.UpdateOptions{})

	return err
}

//nolint:gocyclo
func (t *CRDController) needsUpdate(secret *corev1.Secret, desiredRoles []string) bool {
	talosconfigInSecret, ok := secret.Data[constants.TalosconfigFilename]
	if !ok {
		t.logger.Debug("talosconfig not found in secret", zap.String("key", constants.TalosconfigFilename))

		return true
	}

	parsedTalosconfigInSecret, err := clientconfig.ReadFrom(bytes.NewReader(talosconfigInSecret))
	if err != nil {
		t.logger.Debug("error parsing talosconfig in secret", zap.Error(err))

		return true
	}

	talosconfigCtx := parsedTalosconfigInSecret.Contexts[parsedTalosconfigInSecret.Context]

	talosconfigCA, err := base64.StdEncoding.DecodeString(talosconfigCtx.CA)
	if err != nil {
		t.logger.Debug("error decoding talosconfig CA", zap.Error(err))

		return true
	}

	if !bytes.Equal(t.talosCA.Crt, talosconfigCA) {
		t.logger.Debug("ca mismatch detected")

		return true
	}

	if len(talosconfigCtx.Endpoints) != 1 || talosconfigCtx.Endpoints[0] != endpoint {
		t.logger.Debug(
			"endpoint mismatch detected",
			zap.Strings("actual", talosconfigCtx.Endpoints),
			zap.Strings("expected", []string{endpoint}),
		)

		return true
	}

	talosconfigCRT, err := base64.StdEncoding.DecodeString(talosconfigCtx.Crt)
	if err != nil {
		t.logger.Debug("error decoding talosconfig CRT", zap.Error(err))

		return true
	}

	block, _ := pem.Decode(talosconfigCRT)
	if block == nil {
		t.logger.Debug("could not decode talosconfig CRT")

		return true
	}

	certificate, err := stdlibx509.ParseCertificate(block.Bytes)
	if err != nil {
		t.logger.Debug("error parsing certificate in talosconfig of secret", zap.Error(err))

		return true
	}

	if certificate.NotAfter.IsZero() {
		t.logger.Debug("certificate in talosconfig of secret has no expiration date", zap.Error(err))

		return true
	}

	if time.Now().Add(certTTL).Before(certificate.NotAfter) {
		t.logger.Debug(
			"certificate in talosconfig has expiration date too far in the future",
			zap.Time("expiration", certificate.NotAfter),
		)

		return true
	}

	if time.Now().Add(certRenewThreshold).After(certificate.NotAfter) {
		t.logger.Debug(
			"certificate in talosconfig needs renewal",
			zap.Time("expiration", certificate.NotAfter),
		)

		return true
	}

	actualRoles := certificate.Subject.Organization

	sort.Strings(actualRoles)
	sort.Strings(desiredRoles)

	if !slices.Equal(actualRoles, desiredRoles) {
		t.logger.Debug("roles in certificate do not match desired roles",
			zap.Strings("actual", actualRoles), zap.Strings("desired", desiredRoles))

		return true
	}

	return false
}

func (t *CRDController) newSecret(talosSA *unstructured.Unstructured, roles role.Set) (*corev1.Secret, error) {
	config, err := t.generateTalosconfig(roles)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: talosSA.GetName(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(talosSA, talosSAGVK),
			},
		},
		Data: map[string][]byte{
			constants.TalosconfigFilename: config,
		},
	}, nil
}

func (t *CRDController) generateTalosconfig(roles role.Set) ([]byte, error) {
	var newCert *x509.PEMEncodedCertificateAndKey

	newCert, err := secrets.NewAdminCertificateAndKey(time.Now(), t.talosCA, roles, certTTL)
	if err != nil {
		return nil, err
	}

	newTalosconfig := clientconfig.NewConfig(talosconfigContextName, []string{endpoint}, t.talosCA.Crt, newCert)

	return newTalosconfig.Bytes()
}
