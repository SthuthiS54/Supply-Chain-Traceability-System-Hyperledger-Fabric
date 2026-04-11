# Supply Chain Traceability System
### Built on Hyperledger Fabric v2.5

A **production-grade blockchain supply chain application** built on Hyperledger Fabric v2.5 featuring three-organization role-based access control, complete product lifecycle tracking via the `GetHistoryForKey` API, rich CouchDB queries, composite key quality records, paginated queries, and real-time event emission.

---

## Group Members

| Name | Roll Number |
|------|-------------|
| Abhinaya Siripurapu | 221CS102 |
| Sivvala Vineela | 221CS155 |
| Sthuthi S | 221CS156 |
| Varahi Suvarna | 221CS259 |

---

## Table of Contents

- [Problem Statement](#problem-statement)
- [Architecture Overview](#architecture-overview)
- [Key Features](#key-features)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  - [Step 1 - Start the Network](#step-1---start-the-network)
  - [Step 2 - Deploy the Chaincode](#step-2---deploy-the-chaincode)
  - [Step 3 - Run Transactions](#step-3---run-transactions)
  - [Step 4 - Query the Ledger](#step-4---query-the-ledger)
- [Chaincode Functions Reference](#chaincode-functions-reference)
- [Data Models](#data-models)
- [Role-Based Access Control](#role-based-access-control)
- [GetHistoryForKey - Product Lifecycle Tracking](#gethistoryforkey---product-lifecycle-tracking)
- [CouchDB Rich Queries](#couchdb-rich-queries)
- [Teardown](#teardown)
- [Sample Output](#sample-output)
- [Technology Stack](#technology-stack)

---

## Problem Statement

Traditional supply chains suffer from opacity - once a product leaves a manufacturer, there is no tamper-proof record of where it has been. This project solves that by:

1. **Manufacturer** creates a product batch and registers it immutably on a shared ledger.
2. **Distributor** picks it up and updates its shipment status in transit.
3. **Retailer** confirms delivery at the final destination.
4. **Any participant** can query the complete, timestamped lifecycle of any batch using the `GetHistoryForKey` API - a blockchain-native audit trail that cannot be altered.

---

## Architecture Overview

```
┌───────────────────────────────────────────────────────────┐
│                   supplychainchannel                      │
│                                                           │
│  ┌────────────────┐  ┌───────────────┐  ┌──────────────┐  │
│  │  Manufacturer  │  │  Distributor  │  │   Retailer   │  │
│  │  peer0:7051    │  │  peer0:9051   │  │  peer0:11051 │  │
│  │ ManufacturerMSP│  │ DistributorMSP│  │  RetailerMSP │  │
│  │  couchdb0      │  │  couchdb1     │  │  couchdb2    │  │
│  └────────────────┘  └───────────────┘  └──────────────┘  │
│                                                           │
│              orderer.example.com:7050                     │
└───────────────────────────────────────────────────────────┘
```

**Channel:** `supplychainchannel`  
**Consensus:** Raft (single orderer)  
**State DB:** CouchDB (enables rich JSON queries)  
**Chaincode Language:** Go (`fabric-contract-api-go`)

---

## Key Features

| Feature | Description |
|---|---|
| **Role-Based Access Control** | MSP-level enforcement - only the correct org can call restricted functions |
| **Full Lifecycle History** | `GetHistoryForKey` returns every state change with timestamp and transaction ID |
| **Rich CouchDB Queries** | Filter by status, holder, location, date range; paginated results |
| **Composite Key QC Records** | Multiple quality checks per batch, each stored independently |
| **Blockchain Events** | `QuantityUpdateEvent` and `TransferEvent` emitted for external listeners |
| **Batch Recall** | Manufacturer can recall a batch with a reason; fully auditable |
| **Tombstone History** | Deleted batches still show history + tombstone (proves immutability) |
| **Integrity Verification** | `VerifyBatchIntegrity` compares world state against latest history entry |
| **Ledger Statistics** | `GetLedgerStats` gives a real-time count of batches by status |
| **Pagination** | `GetBatchesWithPagination` supports cursor-based paging for large datasets |

---

## Project Structure

```
supplychain-network/
├── chaincode/
│   └── supplychain/
│       └── go/
│           ├── supplychain.go              # Main chaincode
│           ├── go.mod
│           └── META-INF/
│               └── statedb/couchdb/indexes/
│                   ├── timestamp_index.json
│                   ├── location_index.json
│                   └── productname_index.json
├── compose/
│   ├── compose-test-net.yaml              # Main Docker Compose (peers, orderer)
│   └── compose-couch.yaml                 # CouchDB sidecar containers
├── configtx/
│   └── configtx.yaml                      # Channel & org configuration
├── organizations/cryptogen/
│   ├── crypto-config-manufacturer.yaml
│   ├── crypto-config-distributor.yaml
│   ├── crypto-config-retailer.yaml
│   └── crypto-config-orderer.yaml
├── scripts/
│   ├── createChannel.sh
│   ├── envVar.sh
│   └── setAnchorPeer.sh
└── network.sh                             # network management script
```

---

## Prerequisites

| Requirement | Version | Install |
|---|---|---|
| Docker Engine | 20.x+ | [docs.docker.com](https://docs.docker.com/engine/install/) |
| Docker Compose | v2.x | Included with Docker Desktop |
| Go | 1.21+ | [go.dev/dl](https://go.dev/dl/) |
| Hyperledger Fabric Binaries | 2.5.x | See below |
| `jq` | any | `sudo apt install jq` |

### Install Hyperledger Fabric Binaries

```bash
curl -sSL https://bit.ly/2ysbOFE | bash -s -- 2.5.15 1.5.7
export PATH=$PATH:$HOME/fabric-samples/bin
export FABRIC_CFG_PATH=$HOME/fabric-samples/config
```

### Fix Docker on Ubuntu 24.04 (if needed)

```bash
sudo update-alternatives --set iptables /usr/sbin/iptables-legacy
sudo update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy
sudo systemctl start docker
sudo usermod -aG docker $USER && newgrp docker
```

---

## Quick Start

### Step 1 - Start the Network

Navigate to the network directory and clean any previous state:

```bash
cd ~/fabric-samples/supplychain-network

# Clean environment
./network.sh down
docker volume rm $(docker volume ls -q) 2>/dev/null || true
docker network prune -f
rm -rf organizations/peerOrganizations organizations/ordererOrganizations channel-artifacts/
```

Start the network with CouchDB and create the channel:

```bash
./network.sh up createChannel -c supplychainchannel -s couchdb
```

Verify all **7 containers** are running:

```bash
docker ps --format "table {{.Names}}\t{{.Status}}"
```

Expected containers: `peer0.manufacturer`, `peer0.distributor`, `peer0.retailer`, `orderer`, `couchdb0`, `couchdb1`, `couchdb2`.

Verify peers joined the channel:

```bash
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="ManufacturerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/manufacturer.example.com/users/Admin@manufacturer.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

peer channel list
peer channel getinfo -c supplychainchannel
```

---

### Step 2 - Deploy the Chaincode

#### Package

```bash
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="ManufacturerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/manufacturer.example.com/users/Admin@manufacturer.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

peer lifecycle chaincode package supplychain.tar.gz \
  --path ./chaincode/supplychain/go/ \
  --lang golang \
  --label supplychain_1.0
```

#### Install on All Three Peers

<details>
<summary><b>Manufacturer (port 7051)</b></summary>

```bash
peer lifecycle chaincode install supplychain.tar.gz
```
</details>

<details>
<summary><b>Distributor (port 9051)</b></summary>

```bash
export CORE_PEER_LOCALMSPID="DistributorMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/distributor.example.com/peers/peer0.distributor.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/distributor.example.com/users/Admin@distributor.example.com/msp
export CORE_PEER_ADDRESS=localhost:9051

peer lifecycle chaincode install supplychain.tar.gz
```
</details>

<details>
<summary><b>Retailer (port 11051)</b></summary>

```bash
export CORE_PEER_LOCALMSPID="RetailerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/retailer.example.com/peers/peer0.retailer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/retailer.example.com/users/Admin@retailer.example.com/msp
export CORE_PEER_ADDRESS=localhost:11051

peer lifecycle chaincode install supplychain.tar.gz
```
</details>

#### Get Package ID and Approve for All Orgs

```bash
# Switch back to Manufacturer
export CORE_PEER_LOCALMSPID="ManufacturerMSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/manufacturer.example.com/users/Admin@manufacturer.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

peer lifecycle chaincode queryinstalled
# Copy the Package ID from output, then:
export CC_PACKAGE_ID=supplychain_1.0:abc123...
```

Approve for each org (repeat after switching env vars for Distributor and Retailer):

```bash
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

#### Check Readiness and Commit

```bash
peer lifecycle chaincode checkcommitreadiness \
  --channelID supplychainchannel \
  --name supplychain \
  --version 1.0 \
  --sequence 1 \
  --tls \
  --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem \
  --output json
# All three orgs should show: "true"
```

```bash
# Switch back to Manufacturer env, then commit:
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

---

### Step 3 - Run Transactions

The core supply chain flow follows three mandatory stages in order:

```
CREATED => IN_TRANSIT => DELIVERED
(Manufacturer)  (Distributor)   (Retailer)
```

#### 1. CreateProductBatch - Manufacturer Only

```bash
# Set Manufacturer env first (see Step 1 export block)
peer chaincode invoke \
  -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile ${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem \
  -C supplychainchannel -n supplychain \
  --peerAddresses localhost:7051 \
  --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/manufacturer.example.com/peers/peer0.manufacturer.example.com/tls/ca.crt \
  --peerAddresses localhost:9051 \
  --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/distributor.example.com/peers/peer0.distributor.example.com/tls/ca.crt \
  --peerAddresses localhost:11051 \
  --tlsRootCertFiles ${PWD}/organizations/peerOrganizations/retailer.example.com/peers/peer0.retailer.example.com/tls/ca.crt \
  -c '{"function":"CreateProductBatch","Args":["BATCH001","Paracetamol","1000","Mumbai"]}'
```

#### 2. UpdateShipmentStatus - Distributor Only

```bash
# Set Distributor env, then:
-c '{"function":"UpdateShipmentStatus","Args":["BATCH001","Delhi"]}'
```

#### 3. ConfirmDelivery - Retailer Only

```bash
# Set Retailer env, then:
-c '{"function":"ConfirmDelivery","Args":["BATCH001","Bangalore"]}'
```

#### 4. UpdateBatchQuantity - Any Org

```bash
-c '{"function":"UpdateBatchQuantity","Args":["BATCH001","950","Damaged goods removed"]}'
```

#### 5. AddQualityCheck - Any Org

```bash
-c '{"function":"AddQualityCheck","Args":["BATCH001","PASS","All parameters within range"]}'
```

#### 6. RecallBatch - Manufacturer Only

```bash
-c '{"function":"RecallBatch","Args":["BATCH002","Contamination detected"]}'
```

#### 7. DeleteBatch - Any Org

```bash
-c '{"function":"DeleteBatch","Args":["BATCH002"]}'
# Note: History + tombstone record remains on-chain even after deletion
```

---

### Step 4 - Query the Ledger

All query commands use `peer chaincode query`:

```bash
peer chaincode query -C supplychainchannel -n supplychain -c '<JSON>' | jq .
```

| # | Function | Args | Description |
|---|---|---|---|
| 8 | `QueryBatch` | `["BATCH001"]` | Current state of a batch |
| 9 | `GetProductHistory` | `["BATCH001"]` | Full lifecycle via GetHistoryForKey |
| 10 | `GetAllBatches` | `[]` | All batches on ledger |
| 11 | `QueryBatchesByStatus` | `["DELIVERED"]` | Filter by status |
| 12 | `QueryBatchesByHolder` | `["Retailer"]` | Filter by current holder |
| 13 | `GetQualityChecks` | `["BATCH001"]` | All QC records for a batch |
| 14 | `BatchExists` | `["BATCH001"]` | Existence check (true/false) |
| 15 | `VerifyBatchIntegrity` | `["BATCH001"]` | World state vs history check |
| 16 | `GetBatchesByDateRange` | `["2024-01-01T00:00:00Z","2026-12-31T23:59:59Z"]` | Date range filter |
| 17 | `GetLedgerStats` | `[]` | Count by status |
| 18 | `GetBatchesWithPagination` | `["DELIVERED","3",""]` | Paginated results |
| 19 | `GetProductHistory` | `["BATCH002"]` | History for deleted batch (tombstone) |

---

## Chaincode Functions Reference

### Write Functions (invoke)

| Function | Permitted Caller | Workflow Constraint |
|---|---|---|
| `CreateProductBatch` | ManufacturerMSP only | Batch must not already exist |
| `UpdateShipmentStatus` | DistributorMSP only | Batch must be in `CREATED` state |
| `ConfirmDelivery` | RetailerMSP only | Batch must be in `IN_TRANSIT` state |
| `RecallBatch` | ManufacturerMSP only | Batch must not already be `RECALLED` |
| `UpdateBatchQuantity` | Any org | Not allowed if `DELIVERED` or `RECALLED` |
| `AddQualityCheck` | Any org | Batch must exist; result must be `PASS` or `FAIL` |
| `TransferOwnership` | Any org | Not allowed if `DELIVERED` or `RECALLED` |
| `DeleteBatch` | Any org | Batch must exist |

### Read Functions (query)

| Function | Backend | Notes |
|---|---|---|
| `QueryBatch` | LevelDB/CouchDB world state | Point lookup |
| `GetProductHistory` | Blockchain history | Uses `GetHistoryForKey` |
| `BatchExists` | World state | Returns bool |
| `GetQualityChecks` | Composite key range scan | All QC records for a batch |
| `VerifyBatchIntegrity` | World state + history | Integrity audit |
| `GetAllBatches` | CouchDB rich query | Excludes composite key entries |
| `QueryBatchesByStatus` | CouchDB rich query | Supported: `CREATED`, `IN_TRANSIT`, `DELIVERED`, `RECALLED`, `TRANSFERRED` |
| `QueryBatchesByHolder` | CouchDB rich query | Supported: `Manufacturer`, `Distributor`, `Retailer` |
| `GetBatchesByLocation` | CouchDB rich query | Exact location match |
| `GetBatchesByDateRange` | CouchDB rich query | RFC3339 timestamps, sorted ascending |
| `GetLedgerStats` | CouchDB rich query | Aggregated count by status |
| `GetBatchesWithPagination` | CouchDB paginated query | Pass bookmark from previous page |

---

## Data Models

### ProductBatch

```json
{
  "batchID":       "BATCH001",
  "productName":   "Paracetamol",
  "quantity":      1000,
  "status":        "DELIVERED",
  "currentHolder": "Retailer",
  "location":      "Bangalore",
  "timestamp":     "2025-04-10T14:32:05Z",
  "transactionID": "abc123def456..."
}
```

### HistoryRecord

```json
{
  "txID":      "abc123def456...",
  "timestamp": "2025-04-10T14:32:05Z",
  "isDelete":  false,
  "record":    { "batchID": "BATCH001", "status": "DELIVERED" }
}
```

### QualityCheck

```json
{
  "batchID":   "BATCH001",
  "checkedBy": "ManufacturerMSP",
  "result":    "PASS",
  "remarks":   "All parameters within range",
  "timestamp": "2025-04-10T12:00:00Z",
  "txID":      "xyz789..."
}
```

### LedgerStats

```json
{
  "total": 5,
  "created": 1,
  "inTransit": 1,
  "delivered": 2,
  "recalled": 1,
  "transferred": 0
}
```
### PaginatedResult
 
Returned by `GetBatchesWithPagination`. Pass the `bookmark` value from one response as the third argument in the next call to retrieve the following page.
 
```json
{
  "records": [
    {
      "batchID": "BATCH..",
      "productName": "Abcd...",
      "quantity": 950,
      "status": "DELIVERED",
      "currentHolder": "Retailer",
      "location": "Bangalore",
      "timestamp": "2026-04-11T09:11:38Z",
      "transactionID": "ea54d1aeb6..."
    },
    {
      ...
    },
    {
      ...
    }
  ],
  "fetchedRecordsCount": 3,
  "bookmark": "g1AAAAB..."
}
```


---

## Role-Based Access Control

Access control is enforced at the MSP level inside the chaincode using `GetClientIdentity().GetMSPID()`. This means it is **enforced by the blockchain itself**, not by any application layer.

```
CreateProductBatch    => ManufacturerMSP ONLY
UpdateShipmentStatus  => DistributorMSP  ONLY
ConfirmDelivery       => RetailerMSP     ONLY
RecallBatch           => ManufacturerMSP ONLY
```

Any attempt by the wrong organization to call a restricted function returns:

```
Error: access denied: this function requires ManufacturerMSP but was called by DistributorMSP
```

---

## GetHistoryForKey - Product Lifecycle Tracking

The `GetProductHistory` function uses the Fabric `GetHistoryForKey` API to retrieve every version of a batch ever written to the blockchain. This is the core audit trail capability of the system.

Key properties:

- History is **immutable** - even after `DeleteBatch`, all previous states plus a tombstone entry remain visible
- Timestamps come from the **ordering service clock**, not client clocks, so they are trustworthy
- Each entry carries the **transaction ID** for cross-referencing with block explorer tools

---

## CouchDB Rich Queries

CouchDB is used as the state database, enabling MongoDB-style JSON selector queries. Custom indexes are defined in `META-INF/statedb/couchdb/indexes/` for performance.

Query all delivered batches:

```bash
peer chaincode query -C supplychainchannel -n supplychain \
  -c '{"function":"QueryBatchesByStatus","Args":["DELIVERED"]}' | jq .
```

Paginated query - page 1, 3 records per page:

```bash
peer chaincode query -C supplychainchannel -n supplychain \
  -c '{"function":"GetBatchesWithPagination","Args":["DELIVERED","3",""]}' | jq .
# Pass the "bookmark" value from the response as the third arg to get page 2
```

---

## Teardown

```bash
./network.sh down
```

This stops all containers, removes the channel artifacts, and cleans up crypto material.

---

## Sample Output

CreateProductBatch:

<img width="1678" height="140" alt="image" src="https://github.com/user-attachments/assets/a213f0a4-5eef-49c4-b490-b49312341928" />


GetProductHistory (BATCH001):

<img width="1722" height="794" alt="image" src="https://github.com/user-attachments/assets/3842dfd7-c540-4770-929b-29c3f447a55a" />


GetLedgerStats:

<img width="1861" height="255" alt="Screenshot from 2026-04-11 19-17-38" src="https://github.com/user-attachments/assets/78712206-33cd-4156-8151-ab9d9aa06d9a" />


---

## Technology Stack

| Layer | Technology |
|---|---|
| Blockchain Platform | Hyperledger Fabric v2.5 |
| Chaincode Language | Go 1.21 + `fabric-contract-api-go` |
| State Database | Apache CouchDB |
| Consensus | Raft (CFT) |
| Crypto | ECDSA (P256) via `cryptogen` |
| Container Runtime | Docker + Docker Compose v2 |
| CLI Tools | Fabric peer CLI, `jq` |
