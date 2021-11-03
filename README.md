# Initial Setup
## Clone this repo

```sh
cd ~/workspace
git clone git@github.com:cloudfoundry/cf-k8s-controllers.git
cd cf-k8s-controllers
```
# cf-k8s-controllers
### **NOTE: This section of the README assumes you are working out of the `controllers` subdirectory.**
```sh
cd controllers
```

## Installation


## Prerequisites

### Install Cert-Manager
To deploy cf-k8s-controller and run it in a cluster, you must first [install cert-manager](https://cert-manager.io/docs/installation/).
```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml
```
Or
```sh
kubectl apply -f dependencies/cert-manager.yaml
```

### Install Kpack
To deploy cf-k8s-controller and run it in a cluster, you must first [install kpack](https://github.com/pivotal/kpack/blob/main/docs/install.md).
```sh
kubectl apply -f https://github.com/pivotal/kpack/releases/download/v0.4.1/release-0.4.1.yaml
```
Or
```sh
kubectl apply -f dependencies/kpack-release-0.4.1.yaml
```

#### Configure an Image Registry Credentials Secret
Edit the file: `config/kpack/cluster_builder.yaml` and set the `tag` field to be the registry location you want your ClusterBuilder image to be uploaded to.

Run the command below, substituting the values for the Docker credentials to the registry where images will be uploaded to.
```sh
kubectl create secret docker-registry image-registry-credentials \
    --docker-username="<DOCKER_USERNAME>" \
    --docker-password="<DOCKER_PASSWORD>" \
     --docker-server="<DOCKER_SERVER>" --namespace default
```

#### Configure a Default Builder
```sh
kubectl apply -f dependencies/kpack/service_account.yaml \
    -f dependencies/kpack/cluster_stack.yaml \
    -f dependencies/kpack/cluster_store.yaml \
    -f dependencies/kpack/cluster_builder.yaml
```

### Install Contour and Envoy
To deploy cf-k8s-controller and run it in a cluster, you must first [install contour](https://projectcontour.io/getting-started/).
```sh
kubectl apply -f https://projectcontour.io/quickstart/contour.yaml
```
Or
```sh
kubectl apply -f dependencies/contour-1.19.1.yaml

```

#### Configure Ingress
To enable external access to workloads running on the cluster, you must configure ingress. Generally, a load balancer service will route traffic to the cluster with an external IP and an ingress controller will route the traffic based on domain name or the Host header.

Provisioning a load balancer service is generally handled automatically by Contour, given the cluster infrastructure provider supports load balancer services. When a load balancer service is reconciled, it is assigned an external IP by the infrastructure provider. The external IP is visible on the Service resource itself via `kubectl get service envoy -n projectcontour -o wide`. You can use that external IP to define a DNS A record to route traffic based on your desired domain name.

With the load balancer provisioned, you must configure the ingress controller to route traffic based on your desired domain/host name. For Contour, this configuration goes on an HTTPProxy, for which we have a default resource defined to route traffic to the [CF API](https://github.com/cloudfoundry/cf-k8s-api).

#### Domain Management
To be able to create workload routes via the [CF API](https://github.com/cloudfoundry/cf-k8s-api) in the absence of the domain management endpoints, you must first create the appropriate `CFDomain` resource(s) for your cluster. Each desired domain name should be specified via the `spec.name` property of a distinct resource. The `metadata.name` for the resource can be set to any unique value (the API will use a GUID). See `config/samples/cfdomain.yaml` for an example.

#### Configure a Controller Certificate
Eirini-controller requires a certificate for the controller and webhook. If you are using openssl, or libressl v3.1.0 or later:
```sh
openssl req -x509 -newkey rsa:4096 \
  -keyout tls.key -out tls.crt \
  -nodes -subj '/CN=localhost' \
  -addext "subjectAltName = DNS:*.eirini-controller.svc, DNS:*.eirini-controller.svc.cluster.local" \
  -days 365
```

If you are using an older version of libressl (the default on OSX):
```sh
openssl req -x509 -newkey rsa:4096 \
  -keyout tls.key -out tls.crt \
  -nodes -subj '/CN=localhost' \
  -extensions SAN -config <(cat /etc/ssl/openssl.cnf <(printf "[ SAN ]\nsubjectAltName='DNS:*.eirini-controller.svc, DNS:*.eirini-controller.svc.cluster.local'")) \
  -days 365
```

Once you have created a certificate, use it to create the secret required by eirini-controller:
```
kubectl create secret -n eirini-controller generic eirini-webhooks-certs --from-file=tls.crt=./tls.crt --from-file=tls.ca=./tls.crt --from-file=tls.key=./tls.key
```

#### Install

#### Install Eirini-Controller from release url
To deploy cf-k8s-controller and run it in a cluster, you must first install [eirini-controller](https://github.com/cloudfoundry-incubator/eirini-controller).

#### Install Eirini-Controller from local repository
Clone the [eirini-controller](https://github.com/cloudfoundry-incubator/eirini-controller) repo and go to its root directory, render the Helm chart, and apply it. In this case we use the image built based on the latest commit of main at the time of authoring.
```sh
# Set the certificate authority value for the eirini installation
export webhooks_ca_bundle="$(kubectl get secret -n eirini-controller eirini-webhooks-certs -o jsonpath="{.data['tls\.ca']}")"
# Run from the eirini-controller repository root
helm template eirini-controller "deployment/helm" \
  --values "deployment/helm/values-template.yaml" \
  --set "webhooks.ca_bundle=${webhooks_ca_bundle}" \
  --set "workloads.create_namespaces=true" \
  --set "workloads.default_namespace=cf" \
  --set "controller.registry_secret_name=image-registry-credentials" \
  --set "images.eirini_controller=eirini/eirini-controller@sha256:4dc6547537e30d778e81955065686b6d4d6162821f1ce29f7b80b3aefe20afb3" \
  --namespace "eirini-controller" | kubectl apply -f -
```

---
## Development Workflow

## Running the unit tests
```sh
go test ./...
```

## Running the integration tests
```sh
# First time (pulls kube-apiserver and etcd for envtest):
make test

# Afterwards
export KUBEBUILDER_ASSETS=$PWD/testbin/bin/
go test ./... -tags=integration
```
## Install all dependencies with hack script
```sh
# modify kpack dependency files to point towards your registry
hack/install-dependencies.sh -g "<PATH_TO_GCR_CREDENTIALS>"
```
**Note**: This will not work by default on OSX with a `libressl` version prior to `v3.1.0`. You can install the latest version of `openssl` by running the following commands:
```sh
brew install openssl
ln -s /usr/local/opt/openssl@3/bin/openssl /usr/local/bin/openssl
```

## Configure cf-k8s-controllers
Configuration file for cf-k8s-controllers is at `config/base/cf_k8s_controllers_k8s.yaml`

Note: Edit this file and set the `kpackImageTag` to be the registry location you want for storing the images.

## Build, Install and Deploy to K8s cluster
Set the $IMG environment variable to a location you have push/pull access. For example:
```sh
export IMG=foo/cf-k8s-controllers:bar #Replace this with your image ref
make generate docker-build docker-push install deploy
```
*This will generate the CRD bases, build and push an image with the repository and tag specified by the environment variable, install CRDs and deploy the controller manager. This should cover an entire development loop. Further fine grain instructions are below.*

---
## Using the Makefile
### Set image respository and tag for controller manager
```sh
export IMG=foo/cf-k8s-controllers:bar #Replace this with your image ref
```
### Generate CRD bases
```sh
make generate
```
### Build Docker image
```
make docker-build
```
### Push Docker image
```
make docker-push
```
### Install CRDs to K8s in current Kubernetes context
```
make install
```
### Deploy controller manager to K8s in current Kubernetes context
```
make deploy
```

### Generate reference yaml
Build reference yaml (with defaults) to be applied with kubectl
```
make build-reference
```
---
## Using kubectl
Apply CRDs and controller-manager with reference defaults
```
kubectl apply -f reference/cf-k8s-controllers.yaml
```
---
## Sample Resources
### Apply sample instances of the resources
```
kubectl apply -f config/samples/. --recursive
```
---
# cf-k8s-api
### **NOTE: This section of the README assumes you are working out of the `api` subdirectory.**
```sh
cd api
```
This subdirectory contains what we call the "CF API Shim", an experimental implementation of the V3 Cloud Foundry API that is backed entirely by Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

For more information about what we're building, check out the [Vision for CF on Kubernetes](https://docs.google.com/document/d/1rG814raI5UfGUsF_Ycrr8hKQMo1RH9TRMxuvkgHSdLg/edit) document.

## Documentation
While this project seeks to faithfully reproduce the [V3 CF APIs](https://v3-apidocs.cloudfoundry.org/version/release-candidate/) as much as is feasible, not all APIs have been implemented and not all features are supported.

We maintain our own set of [API Endpoint docs](docs/api.md) for tracking what is currently supported by the Shim.

## Installation

### Dependencies
The api relies on the controllers and CRDs referenced in the previous section of this README.

### Running Locally
make
```make
make run
```
shell
```sh
go run main.go
```

### Deploying the API to your cluster

**Note** Supports ingress with only GKE

### Editing Local Configuration
To specify a custom configuration file, set the `CONFIG` environment variable to its path when running the web server.
Refer to the [default config](config/cf_k8s_api_config.yaml) for the config file structure and options.

Edit the file `config/base/cf_k8s_api_config.yaml` and set the `packageRegistryBase` field to be the registry location you want your source package image to be uploaded to.
Edit the file `config/base/api_url_patch.yaml` to specify the desired URL for the deployed API.

### Using make
You can deploy the app to your cluster by running `make deploy` from the project root.

### Using Kubectl
You can deploy the app to your cluster by running `kubectl apply -f reference/cf-k8s-api.yaml` from the project root.

### Post Deployment

#### Configure Image Registry Credentials Secret
Run the command below, substituting the values for the Docker credentials to the registry where source package images will be uploaded to.

```
kubectl create secret docker-registry image-registry-secret \
  --docker-username="<DOCKER_USERNAME>" \
  --docker-password="<DOCKER_PASSWORD>" \
  --docker-server="<DOCKER_SERVER>" --namespace cf-k8s-api-system
```

#### Configure API Ingress TLS Certificate Secret
Generate a self-signed certificate. If you are using openssl, or libressl v3.1.0 or later:
```
openssl req -x509 -newkey rsa:4096 \
  -keyout tls.key -out tls.crt \
  -nodes -subj '/CN=localhost' \
  -addext "subjectAltName = DNS:localhost" \
  -days 365
```

If you are using an older version of libressl (the default on OSX):
```
openssl req -x509 -newkey rsa:4096 \
  -keyout tls.key -out tls.crt \
  -nodes -subj '/CN=localhost' \
  -extensions SAN -config <(cat /etc/ssl/openssl.cnf <(printf "[ SAN ]\nsubjectAltName='DNS:localhost'")) \
  -days 365
```

Create a TLS secret called `cf-k8s-api-ingress-cert` using the self-signed
certificate generated above, or from your own existing certificate:
```
kubectl create secret tls \
  cf-k8s-api-ingress-cert \
  --cert=./tls.crt --key=./tls.key \
  -n cf-k8s-api-system
```

**NOTE**: If you choose to generate a self-signed certificate, you will need to
either skip TLS validation or use the `--cacert` flag with the generated
certificate when connecting to the API.

### Running Tests
make
```sh
make test
```
shell (testbin must be sourced first if using this method)
```sh
KUBEBUILDER_ASSETS=$PWD/testbin/bin go test ./... -coverprofile cover.out
```

### Updating CRDs for Tests
Some tests run a real Kubernetes API/etcd via the [`envtest`](https://book.kubebuilder.io/reference/envtest.html) package. These tests rely on the CRDs from `controllers` subdirectory.
To update these CRDs refer to the `cf-k8s-controllers` installation instructions above.

## Regenerate kubernetes resources after making changes
To regenerate the kubernetes resources under `./config`, run `make manifests` from the root of the project.

## Generate reference yaml

```sh
make build-reference
```

## Contributing
This project follows [Cloud Foundry Code of Conduct](https://www.cloudfoundry.org/code-of-conduct/)
