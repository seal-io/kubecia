# Kubernetes Cloud Identify Authenticator(KubeCIA)

> tl;dr: Available [client-go credential (exec) plugin](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins), no Cloud Provider CLI required.

[![](https://goreportcard.com/badge/github.com/seal-io/kubecia)](https://goreportcard.com/report/github.com/seal-io/kubecia)
[![](https://img.shields.io/github/actions/workflow/status/seal-io/kubecia/ci.yml?label=ci)](https://github.com/seal-io/kubecia/actions)
[![](https://img.shields.io/github/v/tag/seal-io/kubecia?label=release)](https://github.com/seal-io/kubecia/releases)
[![](https://img.shields.io/github/downloads/seal-io/kubecia/total)](https://github.com/seal-io/kubecia/releases)
[![](https://img.shields.io/github/license/seal-io/kubecia?label=license)](https://github.com/seal-io/kubecia#license)

This tool is maintained by [Seal](https://github.com/seal-io).

## Background

Since Kubernetes v1.22, we can use external credential plugins to authenticate with Kubernetes clusters. However, using
external credential plugins requires the Cloud Provider CLI to be installed, for some scenarios, such as CI/CD, it is
overkill and not friendly to automation task preparation.

```shell
$ docker images --format "{{.Repository}}:{{.Tag}}\t{{.Size}}" | grep cli

gcr.io/google.com/cloudsdktool/google-cloud-cli:latest	2.82GB
public.ecr.aws/aws-cli/aws-cli:latest	415MB
mcr.microsoft.com/azure-cli:latest	722MB
```

KubeCIA, which is a lightweight and easy-to-use credential plugin for Kubernetes, is born to reduce the dependency of
Cloud Provider CLI.

## Usage

KubeCIA can call the Cloud Provider API to get the credential and consume the local filesystem as caching. The following
example shows how to use KubeCIA to get credentials for EKS cluster.

```yaml
apiVersion: v1
kind: Config
users:
  - name: eks-user
    user:
      exec:
        # -- KubeCIA only supports `client.authentication.k8s.io/v1` API version.
        apiVersion: "client.authentication.k8s.io/v1"
        # -- API version `client.authentication.k8s.io/v1` needs configuring `interactiveMode`.
        interactiveMode: Never
        command: "kubecia"
        args:
          - "aws"
        env:
          # -- KubeCIA can retrieve the environment variables prefixed with `KUBECIA_`,
          # -- second segment must be the upper case of the sub command.
          - name: KUBECIA_AWS_ACCESS_KEY_ID
            value: <REPLACE_WITH_YOUR_AWS_ACCESS_KEY_ID>
          - name: KUBECIA_AWS_SECRET_ACCESS_KEY
            # -- For sensitive value, KubeCIA will try to expand from the environment variable.
            value: "$AWS_SECRET_ACCESS_KEY"
          - name: KUBECIA_AWS_REGION
            value: <REPLACE_WITH_YOUR_AWS_REGION>
          - name: KUBECIA_AWS_CLUSTER
            value: <REPLACE_WITH_YOUR_EKS_CLUSTER_ID_OR_NAME>
          - name: KUBECIA_AWS_ASSUME_ROLE_ARN
            value: <REPLACE_WITH_YOUR_EKS_ASSUME_ROLE_ARN>
clusters:
  - name: eks-cluster
    cluster:
      server: <REPLACE_WITH_YOUR_EKS_ENDPOINT>
      certificate-authority: <REPLACE_WITH_YOUR_EKS_CA_PEM_PATH>
contexts:
  - name: eks-cluster
    context:
      cluster: eks-cluster
      user: eks-user
current-context: eks-cluster
```

### Centralized Service Mode

KubeCIA can be set up as a centralized service by `kubecia serve` command.

```shell
$ kubecia serve --socket /var/run/kubecia.sock
```

Under this mode, the above configuration can also work.

When acting as a sidecar, main containers can
use any Unix socket tool to call centralized KubeCIA service, the following example shows how to
use [cURL(7.40.0+)](https://curl.se/libcurl/c/CURLOPT_UNIX_SOCKET_PATH.html) to get.

```yaml
apiVersion: v1
kind: Config
users:
  - name: eks-user
    user:
      exec:
        apiVersion: "client.authentication.k8s.io/v1"
        command: "curl"
        args:
          - "--silent"
          - "--output"
          - "-"
          - "--location"
          - "--unix-socket"
          # -- KubeCIA service will listen on this Unix socket at default, change it by `--socket` flag.
          - "/var/run/kubecia.sock"
          - "--user"
          # -- The service principal credentials, e.g. the AWS access_key_id and secret_access_key, the Azure client_id and client_secret,
          # -- are required to be provided via `Authentication` header.
          - "<REPLACE_WITH_YOUR_AWS_ACCESS_KEY_ID>:<REPLACE_WITH_YOUR_AWS_SECRET_ACCESS_KEY>"
          - "http:/./aws/<REPLACE_WITH_YOUR_AWS_REGION>/<REPLACE_WITH_YOUR_EKS_CLUSTER_ID_OR_NAME>/<REPLACE_WITH_YOUR_EKS_ROLE_ARN>"
        interactiveMode: Never
clusters:
  - name: eks-cluster
    cluster:
      server: <REPLACE_WITH_YOUR_EKS_ENDPOINT>
      certificate-authority: <REPLACE_WITH_YOUR_EKS_CA_PEM_PATH>
contexts:
  - name: eks-cluster
    context:
      cluster: eks-cluster
      user: eks-user
current-context: eks-cluster
```

But describing the sensitive credentials in the configuration file is not recommended, it is recommended to use the
shell injection as below.

```yaml 
apiVersion: v1
kind: Config
users:
  - name: eks-user
    user:
      exec:
        apiVersion: "client.authentication.k8s.io/v1"
        command: "/bin/bash"
        args:
          - "-c"
          - "curl --silent --output - --location --unix-socket /var/run/kubecia.sock --user ${AWS_ACCESS_KEY_ID}:${AWS_SECRET_ACCESS_KEY} http:/./aws/${AWS_REGION}/${EKS_CLUSTER}/${EKS_ROLE_ARN}"
        env:
          ##
          ## AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are optional present at here,
          ## there can be provided by environment variables.
          ##
          # - name: AWS_ACCESS_KEY_ID
          #  value: <REPLACE_WITH_YOUR_AWS_ACCESS_KEY_ID>
          # - name: AWS_SECRET_ACCESS_KEY
          #  value: <REPLACE_WITH_YOUR_AWS_SECRET_ACCESS_KEY>
          - name: AWS_REGION
            value: <REPLACE_WITH_YOUR_AWS_REGION>
          - name: EKS_CLUSTER
            value: <REPLACE_WITH_YOUR_EKS_CLUSTER_ID_OR_NAME>
          - name: EKS_ROLE_ARN
            value: <REPLACE_WITH_YOUR_EKS_ROLE_ARN>
        interactiveMode: Never
clusters:
  - name: eks-cluster
    cluster:
      server: <REPLACE_WITH_YOUR_EKS_ENDPOINT>
      certificate-authority: <REPLACE_WITH_YOUR_EKS_CA_PEM_PATH>
contexts:
  - name: eks-cluster
    context:
      cluster: eks-cluster
      user: eks-user
current-context: eks-cluster
```

## Notice

KubeCIA only response result with `apiVersion: "client.authentication.k8s.io/v1"`, please update the kubectl if not
supported.

KubeCIA focuses on obtaining the token for accessing the Kubernetes cluster based on the user's service principal
credential, to other features or modes, please review the below links.

- [kubernetes-sigs/aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator)
- [Azure/kubelogin](https://github.com/Azure/kubelogin)
- [Here's what to know about changes to kubectl authentication coming in GKE v1.26.](https://cloud.google.com/blog/products/containers-kubernetes/kubectl-auth-changes-in-gke)

KubeCIA establishes on Unix socket, to expose the service to the network, please use the socket proxy, like
[ncat](https://nmap.org/ncat/guide/index.html).

```shell
$ ncat --verbose --listen --keep-open --source-port 80 --sh-exec "ncat --unixsock /var/run/kubecia.sock"
```

# License

Copyright (c) 2024 [Seal, Inc.](https://seal.io)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at [LICENSE](./LICENSE) file for details.

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
