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

package certmanager

import (
	"context"

	operatorv1alpha1 "github.com/ibm/ibm-cert-manager-operator/pkg/apis/operator/v1alpha1"

	certmgr "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile reads that state of the cluster for a CertManagerSharedCA object and makes changes based on the state read
// and what is in the CertManagerSharedCA.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func deployDefaultCA(instance *operatorv1alpha1.CertManager, client client.Client, scheme *runtime.Scheme, ns string) error {
	log := logf.Log.WithName("shared-ca")
	var ssIssuerName = "cs-ss-issuer"
	ssIssuer := &certmgr.Issuer{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: ssIssuerName, Namespace: ns}, ssIssuer)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new self signed Issuer", "Name", ssIssuerName, "Namespace", ns)
		ssIssuer := newIssuer(ssIssuerName, ns)
		// Set CertManagerSharedCA instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, ssIssuer, scheme); err != nil {
			return err
		}
		err = client.Create(context.TODO(), ssIssuer)
		if err != nil {
			log.Error(err, "Error creating self signed issuer, requeueing")
			return err
		}
	} else if err != nil {
		log.Error(err, "Error accessing self signed issuer, requeueing")
		return err
	}
	var caCertName = "cs-ca-certificate"
	var caSecretName = "cs-ca-certificate-secret"

	caCert := &certmgr.Certificate{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: caCertName, Namespace: ns}, caCert)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new CA Certificate", "Name", caCertName, "Namespace", ns)
		caCertificate := newCert(caCertName, ns, caSecretName, ssIssuerName, "Issuer")
		// Set CertManagerSharedCA instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, caCertificate, scheme); err != nil {
			return err
		}
		err = client.Create(context.TODO(), caCertificate)
		if err != nil {
			log.Error(err, "Error creating CA Certificate, requeueing")
			return err
		}
	} else if err != nil {
		log.Error(err, "Error accessing CA Certificate, requeueing")
		return err
	}

	var caClusterIssuerName = "cs-ca-issuer"

	// Check if this ClusterIssuer already exists
	found := &certmgr.ClusterIssuer{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: caClusterIssuerName, Namespace: ""}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ClusterIssuer", "Pod.Namespace", caClusterIssuerName)
		clusterIssuer := newClusterIssuer(caClusterIssuerName, caSecretName)
		// Set CertManagerSharedCA instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, clusterIssuer, scheme); err != nil {
			return err
		}
		err = client.Create(context.TODO(), clusterIssuer)
		if err != nil {
			log.Error(err, "Error creating CA ClusterIssuer, requeueing")
			return err
		}
	} else if err != nil {
		log.Error(err, "Error accessing CA ClusterIssuer, requeueing")
		return err
	}
	return nil
}

func byoCA(instance *operatorv1alpha1.CertManager, client client.Client, scheme *runtime.Scheme, ns string) error {

	return nil
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
