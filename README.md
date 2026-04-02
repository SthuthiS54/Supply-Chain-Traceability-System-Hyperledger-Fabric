cat > ~/fabric-samples/supplychain-network/README.md << 'EOF'
# Supply Chain Tracking System - Hyperledger Fabric

A blockchain-based supply chain tracking application built on Hyperledger Fabric v2.5.15 with three organizations: Manufacturer, Distributor, and Retailer.

## Network Architecture
```
supplychainchannel
├── peer0.manufacturer.example.com  (port 7051)  — ManufacturerMSP
├── peer0.distributor.example.com   (port 9051)  — DistributorMSP
└── peer0.retailer.example.com      (port 11051) — RetailerMSP

Orderer: orderer.example.com (port 7050)
State DB: CouchDB (couchdb0, couchdb1, couchdb2)
```

## Prerequisites

- Docker Engine 20.x+
- Docker Compose v2.x
- Go 1.19+
- Hyperledger Fabric binaries v2.5.x
- jq

### Install Fabric Binaries
```bash
curl -sSL https://bit.ly/2ysbOFE | bash -s -- 2.5.15 1.5.7
export PATH=$PATH:$HOME/fabric-samples/bin
export FABRIC_CFG_PATH=$HOME/fabric-samples/config
```

### Fix Docker Compose (Ubuntu 24)

If you get a `ContainerConfig` error, run:
```bash
sudo update-alternatives --set iptables /usr/sbin/iptables-legacy
sudo update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy
sudo systemctl start docker
```

Force Docker Compose v2 in network.sh (already applied in this repo):
```bash
sed -i '32s/.*/if false; then/' network.sh
```

## Setup

### 1. Clone the repository
```bash
git clone git@github.com:SthuthiS54/Supply-Chain-Network.git
cd Supply-Chain-Network
```

### 2. Add to PATH
```bash
export PATH=$PATH:$HOME/fabric-samples/bin
export FABRIC_CFG_PATH=$HOME/fabric-samples/config
```

### 3. Start the network and create channel
```bash
./network.sh up createChannel -c supplychainchannel -s couchdb
```

This will:
- Generate crypto material for Manufacturer, Distributor, and Retailer
- Start 7 Docker containers (3 peers + orderer + 3 CouchDB instances)
- Create the `supplychainchannel` channel
- Join all 3 peers to the channel
- Set anchor peers for all 3 organizations

### 4. Verify peers joined the channel
```bash
# Set Manufacturer env
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="ManufacturerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/manufacturer.example.com/users/Admin@manufacturer.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

peer channel list
```

## Deploy Chaincode

### 1. Package
```bash
peer lifecycle chaincode package supplychain.tar.gz \
  --path ./chaincode/supplychain/go/ \
  --lang golang \
  --label supplychain_1.0
```

### 2. Install on all peers
```bash
# Manufacturer
export CORE_PEER_LOCALMSPID="ManufacturerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/manufacturer.example.com/users/Admin@manufacturer.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051
peer lifecycle chaincode install supplychain.tar.gz

# Distributor
export CORE_PEER_LOCALMSPID="DistributorMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/distributor.example.com/peers/peer0.distributor.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/distributor.example.com/users/Admin@distributor.example.com/msp
export CORE_PEER_ADDRESS=localhost:9051
peer lifecycle chaincode install supplychain.tar.gz

# Retailer
export CORE_PEER_LOCALMSPID="RetailerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/retailer.example.com/peers/peer0.retailer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/retailer.example.com/users/Admin@retailer.example.com/msp
export CORE_PEER_ADDRESS=localhost:11051
peer lifecycle chaincode install supplychain.tar.gz
```

### 3. Get Package ID
```bash
peer lifecycle chaincode queryinstalled
export CC_PACKAGE_ID=supplychain_1.0:<hash>  # replace with actual hash
```

### 4. Approve for all orgs
```bash
# Run approveformyorg for each org (switch env vars as above)
peer lifecycle chaincode approveformyorg \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --channelID supplychainchannel \
  --name supplychain \
  --version 1.0 \
  --package-id $CC_PACKAGE_ID \
  --sequence 1 \
  --tls \
  --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem
```

