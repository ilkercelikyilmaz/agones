# Copyright 2019 Google LLC All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# The GKE development cluster name
GCP_TF_CLUSTER_NAME ?= agones-tf-cluster

### Deploy cluster with Terraform
terraform-init:
terraform-init: $(ensure-build-image)
	docker run --rm -it $(common_mounts) $(DOCKER_RUN_ARGS) $(build_tag) bash -c '\
	cd $(mount_path)/build/terraform/gke && terraform init && gcloud auth application-default login'

terraform-clean:
	rm -r ../build/terraform/gke/.terraform || true
	rm ../build/terraform/gke/terraform.tfstate* || true

# Creates a cluster and install release version of Agones controller
# Version could be specified by AGONES_VERSION
gcloud-terraform-cluster: GCP_CLUSTER_NODEPOOL_INITIALNODECOUNT ?= 4
gcloud-terraform-cluster: GCP_CLUSTER_NODEPOOL_MACHINETYPE ?= n1-standard-4
gcloud-terraform-cluster: AGONES_VERSION ?= ''
gcloud-terraform-cluster: GCP_TF_CLUSTER_NAME ?= agones-tf-cluster
gcloud-terraform-cluster: LOG_LEVEL ?= debug
gcloud-terraform-cluster: $(ensure-build-image)
gcloud-terraform-cluster:
ifndef GCP_PROJECT
	$(eval GCP_PROJECT=$(shell sh -c "gcloud config get-value project 2> /dev/null"))
endif
	$(DOCKER_RUN) bash -c 'cd $(mount_path)/build/terraform/gke && \
		 terraform apply -auto-approve -var agones_version="$(AGONES_VERSION)" \
		-var name=$(GCP_TF_CLUSTER_NAME) -var machine_type="$(GCP_CLUSTER_NODEPOOL_MACHINETYPE)" \
		-var values_file="" \
		-var zone="$(GCP_CLUSTER_ZONE)" -var project="$(GCP_PROJECT)" \
		-var log_level="$(LOG_LEVEL)" \
		-var node_count=$(GCP_CLUSTER_NODEPOOL_INITIALNODECOUNT)'
	GCP_CLUSTER_NAME=$(GCP_TF_CLUSTER_NAME) $(MAKE) gcloud-auth-cluster

# Creates a cluster and install current version of Agones controller
# Set all necessary variables as `make install` does
# Unifies previous `make gcloud-test-cluster` and `make install` targets
gcloud-terraform-install: GCP_CLUSTER_NODEPOOL_INITIALNODECOUNT ?= 4
gcloud-terraform-install: GCP_CLUSTER_NODEPOOL_MACHINETYPE ?= n1-standard-4
gcloud-terraform-install: ALWAYS_PULL_SIDECAR := true
gcloud-terraform-install: IMAGE_PULL_POLICY := "Always"
gcloud-terraform-install: PING_SERVICE_TYPE := "LoadBalancer"
gcloud-terraform-install: CRD_CLEANUP := true
gcloud-terraform-install: GCP_TF_CLUSTER_NAME ?= agones-tf-cluster
gcloud-terraform-install: LOG_LEVEL ?= debug
gcloud-terraform-install:
ifndef GCP_PROJECT
	$(eval GCP_PROJECT=$(shell sh -c "gcloud config get-value project 2> /dev/null"))
endif
	$(DOCKER_RUN) bash -c ' \
	cd $(mount_path)/build/terraform/gke && terraform apply -auto-approve -var agones_version="$(VERSION)" -var image_registry="$(REGISTRY)" \
		-var pull_policy="$(IMAGE_PULL_POLICY)" \
		-var always_pull_sidecar="$(ALWAYS_PULL_SIDECAR)" \
		-var image_pull_secret="$(IMAGE_PULL_SECRET)" \
		-var ping_service_type="$(PING_SERVICE_TYPE)" \
		-var crd_cleanup="$(CRD_CLEANUP)" \
		-var chart="../../../install/helm/agones/" \
		-var name=$(GCP_TF_CLUSTER_NAME) -var machine_type="$(GCP_CLUSTER_NODEPOOL_MACHINETYPE)" \
		-var zone=$(GCP_CLUSTER_ZONE) -var project=$(GCP_PROJECT) \
		-var log_level=$(LOG_LEVEL) \
		-var node_count=$(GCP_CLUSTER_NODEPOOL_INITIALNODECOUNT)'
	GCP_CLUSTER_NAME=$(GCP_TF_CLUSTER_NAME) $(MAKE) gcloud-auth-cluster

gcloud-terraform-destroy-cluster:
	$(DOCKER_RUN) bash -c 'cd $(mount_path)/build/terraform/gke && \
	 terraform destroy -target module.helm_agones.helm_release.agones -auto-approve && sleep 60 && terraform destroy -auto-approve'
