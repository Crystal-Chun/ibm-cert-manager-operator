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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const caClusterIssuerName = "cs-ca-clusterissuer"
const ssIssuerName = "cs-ss-issuer"
const caCertName = "cs-ca-certificate"
const caSecretName = "cs-ca-certificate-secret"

func deployDefaultCA(instance *operatorv1alpha1.CertManager, client client.Client, scheme *runtime.Scheme, ns string) error {
	log := logf.Log.WithName("shared-ca")

	ssIssuer := &certmgr.Issuer{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: ssIssuerName, Namespace: ns}, ssIssuer)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new self signed Issuer", "Name", ssIssuerName, "Namespace", ns)
		ssIssuer := newIssuer(ssIssuerName, ns)
		// Set CertManager instance as the owner and controller
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

	caCert := &certmgr.Certificate{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: caCertName, Namespace: ns}, caCert)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new CA Certificate", "Name", caCertName, "Namespace", ns)
		caCertificate := newCert(caCertName, ns, caSecretName, ssIssuerName, "Issuer")
		// Set CertManager instance as the owner and controller
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

	// Check if this ClusterIssuer already exists
	clusterIssuerFound := &certmgr.ClusterIssuer{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: caClusterIssuerName, Namespace: ""}, clusterIssuerFound)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ClusterIssuer", "Pod.Namespace", caClusterIssuerName)
		clusterIssuer := newClusterIssuer(caClusterIssuerName, caSecretName)
		// Set CertManager instance as the owner and controller
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
	if err := removeForBYO(client, ns); err != nil {
		return err
	}
	// Check if the ClusterIssuer already exists
	clusterIssuerFound := &certmgr.ClusterIssuer{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: caClusterIssuerName, Namespace: ""}, clusterIssuerFound)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating a new ClusterIssuer", "Pod.Namespace", caClusterIssuerName)
		clusterIssuer := newClusterIssuer(caClusterIssuerName, caSecretName)
		// Set CertManager instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, clusterIssuer, scheme); err != nil {
			return err
		}
		err = client.Create(context.TODO(), clusterIssuer)
		if err != nil {
			log.Error(err, "Error creating CA ClusterIssuer, requeueing")
			return err
		}
	} else if err != nil {
		if errors.IsAlreadyExists(err) {
			// The ClusterIssuer exists, check if we should update the secret name
			if instance.Spec.SharedCA.BYO.SecretName != "" {
				if instance.Spec.SharedCA.BYO.SecretName != clusterIssuerFound.Spec.CA.SecretName {
					clusterIssuer := newClusterIssuer(caClusterIssuerName, instance.Spec.SharedCA.BYO.SecretName)
					// Set CertManager instance as the owner and controller
					if err := controllerutil.SetControllerReference(instance, clusterIssuer, scheme); err != nil {
						return err
					}
					err = client.Update(context.TODO(), clusterIssuer)
					if err != nil {
						log.Error(err, "Error updating CA ClusterIssuer, requeueing")
						return err
					}
				}
			} else if caSecretName != clusterIssuerFound.Spec.CA.SecretName {
				clusterIssuer := newClusterIssuer(caClusterIssuerName, caSecretName)
				// Set CertManager instance as the owner and controller
				if err := controllerutil.SetControllerReference(instance, clusterIssuer, scheme); err != nil {
					return err
				}
				err = client.Update(context.TODO(), clusterIssuer)
				if err != nil {
					log.Error(err, "Error updating CA ClusterIssuer, requeueing")
					return err
				}
			}
		} else {
			log.Error(err, "Error accessing CA ClusterIssuer, requeueing")
			return err
		}

	}
	return nil
}

func removeBYO(client client.Client) error {
	if err := removeClusterIssuer(client); err != nil {
		return err
	}
	return nil
}

func removeForBYO(client client.Client, ns string) error {
	if err := removeCert(client, ns); err != nil {
		return err
	}
	if err := removeSecret(client, ns); err != nil {
		return err
	}
	if err := removeIssuer(client, ns); err != nil {
		return err
	}
	return nil
}

func removeSharedCA(client client.Client, ns string) error {
	if err := removeClusterIssuer(client); err != nil {
		return err
	}
	if err := removeCert(client, ns); err != nil {
		return err
	}
	if err := removeSecret(client, ns); err != nil {
		return err
	}
	if err := removeIssuer(client, ns); err != nil {
		return err
	}
	return nil
}

func removeIssuer(client client.Client, ns string) error {
	ssIssuer := &certmgr.Issuer{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: ssIssuerName, Namespace: ns}, ssIssuer); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	} else {
		if err = client.Delete(context.TODO(), ssIssuer); err != nil {
			return err
		}
	}
	return nil
}

func removeClusterIssuer(client client.Client) error {
	clusterIssuer := &certmgr.ClusterIssuer{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: caClusterIssuerName, Namespace: ""}, clusterIssuer); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	} else {
		if err = client.Delete(context.TODO(), clusterIssuer); err != nil {
			return err
		}
	}
	return nil
}

func removeCert(client client.Client, ns string) error {
	cert := &certmgr.Certificate{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: caCertName, Namespace: ns}, cert); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	} else {
		if err = client.Delete(context.TODO(), cert); err != nil {
			return err
		}
	}
	return nil
}

func removeSecret(client client.Client, ns string) error {
	secret := &corev1.Secret{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: caSecretName, Namespace: ns}, secret); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	} else {
		if err = client.Delete(context.TODO(), secret); err != nil {
			return err
		}
	}
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