### 5. Commit
```bash
peer lifecycle chaincode commit \
  -o localhost:7050 \
  --ordererTLSHostnameOverride orderer.example.com \
  --channelID supplychainchannel \
  --name supplychain \
  --version 1.0 \
  --sequence 1 \
  --tls \
  --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/distributor.example.com/peers/peer0.distributor.example.com/tls/ca.crt \
  --peerAddresses localhost:11051 \
  --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/retailer.example.com/peers/peer0.retailer.example.com/tls/ca.crt
```

## Run Transactions

### Manufacturer creates product batch
```bash
export CORE_PEER_LOCALMSPID="ManufacturerMSP"
export CORE_PEER_ADDRESS=localhost:7051
# (set TLS env vars for Manufacturer)

peer chaincode invoke \
  -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem \
  -C supplychainchannel -n supplychain \
  --peerAddresses localhost:7051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt \
  --peerAddresses localhost:9051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/distributor.example.com/peers/peer0.distributor.example.com/tls/ca.crt \
  --peerAddresses localhost:11051 --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/retailer.example.com/peers/peer0.retailer.example.com/tls/ca.crt \
  -c '{"function":"CreateProductBatch","Args":["BATCH001","Laptop","100","Mumbai"]}'
```

### Distributor updates shipment status
```bash
# Switch to Distributor env vars
peer chaincode invoke ... \
  -c '{"function":"UpdateShipmentStatus","Args":["BATCH001","Delhi"]}'
```

### Retailer confirms delivery
```bash
# Switch to Retailer env vars
peer chaincode invoke ... \
  -c '{"function":"ConfirmDelivery","Args":["BATCH001","Bangalore"]}'
```

### Query current state
```bash
peer chaincode query -C supplychainchannel -n supplychain \
  -c '{"function":"QueryBatch","Args":["BATCH001"]}'
```

### Query full product history
```bash
peer chaincode query -C supplychainchannel -n supplychain \
  -c '{"function":"GetProductHistory","Args":["BATCH001"]}'
```

## Expected Output

**QueryBatch:**
```json
{
  "batchID": "BATCH001",
  "productName": "Laptop",
  "quantity": 100,
  "status": "DELIVERED",
  "currentHolder": "Retailer",
  "location": "Bangalore",
  "timestamp": "2026-04-02T03:48:20Z",
  "transactionID": "ab794ce7..."
}
```

**GetProductHistory:**
```json
[
  {"txID":"ab794ce7...","timestamp":"2026-04-02T03:48:20Z","record":{"status":"DELIVERED","currentHolder":"Retailer","location":"Bangalore"}},
  {"txID":"0c3220a7...","timestamp":"2026-04-02T03:46:52Z","record":{"status":"IN_TRANSIT","currentHolder":"Distributor","location":"Delhi"}},
  {"txID":"c07852fb...","timestamp":"2026-04-02T03:44:50Z","record":{"status":"CREATED","currentHolder":"Manufacturer","location":"Mumbai"}}
]
```

## Chaincode Functions

| Function | Caller | Description |
|---|---|---|
| `CreateProductBatch` | Manufacturer | Creates a new product batch on the ledger |
| `UpdateShipmentStatus` | Distributor | Updates batch status to IN_TRANSIT |
| `ConfirmDelivery` | Retailer | Updates batch status to DELIVERED |
| `QueryBatch` | Any | Returns current state of a batch |
| `GetProductHistory` | Any | Returns full lifecycle using GetHistoryForKey API |

## Tear Down
```bash
./network.sh down
```

## Project Structure
```
supplychain-network/
├── chaincode/supplychain/go/
│   ├── supplychain.go        # Smart contract
│   └── go.mod
├── compose/
│   ├── compose-test-net.yaml # Docker services for all 3 orgs
│   └── compose-couch.yaml    # CouchDB for all 3 orgs
├── configtx/
│   └── configtx.yaml         # Channel and org configuration
├── organizations/
│   └── cryptogen/            # Crypto configs for all 3 orgs
├── scripts/
│   ├── createChannel.sh      # Channel creation script
│   ├── envVar.sh             # Peer environment variables
│   └── setAnchorPeer.sh      # Anchor peer configuration
├── network.sh                # Main network management script
└── README.md
```

## Team Members

- Siripurapu Abhinaya (221CS102)
- Vineela Sivvala (221CS155)
- Sthuthi S (221CS156)
- Varahi Suvarna (221CS259)
