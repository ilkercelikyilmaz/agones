---
title: "Allocator Service"
date: 2019-10-25T05:45:05Z
version: "v1alpha1"
description: >
  Agones provides an mTLS based allocator service that is accessible from outside the cluster using a load balancer. The service is deployed and scales independent to Agones controller.
---

{{< alert title="Alpha" color="warning">}}
This feature is in a pre-release state and might change.
{{< /alert >}}

To allocate a game server, Agones in addition to {{< ghlink href="pkg/apis/allocation/v1/gameserverallocation.go" >}}GameServerAllocations{{< /ghlink >}}, provides a gRPC service with mTLS authentication, called agones-allocator, which is on {{< ghlink href="proto/allocation/v1alpha1" >}}v1alpha1 version{{< /ghlink >}}, starting on agones v1.1.

The gRPC service is accessible through a Kubernetes service that is externalized using a load balancer. For the gRPC request to succeed, a client certificate must be provided that is in the authorization list of the allocator service.

The remainder of this article describes how to manually make a successful allocation request using the gRPC API.

## Find the external IP

The service is hosted under the same namespace as the Agones controller. To find the external IP of your allocator service, replace agones-system namespace with the namespace to which Agones is deployed and execute the following command:

```bash
kubectl get service agones-allocator -n agones-system
```

The output of the command should look like:

<pre>
NAME                        TYPE           CLUSTER-IP      <b>EXTERNAL-IP</b>     PORT(S)            AGE
agones-allocator            LoadBalancer   10.55.251.73    <b>34.82.195.204</b>   443:30250/TCP      7d22h
</pre>

## Server TLS certificate

Replace the default server TLS certificate with a certificate with CN and subjectAltName. There are multiple approaches to generate a certificate, including using CA. The following provides an example of generating a self-signed certificate using openssl and storing it in allocator-tls Kubernetes secret.

```bash
#!/bin/bash
EXTERNAL_IP=`kubectl get services agones-allocator -n agones-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`

TLS_KEY_FILE=tls.key
TLS_CERT_FILE=tls.crt

cat /etc/ssl/openssl.cnf <(printf "\n[SAN]\nsubjectAltName=IP:${EXTERNAL_IP}") > openssl.cnf

openssl req -nodes -new -newkey rsa:2048 \
    -keyout ${TLS_KEY_FILE} \
    -out tls.csr \
    -subj "/CN=${EXTERNAL_IP}/O=${EXTERNAL_IP}" \
    -reqexts SAN \
    -config openssl.cnf

openssl x509 -req -days 365 -in tls.csr \
    -signkey ${TLS_KEY_FILE} \
    -out ${TLS_CERT_FILE} \
    -extensions SAN \
    -extfile openssl.cnf

# After having the TLS certificates ready, run the following command to store the certificate as a Kubernetes TLS secret.
kubectl create secret --save-config=true tls allocator-tls -n agones-system --key=${TLS_KEY_FILE} --cert=${TLS_CERT_FILE} --dry-run -o yaml | kubectl apply -f -

# Optional: Add the TLS signing CA to allocator-tls-ca
TLS_CERT_FILE_VALUE=`cat ${TLS_CERT_FILE} | base64 -w 0`
kubectl get secret allocator-tls-ca -o json -n agones-system | jq '.data["tls-ca.crt"]="'${TLS_CERT_FILE_VALUE}'"' | kubectl apply -f -
```

## Client Certificate

Because agones-allocator uses an mTLS authentication mechanism, client must provide a certificate that is accepted by the server. Here is an example of generating a client certificate. For the agones-allocator service to accept the newly generate client certificate, the generated client certificate CA or public portion of the certificate must be added to a kubernetes secret called `allocator-client-ca`.

```bash
#!/bin/bash

KEY_FILE=client.key
CERT_FILE=client.crt

openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout ${KEY_FILE} -out ${CERT_FILE}

CERT_FILE_VALUE=`cat ${CERT_FILE} | base64 -w 0`

# In case of MacOS
# CERT_FILE_VALUE=`cat ${CERT_FILE} | base64`

# white-list client certificate
kubectl get secret allocator-client-ca -o json -n agones-system | jq '.data["client_trial.crt"]="'${CERT_FILE_VALUE}'"' | kubectl apply -f -
```

The last command creates a new entry in the secret data map called `client_trial.crt` for `allocator-client-ca` and stores it. You can also achieve this by `kubectl edit secret allocator-client-ca -n agones-system`, and then add the entry.

## Restart pods

Restart pods to get the new TLS certificate loaded to the agones-allocator service.

```bash
kubectl get pods -n agones-system -o=name | grep agones-allocator | xargs kubectl delete -n agones-system
```

## Send allocation request

Now the service is ready to accept requests from the client with the generated certificates. Create a [fleet](https://agones.dev/site/docs/getting-started/create-fleet/#1-create-a-fleet) and send a gRPC request to agones-allocator by providing the namespace to which the fleet is deployed. You can find the gRPC sample for sending allocation request at {{< ghlink href="examples/allocator-client/main.go" >}}allocator-client sample{{< /ghlink >}}.

```bash
#!/bin/bash

NAMESPACE=default # replace with any namespace

go run examples/allocator-client/main.go --ip ${EXTERNAL_IP} \
    --namespace ${NAMESPACE} \
    --key ${KEY_FILE} \
    --cert ${CERT_FILE} \
    --cacert ${TLS_CERT_FILE}
```

If your matchmaker is external to the cluster on which your game servers are hosted, the `agones-allocator` provides the gRPC API to allocate game services using mTLS authentication, which can scale independently to the Agones controller.