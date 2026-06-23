// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nodedrain_test

import (
	"context"
	"testing"
	"time"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/reporter"
)

func volumeAttachment(name, nodeName string) *storagev1.VolumeAttachment {
	pvName := "pv-" + name

	return &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: nodeName,
			Attacher: "csi.example.com",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
		},
	}
}

func TestWaitForVolumeDetach(t *testing.T) {
	const nodeName = "node-1"

	for _, test := range []struct {
		name    string
		objects []*storagev1.VolumeAttachment
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "no attachments",
			timeout: time.Second,
		},
		{
			name:    "attachment on another node is ignored",
			objects: []*storagev1.VolumeAttachment{volumeAttachment("va-other", "node-2")},
			timeout: time.Second,
		},
		{
			name:    "attachment on the node times out",
			objects: []*storagev1.VolumeAttachment{volumeAttachment("va-1", nodeName)},
			timeout: 100 * time.Millisecond,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			objects := make([]runtime.Object, 0, len(test.objects))
			for _, va := range test.objects {
				objects = append(objects, va)
			}

			clientset := fake.NewClientset(objects...)

			err := nodedrain.WaitForVolumeDetach(context.Background(), clientset, nodeName, test.timeout, func(reporter.Update) {})
			if (err != nil) != test.wantErr {
				t.Fatalf("WaitForVolumeDetach() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
