package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SupplyChainContract is the main contract struct that embeds
// the Fabric contract API base. All chaincode functions are
// defined as methods on this struct.
type SupplyChainContract struct {
	contractapi.Contract
}

// ProductBatch holds all information about a single product batch
// moving through the supply chain. Each field is tagged for JSON
// serialization so it can be stored and retrieved from the ledger.
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

// HistoryRecord wraps a single entry from the blockchain history of a batch.
// IsDelete tells us if this entry represents a deletion (tombstone).
type HistoryRecord struct {
	TxID      string       `json:"txID"`
	Timestamp string       `json:"timestamp"`
	IsDelete  bool         `json:"isDelete"`
	Record    ProductBatch `json:"record"`
}

// QualityCheck stores a single quality inspection record for a batch.
// Multiple QC records can exist for the same batch, stored using composite keys.
type QualityCheck struct {
	BatchID   string `json:"batchID"`
	CheckedBy string `json:"checkedBy"`
	Result    string `json:"result"` // PASS or FAIL
	Remarks   string `json:"remarks"`
	Timestamp string `json:"timestamp"`
	TxID      string `json:"txID"`
}

// LedgerStats gives a summary count of all batches grouped by status.
// Useful for dashboard queries and monitoring the overall supply chain state.
type LedgerStats struct {
	Total       int `json:"total"`
	Created     int `json:"created"`
	InTransit   int `json:"inTransit"`
	Delivered   int `json:"delivered"`
	Recalled    int `json:"recalled"`
	Transferred int `json:"transferred"`
}

// PaginatedResult wraps the output of a paginated CouchDB query.
// The Bookmark field must be passed back in the next call to get the next page.
type PaginatedResult struct {
	Records             []*ProductBatch `json:"records"`
	FetchedRecordsCount int             `json:"fetchedRecordsCount"`
	Bookmark            string          `json:"bookmark"`
}

// =============================================================
// ROLE-BASED ACCESS CONTROL
// =============================================================

// checkMSP checks whether the organization calling this function
// matches the expected MSP. If someone from the wrong org tries
// to call a restricted function, they get a clear error message.
// This is used to enforce that only Manufacturer can create batches,
// only Distributor can update shipment, and so on.
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

// =============================================================
// WRITE FUNCTIONS
// =============================================================

// CreateProductBatch is called by the Manufacturer to register a new
// product batch on the ledger. It checks for duplicate batch IDs before
// writing, and sets the initial status to CREATED.
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

// UpdateShipmentStatus is called by the Distributor when they receive a batch
// and dispatch it for delivery. The batch must be in CREATED state first,
// otherwise the transition is rejected to enforce correct workflow order.
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

// ConfirmDelivery is called by the Retailer when they receive the batch
// at their end. The batch must be IN_TRANSIT for this to succeed.
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

// UpdateBatchQuantity allows any organization to correct the quantity of a batch,
// for example when some units are damaged during transit. It emits a blockchain
// event so that external applications can be notified of the change.
func (s *SupplyChainContract) UpdateBatchQuantity(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	newQuantity int,
	reason string,
) error {
	clientMSP, _ := ctx.GetClientIdentity().GetMSPID()

	batch, err := s.getBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch.Status == "DELIVERED" || batch.Status == "RECALLED" {
		return fmt.Errorf("cannot update quantity for batch with status: %s", batch.Status)
	}

	oldQuantity := batch.Quantity
	batch.Quantity = newQuantity
	batch.Timestamp = time.Now().Format(time.RFC3339)
	batch.TransactionID = ctx.GetStub().GetTxID()

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	// Emit event for quantity change
	eventPayload := fmt.Sprintf(
		`{"batchID":"%s","oldQuantity":%d,"newQuantity":%d,"reason":"%s","updatedBy":"%s"}`,
		batchID, oldQuantity, newQuantity, reason, clientMSP,
	)
	ctx.GetStub().SetEvent("QuantityUpdateEvent", []byte(eventPayload))

	return ctx.GetStub().PutState(batchID, batchJSON)
}

// RecallBatch is used by the Manufacturer to mark a batch as recalled,
// for example due to contamination or quality failure. Once recalled,
// the batch cannot be recalled again.
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

// TransferOwnership handles a generic handoff of a batch between organizations,
// outside the normal CREATED => IN_TRANSIT => DELIVERED flow. It emits a
// TransferEvent so that applications can track ownership changes in real time.
func (s *SupplyChainContract) TransferOwnership(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	newHolder string,
	newLocation string,
) error {
	clientMSP, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client MSP: %v", err)
	}

	batch, err := s.getBatch(ctx, batchID)
	if err != nil {
		return err
	}

	if batch.Status == "DELIVERED" || batch.Status == "RECALLED" {
		return fmt.Errorf("cannot transfer batch with status: %s", batch.Status)
	}

	oldHolder := batch.CurrentHolder
	batch.CurrentHolder = newHolder
	batch.Location = newLocation
	batch.Status = "TRANSFERRED"
	batch.Timestamp = time.Now().Format(time.RFC3339)
	batch.TransactionID = ctx.GetStub().GetTxID()

	batchJSON, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	// Emit a transfer event
	eventPayload := fmt.Sprintf(`{"batchID":"%s","from":"%s","to":"%s","by":"%s"}`,
		batchID, oldHolder, newHolder, clientMSP)
	ctx.GetStub().SetEvent("TransferEvent", []byte(eventPayload))

	return ctx.GetStub().PutState(batchID, batchJSON)
}

