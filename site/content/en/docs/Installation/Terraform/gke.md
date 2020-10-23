---
title: "Installing Agones on Google Kubernetes Engine using Terraform"
linkTitle: "Google Cloud"
weight: 10
description: >
  You can use Terraform to provision a GKE cluster and install Agones on it.
---

## Before you begin

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
1. From the top-right corner of the console, click the 
   **Activate Google Cloud Shell** button: ![cloud shell](../../../../images/cloud-shell.png)
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

## Installation

An example configuration can be found here:
 {{< ghlink href="examples/terraform-submodules/gke/module.tf" >}}Terraform configuration with Agones submodule{{< /ghlink >}}. 
 
Copy this file into a local directory where you will execute the terraform commands.

The GKE cluster created from the example configuration will contain 3 Node Pools:

- `"default"` node pool with `"game-server"` tag, containing 4 nodes.
- `"agones-system"` node pool for Agones Controller.
- `"agones-metrics"` for monitoring and metrics collecting purpose.

Additionally, a `"tiller"` service account will be created with ClusterRole.

Configurable parameters:

- project - your Google Cloud Project ID (required)
- name - the name of the GKE cluster (default is "agones-terraform-example")
- agones_version - the version of agones to install (an empty string, which is the default, is the latest version from the [Helm repository](https://agones.dev/chart/stable))
- machine_type - machine type for hosting game servers (default is "n1-standard-4")
- node_count - count of game server nodes for the default node pool (default is "4")
- zone - the name of the [zone](https://cloud.google.com/compute/docs/regions-zones) you want your cluster to be
  created in (default is "us-west1-c")
- network - the name of the VPC network you want your cluster and firewall rules be connected to (default is "default")
- log_level - possible values: Fatal, Error, Warn, Info, Debug (default is "info")
- feature_gates - a list of alpha and beta version features to enable. For example, "PlayerTracking=true&ContainerPortAllocation=true"

{{% alert title="Warning" color="warning"%}}
On the lines that read `source = "git::https://github.com/googleforgames/agones.git//install/terraform/modules/gke/?ref=master"`
make sure to change `?ref=master` to match your targeted Agones release, as Terraform modules can change between
releases.

For example, if you are targeting release {{< release-branch >}}, then you will want to have 
`source = "git::https://github.com/googleforgames/agones.git//install/terraform/modules/gke/?ref=release-{{< release-branch >}}"`
as your source.
{{% /alert %}}

### Creating the cluster

In the directory where you created `module.tf`, run:
```
terraform init
```

This will cause terraform to clone the Agones repository and use the `./install/terraform` folder as starting point of
Agones submodule, which contains all necessary Terraform configuration files.

Next make sure that you can authenticate using gcloud:
```
gcloud auth application-default login
```

Now you can create your GKE cluster (optionally specifying the version of Agones you want to use):
```
terraform apply -var project="<YOUR_GCP_ProjectID>" [-var agones_version="1.0.0"]
```

To verify that the cluster was created successfully, set up your kubectl credentials:
```
gcloud container clusters get-credentials --zone us-west1-c agones-terraform-example
```

Then check that you have access to the Kubernetes cluster:
```
kubectl get nodes
```

You should have 6 nodes in `Ready` state.

### Uninstall the Agones and delete GKE cluster

To delete all resources provisioned by Terraform:
```
terraform destroy
```

## Next Steps

- [Confirm Agones is up and running]({{< relref "../confirm.md" >}})