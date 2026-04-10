package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SupplyChainContract struct {
	contractapi.Contract
}

type ProductBatch struct {
	BatchID       string `json:"batchID"`
	ProductName   string `json:"productName"`
	Quantity      int    `json:"quantity"`
	Status        string `json:"status"`
	CurrentHolder string `json:"currentHolder"`
	Location      string `json:"location"`
	Timestamp     string `json:"timestamp"`
	TransactionID string `json:"transactionID"`
}

type HistoryRecord struct {
	TxID      string       `json:"txID"`
	Timestamp string       `json:"timestamp"`
	IsDelete  bool         `json:"isDelete"`
	Record    ProductBatch `json:"record"`
}

// ─────────────────────────────────────────────────────────────
// ROLE-BASED ACCESS CONTROL HELPER
// ─────────────────────────────────────────────────────────────

// checkMSP verifies that the caller belongs to the expected MSP.
// If not, it returns a clear error message showing who tried to call.
func checkMSP(ctx contractapi.TransactionContextInterface, expectedMSP string) error {
	clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client MSP ID: %v", err)
	}
	if clientMSPID != expectedMSP {
		return fmt.Errorf(
			"access denied: this function requires %s but was called by %s",
			expectedMSP, clientMSPID,
		)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// WRITE FUNCTIONS (role-restricted)
// ─────────────────────────────────────────────────────────────

// CreateProductBatch - MANUFACTURER ONLY
func (s *SupplyChainContract) CreateProductBatch(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	productName string,
	quantity int,
	location string,
) error {

	// ── ROLE CHECK ──
	if err := checkMSP(ctx, "ManufacturerMSP"); err != nil {
		return err
	}

	existing, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return fmt.Errorf("failed to read from ledger: %v", err)
	}
	if existing != nil {
		return fmt.Errorf("batch %s already exists", batchID)
	}

	batch := ProductBatch{
		BatchID:       batchID,
		ProductName:   productName,
		Quantity:      quantity,
		Status:        "CREATED",
		CurrentHolder: "Manufacturer",
		Location:      location,
		Timestamp:     time.Now().Format(time.RFC3339),
		TransactionID: ctx.GetStub().GetTxID(),
	}

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(batchID, batchJSON)
}

// UpdateShipmentStatus - DISTRIBUTOR ONLY
func (s *SupplyChainContract) UpdateShipmentStatus(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	location string,
) error {

	// ── ROLE CHECK ──
	if err := checkMSP(ctx, "DistributorMSP"); err != nil {
		return err
	}

	batch, err := s.getBatch(ctx, batchID)
	if err != nil {
		return err
	}

	if batch.Status != "CREATED" {
		return fmt.Errorf("batch must be CREATED to update shipment, current status: %s", batch.Status)
	}

	batch.Status = "IN_TRANSIT"
	batch.CurrentHolder = "Distributor"
	batch.Location = location
	batch.Timestamp = time.Now().Format(time.RFC3339)
	batch.TransactionID = ctx.GetStub().GetTxID()

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(batchID, batchJSON)
}

// ConfirmDelivery - RETAILER ONLY
func (s *SupplyChainContract) ConfirmDelivery(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	location string,
) error {

	// ── ROLE CHECK ──
	if err := checkMSP(ctx, "RetailerMSP"); err != nil {
		return err
	}

	batch, err := s.getBatch(ctx, batchID)
	if err != nil {
		return err
	}

	if batch.Status != "IN_TRANSIT" {
		return fmt.Errorf("batch must be IN_TRANSIT to confirm delivery, current status: %s", batch.Status)
	}

	batch.Status = "DELIVERED"
	batch.CurrentHolder = "Retailer"
	batch.Location = location
	batch.Timestamp = time.Now().Format(time.RFC3339)
	batch.TransactionID = ctx.GetStub().GetTxID()

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(batchID, batchJSON)
}

// ─────────────────────────────────────────────────────────────
// READ FUNCTIONS (open to all orgs)
// ─────────────────────────────────────────────────────────────

// QueryBatch - any org can query current state of a batch
func (s *SupplyChainContract) QueryBatch(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) (*ProductBatch, error) {
	return s.getBatch(ctx, batchID)
}

// GetProductHistory - any org can view full lifecycle using GetHistoryForKey
func (s *SupplyChainContract) GetProductHistory(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) ([]HistoryRecord, error) {

	iterator, err := ctx.GetStub().GetHistoryForKey(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history for %s: %v", batchID, err)
	}
	defer iterator.Close()

	var history []HistoryRecord

	for iterator.HasNext() {
		response, err := iterator.Next()
		if err != nil {
			return nil, err
		}

		var batch ProductBatch
		if !response.IsDelete && response.Value != nil {
			err = json.Unmarshal(response.Value, &batch)
			if err != nil {
				return nil, err
			}
		}

		t := time.Unix(response.Timestamp.Seconds, int64(response.Timestamp.Nanos))

		record := HistoryRecord{
			TxID:      response.TxId,
			Timestamp: t.Format(time.RFC3339),
			IsDelete:  response.IsDelete,
			Record:    batch,
		}

		history = append(history, record)
	}

	return history, nil
}

// ─────────────────────────────────────────────────────────────
// RICH COUCHDB QUERIES (open to all orgs)
// ─────────────────────────────────────────────────────────────