// DeleteBatch removes a batch from the world state. Even after deletion,
// the full history of the batch including a tombstone record remains on
// the blockchain, which demonstrates immutability of historical data.
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

// AddQualityCheck records a quality inspection result for a batch.
// Each QC entry is stored with a composite key (QC~batchID~txID) so that
// multiple checks can exist for the same batch without overwriting each other.
func (s *SupplyChainContract) AddQualityCheck(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	result string,
	remarks string,
) error {
	// verify batch exists
	_, err := s.getBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if result != "PASS" && result != "FAIL" {
		return fmt.Errorf("result must be PASS or FAIL, got: %s", result)
	}

	clientMSP, _ := ctx.GetClientIdentity().GetMSPID()

	qc := QualityCheck{
		BatchID:   batchID,
		CheckedBy: clientMSP,
		Result:    result,
		Remarks:   remarks,
		Timestamp: time.Now().Format(time.RFC3339),
		TxID:      ctx.GetStub().GetTxID(),
	}

	// Store using composite key: QC~batchID~txID
	compositeKey, err := ctx.GetStub().CreateCompositeKey("QC", []string{batchID, ctx.GetStub().GetTxID()})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	qcJSON, err := json.Marshal(qc)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(compositeKey, qcJSON)
}

// =============================================================
// READ FUNCTIONS
// =============================================================

// QueryBatch returns the current state of a batch from the world state.
// Any organization can call this to check the latest status of a batch.
func (s *SupplyChainContract) QueryBatch(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) (*ProductBatch, error) {
	return s.getBatch(ctx, batchID)
}

// GetProductHistory returns the complete lifecycle of a batch by reading
// all historical states from the blockchain using GetHistoryForKey.
// Each entry includes the transaction ID, timestamp, and the batch state
// at that point. If a batch was deleted, the last entry will have IsDelete = true.
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

// BatchExists is a simple utility that checks whether a batch key exists
// on the ledger. Returns true if found, false if not.
func (s *SupplyChainContract) BatchExists(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) (bool, error) {
	batchJSON, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return false, fmt.Errorf("failed to read from ledger: %v", err)
	}
	return batchJSON != nil, nil
}

// GetQualityChecks retrieves all quality check records for a given batch.
// It uses GetStateByPartialCompositeKey to find all entries that start
// with the prefix QC~batchID, regardless of the txID suffix.
func (s *SupplyChainContract) GetQualityChecks(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) ([]QualityCheck, error) {
	iterator, err := ctx.GetStub().GetStateByPartialCompositeKey("QC", []string{batchID})
	if err != nil {
		return nil, fmt.Errorf("failed to get QC records: %v", err)
	}
	defer iterator.Close()

	var checks []QualityCheck
	for iterator.HasNext() {
		result, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		var qc QualityCheck
		err = json.Unmarshal(result.Value, &qc)
		if err != nil {
			return nil, err
		}
		checks = append(checks, qc)
	}
	if checks == nil {
		checks = []QualityCheck{}
	}
	return checks, nil
}

// VerifyBatchIntegrity is an audit function that compares the current world
// state of a batch against the most recent entry in its blockchain history.
// If they match, the ledger is intact. A mismatch would indicate tampering.
func (s *SupplyChainContract) VerifyBatchIntegrity(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) (bool, error) {
	// Get current state
	current, err := s.getBatch(ctx, batchID)
	if err != nil {
		return false, err
	}

	// Get history and find latest non-delete entry
	iterator, err := ctx.GetStub().GetHistoryForKey(batchID)
	if err != nil {
		return false, err
	}
	defer iterator.Close()

	var latest *ProductBatch
	for iterator.HasNext() {
		response, err := iterator.Next()
		if err != nil {
			return false, err
		}
		if !response.IsDelete && response.Value != nil {
			var batch ProductBatch
			err = json.Unmarshal(response.Value, &batch)
			if err != nil {
				return false, err
			}
			latest = &batch
			break // history is newest-first
		}
	}

	if latest == nil {
		return false, fmt.Errorf("no history found for batch %s", batchID)
	}

	// Compare current state with latest history entry
	intact := current.Status == latest.Status &&
		current.CurrentHolder == latest.CurrentHolder &&
		current.TransactionID == latest.TransactionID

	return intact, nil
}

// =============================================================
// RICH COUCHDB QUERIES
// =============================================================

