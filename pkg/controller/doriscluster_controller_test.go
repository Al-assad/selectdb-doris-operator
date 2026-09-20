// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package controller

import (
	"context"
	"testing"

	dorisv1 "github.com/selectdb/doris-operator/api/doris/v1"
	"github.com/selectdb/doris-operator/pkg/controller/sub_controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type recordingSubController struct {
	syncCalls   int
	clearCalls  int
	statusCalls int
}

func (r *recordingSubController) Sync(context.Context, *dorisv1.DorisCluster) error {
	r.syncCalls++
	return nil
}

func (r *recordingSubController) ClearResources(context.Context, *dorisv1.DorisCluster) (bool, error) {
	r.clearCalls++
	return true, nil
}

func (r *recordingSubController) GetControllerName() string {
	return "recording"
}

func (r *recordingSubController) UpdateComponentStatus(*dorisv1.DorisCluster) error {
	r.statusCalls++
	return nil
}

func TestReconcilePausedDorisCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := dorisv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	dcr := &dorisv1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "paused",
			Namespace: "default",
			Annotations: map[string]string{
				dorisv1.AnnotationReconcilePaused: "true",
			},
		},
	}
	recorder := &recordingSubController{}
	reconciler := &DorisClusterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(dcr).Build(),
		Scs: map[string]sub_controller.SubController{
			"recording": recorder,
		},
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: dcr.Name, Namespace: dcr.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("Reconcile() result = %+v, want no requeue", result)
	}
	if recorder.syncCalls != 0 || recorder.clearCalls != 0 || recorder.statusCalls != 0 {
		t.Fatalf("paused cluster called subcontrollers: sync=%d clear=%d status=%d",
			recorder.syncCalls, recorder.clearCalls, recorder.statusCalls)
	}
}

func TestPausedDorisClusterDeletionStillCleansResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := dorisv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	now := metav1.Now()
	dcr := &dorisv1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-paused",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"test-finalizer"},
			Annotations: map[string]string{
				dorisv1.AnnotationReconcilePaused: "true",
			},
		},
	}
	recorder := &recordingSubController{}
	reconciler := &DorisClusterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(dcr).Build(),
		Scs: map[string]sub_controller.SubController{
			"recording": recorder,
		},
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: dcr.Name, Namespace: dcr.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if recorder.clearCalls != 1 {
		t.Fatalf("ClearResources() calls = %d, want 1", recorder.clearCalls)
	}
	if recorder.syncCalls != 0 || recorder.statusCalls != 0 {
		t.Fatalf("deleting cluster called unexpected subcontrollers: sync=%d status=%d",
			recorder.syncCalls, recorder.statusCalls)
	}
}

func TestUnpausedDorisClusterContinuesReconciliation(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := dorisv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	dcr := &dorisv1.DorisCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-cluster",
			Namespace: "business",
		},
	}
	recorder := &recordingSubController{}
	reconciler := &DorisClusterReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(dcr).Build(),
		Scs: map[string]sub_controller.SubController{
			"recording": recorder,
		},
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: dcr.Name, Namespace: dcr.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if recorder.syncCalls != 1 || recorder.clearCalls != 1 || recorder.statusCalls != 1 {
		t.Fatalf("unpaused cluster calls: sync=%d clear=%d status=%d, want all 1",
			recorder.syncCalls, recorder.clearCalls, recorder.statusCalls)
	}
}
