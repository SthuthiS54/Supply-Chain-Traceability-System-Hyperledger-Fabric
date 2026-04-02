#!/usr/bin/env bash
#
# Copyright IBM Corp All Rights Reserved
#
# SPDX-License-Identifier: Apache-2.0
#

TEST_NETWORK_HOME=${TEST_NETWORK_HOME:-${PWD}}
. ${TEST_NETWORK_HOME}/scripts/utils.sh

export CORE_PEER_TLS_ENABLED=true
export ORDERER_CA=${TEST_NETWORK_HOME}/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
export PEER0_MANUFACTURER_CA=${TEST_NETWORK_HOME}/organizations/peerOrganizations/manufacturer.example.com/tlsca/tlsca.manufacturer.example.com-cert.pem
export PEER0_DISTRIBUTOR_CA=${TEST_NETWORK_HOME}/organizations/peerOrganizations/distributor.example.com/tlsca/tlsca.distributor.example.com-cert.pem
export PEER0_RETAILER_CA=${TEST_NETWORK_HOME}/organizations/peerOrganizations/retailer.example.com/tlsca/tlsca.retailer.example.com-cert.pem

setGlobals() {
  local USING_ORG=""
  if [ -z "$OVERRIDE_ORG" ]; then
    USING_ORG=$1
  else
    USING_ORG="${OVERRIDE_ORG}"
  fi
  infoln "Using organization ${USING_ORG}"
  if [ $USING_ORG -eq 1 ]; then
    export CORE_PEER_LOCALMSPID=ManufacturerMSP
    export CORE_PEER_TLS_ROOTCERT_FILE=$PEER0_MANUFACTURER_CA
    export CORE_PEER_MSPCONFIGPATH=${TEST_NETWORK_HOME}/organizations/peerOrganizations/manufacturer.example.com/users/Admin@manufacturer.example.com/msp
    export CORE_PEER_ADDRESS=localhost:7051
  elif [ $USING_ORG -eq 2 ]; then
    export CORE_PEER_LOCALMSPID=DistributorMSP
    export CORE_PEER_TLS_ROOTCERT_FILE=$PEER0_DISTRIBUTOR_CA
    export CORE_PEER_MSPCONFIGPATH=${TEST_NETWORK_HOME}/organizations/peerOrganizations/distributor.example.com/users/Admin@distributor.example.com/msp
    export CORE_PEER_ADDRESS=localhost:9051
  elif [ $USING_ORG -eq 3 ]; then
    export CORE_PEER_LOCALMSPID=RetailerMSP
    export CORE_PEER_TLS_ROOTCERT_FILE=$PEER0_RETAILER_CA
    export CORE_PEER_MSPCONFIGPATH=${TEST_NETWORK_HOME}/organizations/peerOrganizations/retailer.example.com/users/Admin@retailer.example.com/msp
    export CORE_PEER_ADDRESS=localhost:11051
  else
    errorln "ORG Unknown"
  fi
  if [ "$VERBOSE" = "true" ]; then
    env | grep CORE
  fi
}

parsePeerConnectionParameters() {
  PEER_CONN_PARMS=()
  PEERS=""
  while [ "$#" -gt 0 ]; do
    setGlobals $1
    if [ $1 -eq 1 ]; then PEER="peer0.manufacturer"
    elif [ $1 -eq 2 ]; then PEER="peer0.distributor"
    elif [ $1 -eq 3 ]; then PEER="peer0.retailer"
    fi
    if [ -z "$PEERS" ]; then
      PEERS="$PEER"
    else
      PEERS="$PEERS $PEER"
    fi
    PEER_CONN_PARMS=("${PEER_CONN_PARMS[@]}" --peerAddresses $CORE_PEER_ADDRESS)
    if [ $1 -eq 1 ]; then CA=$PEER0_MANUFACTURER_CA
    elif [ $1 -eq 2 ]; then CA=$PEER0_DISTRIBUTOR_CA
    elif [ $1 -eq 3 ]; then CA=$PEER0_RETAILER_CA
    fi
    TLSINFO=(--tlsRootCertFiles "${CA}")
    PEER_CONN_PARMS=("${PEER_CONN_PARMS[@]}" "${TLSINFO[@]}")
    shift
  done
}

verifyResult() {
  if [ $1 -ne 0 ]; then
    fatalln "$2"
  fi
}
