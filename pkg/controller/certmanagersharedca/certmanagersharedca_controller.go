//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package certmanagersharedca

import (
	"context"

	operatorv1alpha1 "github.com/ibm/ibm-cert-manager-operator/pkg/apis/operator/v1alpha1"

	certmgr "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_certmanagersharedca")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new CertManagerSharedCA Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCertManagerSharedCA{client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetEventRecorderFor("ibm-cert-manager-operator")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("certmanagersharedca-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CertManagerSharedCA
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.CertManagerSharedCA{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner CertManagerSharedCA
	err = c.Watch(&source.Kind{Type: &certmgr.Certificate{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.CertManagerSharedCA{},
	})
	if err != nil {
		return err
	}
	// Watch for changes to secondary resource Pods and requeue the owner CertManagerSharedCA
	err = c.Watch(&source.Kind{Type: &certmgr.Issuer{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.CertManagerSharedCA{},
	})
	if err != nil {
		return err
	}
	// Watch for changes to secondary resource Pods and requeue the owner CertManagerSharedCA
	err = c.Watch(&source.Kind{Type: &certmgr.ClusterIssuer{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.CertManagerSharedCA{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCertManagerSharedCA implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCertManagerSharedCA{}

// ReconcileCertManagerSharedCA reconciles a CertManagerSharedCA object
type ReconcileCertManagerSharedCA struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a CertManagerSharedCA object and makes changes based on the state read
// and what is in the CertManagerSharedCA.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCertManagerSharedCA) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling CertManagerSharedCA")

	// Fetch the CertManagerSharedCA instance
	instance := &operatorv1alpha1.CertManagerSharedCA{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	var ns = instance.Spec.Namespace
	var ssIssuerName = "cs-ss-issuer"
	ssIssuer := &certmgr.Issuer{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ssIssuerName, Namespace: ns}, ssIssuer)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new self signed Issuer", "Name", ssIssuerName, "Namespace", ns)
		ssIssuer := newIssuer(ssIssuerName, ns)
		// Set CertManagerSharedCA instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, ssIssuer, r.scheme); err != nil {
			r.updateStatus(instance, "Can't set controller reference on self signed issuer", corev1.EventTypeWarning, "Error")
			return reconcile.Result{}, err
		}
		err = r.client.Create(context.TODO(), ssIssuer)
		if err != nil {
			log.Error(err, "Error creating self signed issuer, requeueing")
			r.updateStatus(instance, "Error creating self signed Issuer", corev1.EventTypeWarning, "Error")
			return reconcile.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Error accessing self signed issuer, requeueing")
		return reconcile.Result{}, err
	}
	var caCertName = "cs-ca-certificate"
	var caSecretName = "cs-ca-certificate-secret"

	caCert := &certmgr.Certificate{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: caCertName, Namespace: ns}, caCert)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new CA Certificate", "Name", caCertName, "Namespace", ns)
		caCertificate := newCert(caCertName, ns, caSecretName, ssIssuerName, "Issuer")
		// Set CertManagerSharedCA instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, caCertificate, r.scheme); err != nil {
			r.updateStatus(instance, "Can't set controller reference on certificate", corev1.EventTypeWarning, "Error")
			return reconcile.Result{}, err
		}
		err = r.client.Create(context.TODO(), caCertificate)
		if err != nil {
			log.Error(err, "Error creating CA Certificate, requeueing")
			r.updateStatus(instance, "Error creating CA Certificate", corev1.EventTypeWarning, "Error")
			return reconcile.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Error accessing CA Certificate, requeueing")
		return reconcile.Result{}, err
	}

	var caClusterIssuerName = "cs-ca-issuer"
	if instance.Spec.CAName != "" {
		caClusterIssuerName = instance.Spec.CAName
	}
	// Check if this ClusterIssuer already exists
	found := &certmgr.ClusterIssuer{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: caClusterIssuerName, Namespace: ""}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new ClusterIssuer", "Pod.Namespace", caClusterIssuerName)
		clusterIssuer := newClusterIssuer(caClusterIssuerName, caSecretName)
		// Set CertManagerSharedCA instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, clusterIssuer, r.scheme); err != nil {
			r.updateStatus(instance, "Can't set controller reference on ca clusterissuer", corev1.EventTypeWarning, "Error")
			return reconcile.Result{}, err
		}
		err = r.client.Create(context.TODO(), clusterIssuer)
		if err != nil {
			log.Error(err, "Error creating CA ClusterIssuer, requeueing")
			r.updateStatus(instance, "Error creating CA ClusterIssuer", corev1.EventTypeWarning, "Error")
			return reconcile.Result{}, err
		}

	} else if err != nil {
		log.Error(err, "Error accessing CA ClusterIssuer, requeueing")
		return reconcile.Result{}, err
	}

	r.updateStatus(instance, "Successfully created self signed issuer, CA certificate, and CA clusterissuer", corev1.EventTypeNormal, "Success")
	return reconcile.Result{}, nil
}

func newIssuer(name, ns string) *certmgr.Issuer {
	return &certmgr.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certmgr.IssuerSpec{
			IssuerConfig: certmgr.IssuerConfig{
				SelfSigned: &certmgr.SelfSignedIssuer{},
			},
		},
	}
}

func newCert(name, ns, secret, issuerName, issuerKind string) *certmgr.Certificate {
	return &certmgr.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: certmgr.CertificateSpec{
			CommonName: "ibm-cs-ca",
			IsCA:       true,
			SecretName: secret,
			IssuerRef: certmgr.ObjectReference{
				Name: issuerName,
				Kind: issuerKind,
			},
		},
	}
}

func newClusterIssuer(name, secret string) *certmgr.ClusterIssuer {
	return &certmgr.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: certmgr.IssuerSpec{
			IssuerConfig: certmgr.IssuerConfig{
				CA: &certmgr.CAIssuer{
					SecretName: secret,
				},
			},
		},
	}
}

func (r *ReconcileCertManagerSharedCA) updateStatus(instance *operatorv1alpha1.CertManagerSharedCA, message, event, reason string) {
	r.recorder.Event(instance, event, reason, message)
}
