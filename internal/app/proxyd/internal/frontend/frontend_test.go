/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package frontend

import (
	"context"
	"io/ioutil"
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/backend"

	"k8s.io/client-go/kubernetes/fake"
)

type ProxydSuite struct {
	suite.Suite
}

func TestFrontendSuite(t *testing.T) {
	// Hide all our state transition messages
	log.SetOutput(ioutil.Discard)
	suite.Run(t, new(ProxydSuite))
}

func (suite *ProxydSuite) TestWatch() {
	_, cancel := context.WithCancel(context.Background())
	r, err := NewReverseProxy([]string{"127.0.0.1"}, cancel)
	suite.Assert().NoError(err)
	defer r.Shutdown()

	// Generate a simple pod
	p := genPod()

	// Create our fake k8s client
	client := fake.NewSimpleClientset()
	// nolint: errcheck
	go r.Watch(client)

	output := make(chan string)
	go func() {
		var be map[string]*backend.Backend
		for {
			be = r.Backends()
			if _, ok := be[string(p.UID)]; ok {
				output <- be[string(p.UID)].UID
			}
		}
	}()

	// Verify we have a bootstrap backend
	_, err = client.CoreV1().Pods(metav1.NamespaceSystem).Create(p)
	suite.Assert().NoError(err)

	timeout := time.NewTicker(time.Second * 5)
	select {
	case <-timeout.C:
		suite.T().Error("failed to get updated backend")
	case uid := <-output:
		suite.Equal(string(p.UID), uid)
	}
}

func (suite *ProxydSuite) TestAddFunc() {
	_, cancel := context.WithCancel(context.Background())
	r, err := NewReverseProxy([]string{"127.0.0.1"}, cancel)
	suite.Assert().NoError(err)
	defer r.Shutdown()

	for i := 0; i < 5; i++ {
		r.AddFunc()(genPod())
	}

	bes := r.Backends()

	// Ensure bootstrap backend was removed
	_, bootstrap := bes["bootstrap"]
	suite.Equal(bootstrap, false)

	// Verify we have the appropriate number of new backends
	suite.Equal(len(bes), 5)
}

func (suite *ProxydSuite) TestDeleteFunc() {
	_, cancel := context.WithCancel(context.Background())
	r, err := NewReverseProxy([]string{"127.0.0.1"}, cancel)
	suite.Assert().NoError(err)
	defer r.Shutdown()

	// Add some sample backends
	pods := make([]*v1.Pod, 5)
	for i := 0; i < 5; i++ {
		pods[i] = genPod()
	}
	for _, pod := range pods {
		r.AddFunc()(pod)
	}
	// Delete all sample backends
	for _, pod := range pods {
		r.DeleteFunc()(pod)
	}

	bes := r.Backends()
	suite.Equal(len(bes), 0)
}

func (suite *ProxydSuite) TestUpdateFunc() {
	_, cancel := context.WithCancel(context.Background())
	r, err := NewReverseProxy([]string{"127.0.0.1"}, cancel)
	suite.Assert().NoError(err)
	defer r.Shutdown()

	// Add some sample backend
	pod := genPod()
	r.AddFunc()(pod)

	// Generate a new UID for the pod to simulate(?) a
	// pod getting updated
	oldpod := *pod

	r.UpdateFunc()(&oldpod, pod)

	bes := r.Backends()

	suite.Equal(len(bes), 1)
}

func genPod() (p *v1.Pod) {
	id := rand.Intn(255)

	labels := map[string]string{}
	labels["component"] = "kube-apiserver"

	p = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "fakeapi-" + strconv.Itoa(id),
			Labels: labels,
			UID:    types.UID(uuid.New().String()),
		},
		Status: v1.PodStatus{
			PodIP: "127.0.0." + strconv.Itoa(id),
			ContainerStatuses: []v1.ContainerStatus{
				{
					Ready: true,
				},
			},
			Phase: v1.PodRunning,
		},
	}

	// PodPhase= PodRunning
	return p
}