// QueryBatchesByStatus - returns all batches with a given status
// Example: CREATED, IN_TRANSIT, DELIVERED
func (s *SupplyChainContract) QueryBatchesByStatus(
	ctx contractapi.TransactionContextInterface,
	status string,
) ([]*ProductBatch, error) {

	queryString := fmt.Sprintf(`{"selector":{"status":"%s"}}`, status)
	return executeRichQuery(ctx, queryString)
}

// QueryBatchesByHolder - returns all batches held by a specific org
// Example: Manufacturer, Distributor, Retailer
func (s *SupplyChainContract) QueryBatchesByHolder(
	ctx contractapi.TransactionContextInterface,
	holder string,
) ([]*ProductBatch, error) {

	queryString := fmt.Sprintf(`{"selector":{"currentHolder":"%s"}}`, holder)
	return executeRichQuery(ctx, queryString)
}

// GetAllBatches - returns every batch on the ledger
func (s *SupplyChainContract) GetAllBatches(
	ctx contractapi.TransactionContextInterface,
) ([]*ProductBatch, error) {

	queryString := `{"selector":{"batchID":{"$gt":null}}}`
	return executeRichQuery(ctx, queryString)
}

// executeRichQuery runs a CouchDB selector query and returns matching batches
func executeRichQuery(
	ctx contractapi.TransactionContextInterface,
	queryString string,
) ([]*ProductBatch, error) {

	iterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, fmt.Errorf("rich query failed: %v", err)
	}
	defer iterator.Close()

	var results []*ProductBatch

	for iterator.HasNext() {
		queryResponse, err := iterator.Next()
		if err != nil {
			return nil, err
		}

		var batch ProductBatch
		err = json.Unmarshal(queryResponse.Value, &batch)
		if err != nil {
			return nil, err
		}

		results = append(results, &batch)
	}

	if results == nil {
		results = []*ProductBatch{}
	}

	return results, nil
}

// ─────────────────────────────────────────────────────────────
// HELPER
// ─────────────────────────────────────────────────────────────

func (s *SupplyChainContract) getBatch(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) (*ProductBatch, error) {

	batchJSON, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch %s: %v", batchID, err)
	}
	if batchJSON == nil {
		return nil, fmt.Errorf("batch %s does not exist", batchID)
	}

	var batch ProductBatch
	err = json.Unmarshal(batchJSON, &batch)
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// RecallBatch - MANUFACTURER ONLY - marks a batch as recalled
func (s *SupplyChainContract) RecallBatch(
    ctx contractapi.TransactionContextInterface,
    batchID string,
    reason string,
) error {
    if err := checkMSP(ctx, "ManufacturerMSP"); err != nil {
        return err
    }
    batch, err := s.getBatch(ctx, batchID)
    if err != nil {
        return err
    }
    if batch.Status == "RECALLED" {
        return fmt.Errorf("batch %s is already recalled", batchID)
    }
    batch.Status = "RECALLED"
    batch.Timestamp = time.Now().Format(time.RFC3339)
    batch.TransactionID = ctx.GetStub().GetTxID()
    // store recall reason in Location field (or add a new field)
    batch.Location = "RECALL: " + reason

    batchJSON, err := json.Marshal(batch)
    if err != nil {
        return err
    }
    return ctx.GetStub().PutState(batchID, batchJSON)
}

// DeleteBatch - any authorized org - demonstrates DelState and IsDelete in history
func (s *SupplyChainContract) DeleteBatch(
    ctx contractapi.TransactionContextInterface,
    batchID string,
) error {
    existing, err := ctx.GetStub().GetState(batchID)
    if err != nil {
        return fmt.Errorf("failed to read batch: %v", err)
    }
    if existing == nil {
        return fmt.Errorf("batch %s does not exist", batchID)
    }
    return ctx.GetStub().DelState(batchID)
}

// GetBatchesByDateRange - CouchDB range query on timestamp field
// startDate and endDate in RFC3339 format e.g. "2024-01-01T00:00:00Z"
func (s *SupplyChainContract) GetBatchesByDateRange(
    ctx contractapi.TransactionContextInterface,
    startDate string,
    endDate string,
) ([]*ProductBatch, error) {
    queryString := fmt.Sprintf(
        `{"selector":{"timestamp":{"$gte":"%s","$lte":"%s"}},"sort":[{"timestamp":"asc"}]}`,
        startDate, endDate,
    )
    return executeRichQuery(ctx, queryString)
}

type LedgerStats struct {
    Total      int `json:"total"`
    Created    int `json:"created"`
    InTransit  int `json:"inTransit"`
    Delivered  int `json:"delivered"`
    Recalled   int `json:"recalled"`
}

// GetLedgerStats - returns aggregate counts of batches by status
func (s *SupplyChainContract) GetLedgerStats(
    ctx contractapi.TransactionContextInterface,
) (*LedgerStats, error) {
    batches, err := s.GetAllBatches(ctx)
    if err != nil {
        return nil, err
    }
    stats := &LedgerStats{}
    for _, b := range batches {
        stats.Total++
        switch b.Status {
        case "CREATED":    stats.Created++
        case "IN_TRANSIT": stats.InTransit++
        case "DELIVERED":  stats.Delivered++
        case "RECALLED":   stats.Recalled++
        }
    }
    return stats, nil
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SupplyChainContract{})
	if err != nil {
		fmt.Printf("Error creating chaincode: %v\n", err)
		return
	}
	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting chaincode: %v\n", err)
	}
}