// QueryBatchesByStatus returns all batches that currently have the given status.
// Supported values: CREATED, IN_TRANSIT, DELIVERED, RECALLED, TRANSFERRED
func (s *SupplyChainContract) QueryBatchesByStatus(
	ctx contractapi.TransactionContextInterface,
	status string,
) ([]*ProductBatch, error) {

	queryString := fmt.Sprintf(`{"selector":{"status":"%s"}}`, status)
	return executeRichQuery(ctx, queryString)
}

// QueryBatchesByHolder returns all batches currently held by a specific organization.
// Supported values: Manufacturer, Distributor, Retailer
func (s *SupplyChainContract) QueryBatchesByHolder(
	ctx contractapi.TransactionContextInterface,
	holder string,
) ([]*ProductBatch, error) {

	queryString := fmt.Sprintf(`{"selector":{"currentHolder":"%s"}}`, holder)
	return executeRichQuery(ctx, queryString)
}

// GetAllBatches returns every product batch currently on the ledger.
// The selector filters out composite key entries (QC records) by requiring
// a non-empty productName, so only actual batch records are returned.
func (s *SupplyChainContract) GetAllBatches(
	ctx contractapi.TransactionContextInterface,
) ([]*ProductBatch, error) {
	queryString := `{"selector":{"batchID":{"$gt":null},"productName":{"$gt":""}}}`
	return executeRichQuery(ctx, queryString)
}

// GetBatchesByLocation filters batches by their current location field.
// Useful for finding all batches at a specific city or warehouse.
func (s *SupplyChainContract) GetBatchesByLocation(
	ctx contractapi.TransactionContextInterface,
	location string,
) ([]*ProductBatch, error) {
	queryString := fmt.Sprintf(`{"selector":{"location":"%s"}}`, location)
	return executeRichQuery(ctx, queryString)
}

// GetBatchesByDateRange - CouchDB range query on timestamp field
// startDate and endDate in RFC3339 format
func (s *SupplyChainContract) GetBatchesByDateRange(
	ctx contractapi.TransactionContextInterface,
	startDate string,
	endDate string,
) ([]*ProductBatch, error) {
	queryString := fmt.Sprintf(
		`{"selector":{"productName":{"$gt":""},"timestamp":{"$gte":"%s","$lte":"%s"}},"sort":[{"timestamp":"asc"}]}`,
		startDate, endDate,
	)
	return executeRichQuery(ctx, queryString)
}

// GetLedgerStats counts all batches on the ledger and groups them by status.
// This gives a quick overview of the entire supply chain state at a glance.
func (s *SupplyChainContract) GetLedgerStats(
	ctx contractapi.TransactionContextInterface,
) (*LedgerStats, error) {
	queryString := `{"selector":{"batchID":{"$gt":null},"productName":{"$gt":""}}}`
	batches, err := executeRichQuery(ctx, queryString)
	if err != nil {
		return nil, err
	}
	stats := &LedgerStats{}
	for _, b := range batches {
		stats.Total++
		switch b.Status {
		case "CREATED":
			stats.Created++
		case "IN_TRANSIT":
			stats.InTransit++
		case "DELIVERED":
			stats.Delivered++
		case "RECALLED":
			stats.Recalled++
		case "TRANSFERRED":
			stats.Transferred++
		}
	}
	return stats, nil
}

// GetBatchesWithPagination returns batches filtered by status in pages.
// Pass an empty string as bookmark to get the first page. The bookmark
// returned in the response should be passed in the next call to get
// the following page. pageSize controls how many records per page.
func (s *SupplyChainContract) GetBatchesWithPagination(
	ctx contractapi.TransactionContextInterface,
	status string,
	pageSize int,
	bookmark string,
) (*PaginatedResult, error) {
	queryString := fmt.Sprintf(`{"selector":{"status":"%s"}}`, status)

	iterator, metadata, err := ctx.GetStub().GetQueryResultWithPagination(
		queryString, int32(pageSize), bookmark,
	)
	if err != nil {
		return nil, fmt.Errorf("paginated query failed: %v", err)
	}
	defer iterator.Close()

	var batches []*ProductBatch
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
		batches = append(batches, &batch)
	}
	if batches == nil {
		batches = []*ProductBatch{}
	}

	return &PaginatedResult{
		Records:             batches,
		FetchedRecordsCount: int(metadata.FetchedRecordsCount),
		Bookmark:            metadata.Bookmark,
	}, nil
}

// =============================================================
// INTERNAL HELPER
// =============================================================

// getBatch is an internal helper used by most functions to read a batch
// from the ledger and unmarshal it. Returns a clear error if the batch
// does not exist, which avoids repeating this logic in every function.
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

// executeRichQuery is a shared helper that runs any CouchDB selector query
// and collects the results into a slice of ProductBatch pointers.
// All rich query functions call this internally to avoid duplicating
// the iterator handling logic.
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

// =============================================================
// ENTRY POINT
// =============================================================

// main initializes and starts the chaincode. If chaincode creation
// or startup fails, the error is printed and the process exits.
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
