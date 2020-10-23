---
title: "Install and configure Agones on Kubernetes"
linkTitle: "Installation"
weight: 4
description: >
  Follow this guide to create a cluster on Google Kubernetes Engine (GKE), Minikube, Amazon Elastic Kubernetes Service (EKS), or Azure Kubernetes Service (AKS), and install Agones.
---

In this quickstart, we will create a Kubernetes cluster, and populate it with the resource types that power Agones.

{{< alert title="Note" color="info">}}
When running in production, Agones should be scheduled on a dedicated pool of nodes, distinct from where Game Servers
are scheduled for better isolation and resiliency. By default Agones prefers to be scheduled on nodes labeled with
`agones.dev/agones-system=true` and tolerates the node taint `agones.dev/agones-system=true:NoExecute`.
If no dedicated nodes are available, Agones will run on regular nodes.
{{< /alert >}}

## Usage Requirements

{{% feature expiryVersion="1.2.0" %}}
- Kubernetes cluster version 1.12
    - [Minikube](https://github.com/kubernetes/minikube), [Kind](https://github.com/kubernetes-sigs/kind), [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/),
      [Azure Kubernetes Service](https://azure.microsoft.com/en-us/services/kubernetes-service/) and [Amazon EKS](https://aws.amazon.com/eks/) have been tested
    - If you are creating and managing your own Kubernetes cluster, the
    [MutatingAdmissionWebhook](https://kubernetes.io/docs/admin/admission-controllers/#mutatingadmissionwebhook-beta-in-19), and
    [ValidatingAdmissionWebhook](https://kubernetes.io/docs/admin/admission-controllers/#validatingadmissionwebhook-alpha-in-18-beta-in-19)
    admission controllers are required.
    We also recommend following the
    [recommended set of admission controllers](https://kubernetes.io/docs/admin/admission-controllers/#is-there-a-recommended-set-of-admission-controllers-to-use).
- Firewall access for the range of ports that Game Servers can be connected to in the cluster.
- Game Servers must have the [game server SDK]({{< ref "/docs/Guides/Client SDKs/_index.md"  >}}) integrated, to manage Game Server state, health checking, etc.
{{% /feature %}}

{{% feature publishversion="1.2.0" %}}
- Kubernetes cluster version 1.13
    - [Minikube](https://github.com/kubernetes/minikube), [Kind](https://github.com/kubernetes-sigs/kind), [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/),
      [Azure Kubernetes Service](https://azure.microsoft.com/en-us/services/kubernetes-service/) and [Amazon EKS](https://aws.amazon.com/eks/) have been tested
    - If you are creating and managing your own Kubernetes cluster, the
    [MutatingAdmissionWebhook](https://kubernetes.io/docs/admin/admission-controllers/#mutatingadmissionwebhook-beta-in-19), and
    [ValidatingAdmissionWebhook](https://kubernetes.io/docs/admin/admission-controllers/#validatingadmissionwebhook-alpha-in-18-beta-in-19)
    admission controllers are required.
    We also recommend following the
    [recommended set of admission controllers](https://kubernetes.io/docs/admin/admission-controllers/#is-there-a-recommended-set-of-admission-controllers-to-use).
- Firewall access for the range of ports that Game Servers can be connected to in the cluster.
- Game Servers must have the [game server SDK]({{< ref "/docs/Guides/Client SDKs/_index.md"  >}}) integrated, to manage Game Server state, health checking, etc.
{{% /feature %}}

{{% feature expiryVersion="1.2.0" %}}
{{< alert title="Warning" color="warning">}}
Later versions of Kubernetes may work, but this project is tested against 1.12, and is therefore the supported version.
Agones will update its support to n-1 version of what is available across all major cloud providers - GKE, EKS and AKS
{{< /alert >}}
{{% /feature %}}

{{% feature publishversion="1.2.0" %}}
{{< alert title="Warning" color="warning">}}
Later versions of Kubernetes may work, but this project is tested against 1.13, and is therefore the supported version.
Agones will update its support to n-1 version of what is available across all major cloud providers - GKE, EKS and AKS
{{< /alert >}}
{{% /feature %}}

## Setting up a Google Kubernetes Engine (GKE) cluster

Follow these steps to create a cluster and install Agones directly on Google Kubernetes Engine (GKE).

### Before you begin

Take the following steps to enable the Kubernetes Engine API:

1. Visit the [Kubernetes Engine][kubernetes] page in the Google Cloud Platform Console.
1. Create or select a project.
1. Wait for the API and related services to be enabled. This can take several minutes.
1. [Enable billing][billing] for your project.
  * If you are not an existing GCP user, you may be able to enroll for a $300 US [Free Trial][trial] credit.

[kubernetes]: https://console.cloud.google.com/kubernetes/list
[billing]: https://support.google.com/cloud/answer/6293499#enable-billing
[trial]: https://cloud.google.com/free/

### Choosing a shell

To complete this quickstart, we can use either [Google Cloud Shell][cloud-shell] or a local shell.

Google Cloud Shell is a shell environment for managing resources hosted on Google Cloud Platform (GCP). Cloud Shell comes preinstalled with the [gcloud][gcloud] and [kubectl][kubectl] command-line tools. `gcloud` provides the primary command-line interface for GCP, and `kubectl` provides the command-line interface for running commands against Kubernetes clusters.

If you prefer using your local shell, you must install the gcloud and kubectl command-line tools in your environment.

[cloud-shell]: https://cloud.google.com/shell/
[gcloud]: https://cloud.google.com/sdk/gcloud/
[kubectl]: https://kubernetes.io/docs/user-guide/kubectl-overview/

#### Cloud shell

To launch Cloud Shell, perform the following steps:

1. Go to [Google Cloud Platform Console][cloud]
1. From the top-right corner of the console, click the **Activate Google Cloud Shell** button: ![cloud shell](cloud-shell.png)
1. A Cloud Shell session opens inside a frame at the bottom of the console. Use this shell to run `gcloud` and `kubectl` commands.
1. Set a compute zone in your geographical region with the following command. The compute zone will be something like `us-west1-a`. A full list can be found [here][zones].
   ```bash
   gcloud config set compute/zone [COMPUTE_ZONE]
   ```

[cloud]: https://console.cloud.google.com/home/dashboard
[zones]: https://cloud.google.com/compute/docs/regions-zones/#available

#### Local shell

To install `gcloud` and `kubectl`, perform the following steps:

1. [Install the Google Cloud SDK][gcloud-install], which includes the `gcloud` command-line tool.
1. Initialize some default configuration by running the following command.
   * When asked `Do you want to configure a default Compute Region and Zone? (Y/n)?`, enter `Y` and choose a zone in your geographical region of choice.
   ```bash
   gcloud init
   ```
1. Install the `kubectl` command-line tool by running the following command:
   ```bash
   gcloud components install kubectl
   ```

[gcloud-install]: https://cloud.google.com/sdk/docs/quickstarts

### Creating the cluster

A [cluster][cluster] consists of at least one *cluster master* machine and multiple worker machines called *nodes*: [Compute Engine virtual machine][vms] instances that run the Kubernetes processes necessary to make them part of the cluster.

{{% feature expiryVersion="1.2.0" %}}
```bash
gcloud container clusters create [CLUSTER_NAME] --cluster-version=1.12 \
  --tags=game-server \
  --scopes=gke-default \
  --num-nodes=4 \
  --no-enable-autoupgrade \
  --machine-type=n1-standard-4
```
{{% /feature %}}
{{% feature publishversion="1.2.0" %}}
```bash
gcloud container clusters create [CLUSTER_NAME] --cluster-version=1.13 \
  --tags=game-server \
  --scopes=gke-default \
  --num-nodes=4 \
  --no-enable-autoupgrade \
  --machine-type=n1-standard-4
```
{{% /feature %}}

Flag explanations:

{{% feature expiryVersion="1.2.0" %}}
* cluster-version: Agones requires Kubernetes version 1.12.
{{% /feature %}}
{{% feature publishversion="1.2.0" %}}
* cluster-version: Agones requires Kubernetes version 1.13.
{{% /feature %}}
* tags: Defines the tags that will be attached to new nodes in the cluster. This is to grant access through ports via the firewall created in the next step.
* scopes: Defines the Oauth scopes required by the nodes.
* num-nodes: The number of nodes to be created in each of the cluster's zones. Default: 4. Depending on the needs of your game, this parameter should be adjusted.
* no-enable-autoupgrade: Disable automatic upgrades for nodes to reduce the likelihood of in-use games being disrupted.
* machine-type: The type of machine to use for nodes. Default: n1-standard-4. Depending on the needs of your game, you may wish to [have smaller or larger machines](https://cloud.google.com/compute/docs/machine-types).

_Optional_: Create a dedicated node pool for the Agones controllers. If you choose to skip this step, the Agones
controllers will share the default node pool with your game servers which is fine for kicking the tires but is not
recommended for a production deployment.

```bash
gcloud container node-pools create agones-system \
  --cluster=[CLUSTER_NAME] \
  --no-enable-autoupgrade \
  --node-taints agones.dev/agones-system=true:NoExecute \
  --node-labels agones.dev/agones-system=true \
  --num-nodes=1
```

_Optional_: Create a node pool for [Metrics]({{< relref "../Guides/metrics.md" >}}) if you want to monitor the Agones system using Prometheus with Grafana or Stackdriver.

```bash
gcloud container node-pools create agones-metrics \
  --cluster=[CLUSTER_NAME] \
  --no-enable-autoupgrade \
  --node-taints agones.dev/agones-metrics=true:NoExecute \
  --node-labels agones.dev/agones-metrics=true \
  --num-nodes=1
```

Flag explanations:

* cluster: The name of the cluster in which the node pool is created.
* no-enable-autoupgrade: Disable automatic upgrades for nodes to reduce the likelihood of in-use games being disrupted.
* node-taints: The Kubernetes taints to automatically apply to nodes in this node pool.
* node-labels: The Kubernetes labels to automatically apply to nodes in this node pool.
* num-nodes: The Agones system controllers only require a single node of capacity to run. For faster recovery time in the event of a node failure, you can increase the size to 2.

Finally, let's tell `gcloud` that we are speaking with this cluster, and get auth credentials for `kubectl` to use.

```bash
gcloud config set container/cluster [CLUSTER_NAME]
gcloud container clusters get-credentials [CLUSTER_NAME]
```

[cluster]: https://cloud.google.com/kubernetes-engine/docs/concepts/cluster-architecture
[vms]: https://cloud.google.com/compute/docs/instances/

#### Creating the firewall

We need a firewall to allow UDP traffic to nodes tagged as `game-server` via ports 7000-8000.

```bash
gcloud compute firewall-rules create game-server-firewall \
  --allow udp:7000-8000 \
  --target-tags game-server \
  --description "Firewall to allow game server udp traffic"
```

### Follow Normal Instructions to Install

Continue to [Installing Agones](#installing-agones).

## Setting up a Minikube cluster

This will setup a [Minikube](https://github.com/kubernetes/minikube) cluster, running on an `agones` profile.

### Installing Minikube

First, [install Minikube][minikube], which may also require you to install
a virtualisation solution, such as [VirtualBox][vb] as well.

[minikube]: https://github.com/kubernetes/minikube#installation
[vb]: https://www.virtualbox.org

### Creating an `agones` profile

Let's use a minikube profile for `agones`.

```bash
minikube profile agones
```

### Starting Minikube

The following command starts a local minikube cluster via virtualbox - but this can be
replaced by a [vm-driver](https://github.com/kubernetes/minikube#requirements) of your choice.

{{% feature expiryVersion="1.2.0" %}}
```bash
minikube start --kubernetes-version v1.12.10 --vm-driver virtualbox
```
{{% /feature %}}
{{% feature publishversion="1.2.0" %}}
```bash
minikube start --kubernetes-version v1.13.12 --vm-driver virtualbox
```
{{% /feature %}}

### Follow Normal Instructions to Install

Continue to [Installing Agones](#installing-agones).

## Setting up an Amazon Web Services EKS cluster

### Create EKS Cluster

Create your EKS Cluster using the [Getting Started Guide](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html).

Possible steps are the following:
1. Create new IAM role for cluster management.
1. Run `aws configure` to authorize your `awscli` with proper `AWS Access Key ID` and `AWS Secret Access Key`.
1. Create an example cluster:
{{% feature expiryVersion="1.2.0" %}}
```
eksctl create cluster \
--name prod \
--version 1.12 \
--nodegroup-name standard-workers \
--node-type t3.medium \
--nodes 3 \
--nodes-min 3 \
--nodes-max 4 \
--node-ami auto
```
{{% /feature %}}
{{% feature publishversion="1.2.0" %}}
```
eksctl create cluster \
--name prod \
--version 1.13 \
--nodegroup-name standard-workers \
--node-type t3.medium \
--nodes 3 \
--nodes-min 3 \
--nodes-max 4 \
--node-ami auto
```
{{% /feature %}}

{{< alert title="Note" color="info">}}
EKS does not use the normal Kubernetes networking since it is [incompatible with Amazon VPC networking](https://www.contino.io/insights/kubernetes-is-hard-why-eks-makes-it-easier-for-network-and-security-architects).
{{< /alert >}}

#### Allowing UDP Traffic

For Agones to work correctly, we need to allow UDP traffic to pass through to our EKS cluster worker nodes. To achieve this, we must update the workers' nodepool SG (Security Group) with the proper rule. A simple way to do that is:

* Login to the AWS Management Console
* Go to the VPC Dashboard and select **Security Groups**
* Find the Security Group for the workers nodepool, which will be named something like `eksctl-[cluster-name]-nodegroup-[cluster-name]-workers/SG`
* Select **Inbound Rules**
* **Edit Rules** to add a new **Custom UDP Rule** with a 7000-8000 port range and an appropriate **Source** CIDR range (`0.0.0.0/0` allows all traffic)

### Follow Normal Instructions to Install

Continue to [Installing Agones](#installing-agones).

## Setting up an Azure Kubernetes Service (AKS) Cluster

Follow these steps to create a cluster and install Agones directly on [Azure Kubernetes Service (AKS) ](https://docs.microsoft.com/azure/aks/).

### Choosing your shell

You can use either [Azure Cloud Shell](https://docs.microsoft.com/azure/cloud-shell/overview) or install the [Azure CLI](https://docs.microsoft.com/cli/azure/?view=azure-cli-latest) on your local shell in order to install AKS in your own Azure subscription. Cloud Shell comes preinstalled with `az` and `kubectl` utilities whereas you need to install them locally if you want to use your local shell. If you use Windows 10, you can use the [WIndows Subsystem for Windows](https://docs.microsoft.com/windows/wsl/install-win10) as well.

### Creating the AKS cluster

If you are using Azure CLI from your local shell, you need to login to your Azure account by executing the `az login` command and following the login procedure.

Here are the steps you need to follow to create a new AKS cluster (additional instructions and clarifications are listed [here](https://docs.microsoft.com/azure/aks/kubernetes-walkthrough)):

{{% feature expiryVersion="1.2.0" %}}
```bash
# Declare necessary variables, modify them according to your needs
AKS_RESOURCE_GROUP=akstestrg     # Name of the resource group your AKS cluster will be created in
AKS_NAME=akstest     # Name of your AKS cluster
AKS_LOCATION=westeurope     # Azure region in which you'll deploy your AKS cluster

# Create the Resource Group where your AKS resource will be installed
az group create --name $AKS_RESOURCE_GROUP --location $AKS_LOCATION

# Create the AKS cluster - this might take some time. Type 'az aks create -h' to see all available options
# The following command will create a single Node AKS cluster. Node size is Standard A1 v1 and Kubernetes version is 1.11.8. Plus, SSH keys will be generated for you, use --ssh-key-value to provide your values
az aks create --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME --node-count 1 --generate-ssh-keys --node-vm-size Standard_A4_v2 --kubernetes-version 1.11.8

# Install kubectl
sudo az aks install-cli

# Get credentials for your new AKS cluster
az aks get-credentials --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME
```
{{% /feature %}}
{{% feature publishversion="1.2.0" %}}
```bash
# Declare necessary variables, modify them according to your needs
AKS_RESOURCE_GROUP=akstestrg     # Name of the resource group your AKS cluster will be created in
AKS_NAME=akstest     # Name of your AKS cluster
AKS_LOCATION=westeurope     # Azure region in which you'll deploy your AKS cluster

# Create the Resource Group where your AKS resource will be installed
az group create --name $AKS_RESOURCE_GROUP --location $AKS_LOCATION

# Create the AKS cluster - this might take some time. Type 'az aks create -h' to see all available options
# The following command will create a four Node AKS cluster. Node size is Standard A1 v1 and Kubernetes version is 1.13.12. Plus, SSH keys will be generated for you, use --ssh-key-value to provide your values
az aks create --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME --node-count 4 --generate-ssh-keys --node-vm-size Standard_A4_v2 --kubernetes-version 1.13.12

# Install kubectl
sudo az aks install-cli

# Get credentials for your new AKS cluster
az aks get-credentials --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME
```
{{% /feature %}}

Alternatively, you can use the [Azure Portal](https://portal.azure.com) to create a new AKS cluster [(instructions)](https://docs.microsoft.com/azure/aks/kubernetes-walkthrough-portal).

#### Allowing UDP traffic

For Agones to work correctly, we need to allow UDP traffic to pass through to our AKS cluster. To achieve this, we must update the NSG (Network Security Group) with the proper rule. A simple way to do that is:

* Login to the Azure Portal
* Find the resource group where the AKS resources are kept, which should have a name like `MC_resourceGroupName_AKSName_westeurope`. Alternative, you can type `az resource show --namespace Microsoft.ContainerService --resource-type managedClusters -g $AKS_RESOURCE_GROUP -n $AKS_NAME -o json | jq .properties.nodeResourceGroup`
* Find the Network Security Group object, which should have a name like `aks-agentpool-********-nsg`
* Select **Inbound Security Rules**
* Select **Add** to create a new Rule with **UDP** as the protocol and **7000-8000** as the Destination Port Ranges. Pick a proper name and leave everything else at their default values

Alternatively, you can use the following command, after modifying the `RESOURCE_GROUP_WITH_AKS_RESOURCES` and `NSG_NAME` values:

```bash
az network nsg rule create \
  --resource-group RESOURCE_GROUP_WITH_AKS_RESOURCES \
  --nsg-name NSG_NAME \
  --name AgonesUDP \
  --access Allow \
  --protocol Udp \
  --direction Inbound \
  --priority 520 \
  --source-port-range "*" \
  --destination-port-range 7000-8000
  ```

#### Creating and assigning Public IPs to Nodes

Nodes in AKS don't get a Public IP by default. To assign a Public IP to a Node, find the Resource Group where the AKS resources are installed on the [portal](https://portal.azure.com) (it should have a name like `MC_resourceGroupName_AKSName_westeurope`). Then, you can follow the instructions [here](https://docs.microsoft.com/en-us/azure/site-recovery/concepts-public-ip-address-with-site-recovery) to create a new Public IP and assign it to the Node/VM. For more information on Public IPs for VM NICs, see [this document](https://docs.microsoft.com/azure/virtual-network/virtual-network-network-interface-addresses). If you are looking for an automated way to create and assign Public IPs for your AKS Nodes, check [this project](https://github.com/dgkanatsios/AksNodePublicIPController).

### Follow Normal Instructions to Install

Continue to [Installing Agones](#installing-agones).

## Installing Agones

This will install Agones in your cluster.

### Install with YAML

We can install Agones to the cluster using the
[install.yaml](https://github.com/googleforgames/agones/blob/{{< release-branch >}}/install/yaml/install.yaml) file.

```bash
kubectl create namespace agones-system
kubectl apply -f https://raw.githubusercontent.com/googleforgames/agones/{{< release-branch >}}/install/yaml/install.yaml
```

You can also find the install.yaml in the latest `agones-install` zip from the [releases](https://github.com/googleforgames/agones/releases) archive.

{{< alert title="Warning" color="warning">}}
Installing Agones with the `install.yaml` will setup the TLS certificates stored in this repository for securing
kubernetes webhooks communication. If you want to generate new certificates or use your own,
we recommend using the helm installation.
{{< /alert >}}

### Install using Helm

Also, we can install Agones using [Helm][helm] package manager. If you want more details and configuration
options see the [Helm installation guide for Agones][agones-install-guide]

[helm]: https://docs.helm.sh
[agones-install-guide]: {{< relref "helm.md" >}}

### Confirming Agones started successfully

To confirm Agones is up and running, run the following command:

```bash
kubectl describe --namespace agones-system pods
```

It should describe six pods created in the `agones-system` namespace, with no error messages or status. All `Conditions` sections should look like this:

```
Conditions:
  Type              Status
  Initialized       True
  Ready             True
  ContainersReady   True
  PodScheduled      True
```

All this pods should be in a `RUNNING` state:


```bash
kubectl get pods --namespace agones-system

NAME                                 READY   STATUS    RESTARTS   AGE
agones-allocator-5c988b7b8d-cgtbs    1/1     Running   0          8m47s
agones-allocator-5c988b7b8d-hhhr5    1/1     Running   0          8m47s
agones-allocator-5c988b7b8d-pv577    1/1     Running   0          8m47s
agones-controller-7db45966db-56l66   1/1     Running   0          8m44s
agones-ping-84c64f6c9d-bdlzh         1/1     Running   0          8m37s
agones-ping-84c64f6c9d-sjgzz         1/1     Running   0          8m47s
```

That's it! This creates the [Custom Resource Definitions][crds] that power Agones and allows us to define resources of type `GameServer`.

[crds]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/

## What's next

* Go through the [Create a Game Server Quickstart][quickstart]

[quickstart]: {{< ref "/docs/Getting Started/create-gameserver.md" >}}
