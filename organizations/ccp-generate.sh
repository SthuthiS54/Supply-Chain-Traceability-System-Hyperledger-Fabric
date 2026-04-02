#!/usr/bin/env bash

function one_line_pem {
    echo "`awk 'NF {sub(/\\n/, ""); printf "%s\\\\\\\n",$0;}' $1`"
}

function json_ccp {
    local PP=$(one_line_pem $4)
    local CP=$(one_line_pem $5)
    sed -e "s/\${ORG}/$1/" \
        -e "s/\${P0PORT}/$2/" \
        -e "s/\${CAPORT}/$3/" \
        -e "s#\${PEERPEM}#$PP#" \
        -e "s#\${CAPEM}#$CP#" \
        organizations/ccp-template.json
}

function yaml_ccp {
    local PP=$(one_line_pem $4)
    local CP=$(one_line_pem $5)
    sed -e "s/\${ORG}/$1/" \
        -e "s/\${P0PORT}/$2/" \
        -e "s/\${CAPORT}/$3/" \
        -e "s#\${PEERPEM}#$PP#" \
        -e "s#\${CAPEM}#$CP#" \
        organizations/ccp-template.yaml | sed -e $'s/\\\\n/\\\n          /g'
}

ORG=manufacturer
P0PORT=7051
CAPORT=7054
PEERPEM=organizations/peerOrganizations/manufacturer.example.com/tlsca/tlsca.manufacturer.example.com-cert.pem
CAPEM=organizations/peerOrganizations/manufacturer.example.com/ca/ca.manufacturer.example.com-cert.pem
echo "$(json_ccp $ORG $P0PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/manufacturer.example.com/connection-manufacturer.json
echo "$(yaml_ccp $ORG $P0PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/manufacturer.example.com/connection-manufacturer.yaml

ORG=distributor
P0PORT=9051
CAPORT=8054
PEERPEM=organizations/peerOrganizations/distributor.example.com/tlsca/tlsca.distributor.example.com-cert.pem
CAPEM=organizations/peerOrganizations/distributor.example.com/ca/ca.distributor.example.com-cert.pem
echo "$(json_ccp $ORG $P0PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/distributor.example.com/connection-distributor.json
echo "$(yaml_ccp $ORG $P0PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/distributor.example.com/connection-distributor.yaml

ORG=retailer
P0PORT=11051
CAPORT=9054
PEERPEM=organizations/peerOrganizations/retailer.example.com/tlsca/tlsca.retailer.example.com-cert.pem
CAPEM=organizations/peerOrganizations/retailer.example.com/ca/ca.retailer.example.com-cert.pem
echo "$(json_ccp $ORG $P0PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/retailer.example.com/connection-retailer.json
echo "$(yaml_ccp $ORG $P0PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/retailer.example.com/connection-retailer.yaml
