#!/bin/bash

NAMESPACE=default
EXTERNAL_IP=<IP_ADRESSS_TO_THE_ALLOCATOR_SERVICES_LOAD_BALANCER>
KEY_FILE=client.key
CERT_FILE=client.crt
TLS_CA_FILE=ca.crt

go run ./main.go --ip ${EXTERNAL_IP} --port 443 --namespace ${NAMESPACE} --key ${KEY_FILE} --cert ${CERT_FILE} --cacert ${TLS_CA_FILE} --numberofclients $1 --perclientallocations $2