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

// CreateProductBatch - called by Manufacturer
func (s *SupplyChainContract) CreateProductBatch(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	productName string,
	quantity int,
	location string,
) error {

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

// UpdateShipmentStatus - called by Distributor
func (s *SupplyChainContract) UpdateShipmentStatus(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	location string,
) error {

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

// ConfirmDelivery - called by Retailer
func (s *SupplyChainContract) ConfirmDelivery(
	ctx contractapi.TransactionContextInterface,
	batchID string,
	location string,
) error {

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

// QueryBatch - returns current state of a batch
func (s *SupplyChainContract) QueryBatch(
	ctx contractapi.TransactionContextInterface,
	batchID string,
) (*ProductBatch, error) {
	return s.getBatch(ctx, batchID)
}

// GetProductHistory - returns full lifecycle using GetHistoryForKey
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

// helper
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
