package routes

import (
	"backendAPI/models"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEpochHandlerSerializesProposedMissedAndUnknownProposers(t *testing.T) {
	original := getEpochDetail
	t.Cleanup(func() { getEpochDetail = original })
	getEpochDetail = func(id string) (*models.EpochDetailResponse, error) {
		if id != "2" {
			t.Fatalf("epoch = %s", id)
		}
		return &models.EpochDetailResponse{Epoch: id, StartSlot: 16, EndSlot: 24, Blocks: []models.EpochDetailBlock{
			{Slot: 16, Status: "proposed", ProposerIndex: "0", ProposerPubkey: "0xaabb", BlockNumber: "9", BlockHash: "0xabc"},
			{Slot: 17, Status: "missed", ProposerIndex: "42", ProposerPubkey: "0xccdd"},
			{Slot: 18, Status: "proposed", BlockNumber: "10", BlockHash: "0xdef"},
			{Slot: 19, Status: "missed"},
		}}, nil
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/epoch/:id", handleEpochByID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/epoch/2", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Blocks []map[string]any `json:"blocks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if row := response.Blocks[0]; row["slot"] != float64(16) || row["blockNumber"] != "9" || row["proposerIndex"] != "0" {
		t.Fatalf("proposed = %v", row)
	}
	missed := response.Blocks[1]
	if missed["status"] != "missed" || missed["proposerIndex"] != "42" || missed["proposerPubkey"] != "0xccdd" {
		t.Fatalf("missed = %v", missed)
	}
	for _, field := range []string{"blockNumber", "blockHash", "timestamp"} {
		if _, exists := missed[field]; exists {
			t.Fatalf("missed slot exposes %s", field)
		}
	}
	for _, row := range response.Blocks[2:] {
		if _, exists := row["proposerIndex"]; exists {
			t.Fatal("unknown proposer index was serialized")
		}
		if _, exists := row["proposerPubkey"]; exists {
			t.Fatal("unknown proposer pubkey was serialized")
		}
	}
}

func TestEpochHandlerErrors(t *testing.T) {
	original := getEpochDetail
	t.Cleanup(func() { getEpochDetail = original })
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		message string
		status  int
	}{
		{"invalid epoch number", 404},
		{"epoch 9 has not yet occurred (head: 8)", 404},
		{"database read failed: sensitive diagnostic", 500},
	} {
		getEpochDetail = func(string) (*models.EpochDetailResponse, error) { return nil, errors.New(tc.message) }
		router := gin.New()
		router.GET("/epoch/:id", handleEpochByID)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/epoch/9", nil))
		if recorder.Code != tc.status {
			t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
		}
		if tc.status == 500 && strings.Contains(recorder.Body.String(), "sensitive diagnostic") {
			t.Fatal("handler exposed internal diagnostic")
		}
	}
}
