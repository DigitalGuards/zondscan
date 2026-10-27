package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func activityRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/slashings", handleSlashings)
	r.GET("/validator-set-changes", handleValidatorSetChanges)
	r.GET("/validator/:id", handleValidator)
	return r
}

func TestActivityHandlersPaginationFiltersAndEmptyArrays(t *testing.T) {
	oldEvents, oldChanges := getSlashings, getValidatorSetChanges
	t.Cleanup(func() { getSlashings, getValidatorSetChanges = oldEvents, oldChanges })
	getSlashings = func(ctx context.Context, q models.ValidatorActivityQuery) (*models.SlashingsResponse, error) {
		if q.Page != 2 || q.Limit != 1 || q.ValidatorIndex != "0" {
			t.Fatal(q)
		}
		return &models.SlashingsResponse{Page: q.Page, Limit: q.Limit}, nil
	}
	getValidatorSetChanges = func(ctx context.Context, q models.ValidatorActivityQuery) (*models.ValidatorSetChangesResponse, error) {
		if q.Page != 1 || q.Limit != 20 || q.Kind != "activated" || q.FromEpoch == nil || *q.FromEpoch != 0 || q.ToEpoch == nil || *q.ToEpoch != 4 || q.ValidatorIndex != "0" {
			t.Fatal(q)
		}
		return &models.ValidatorSetChangesResponse{Page: q.Page, Limit: q.Limit}, nil
	}
	r := activityRouter()
	for _, tc := range []struct{ path, field string }{
		{"/slashings?page=2&limit=1&validator=000", "events"},
		{"/validator-set-changes?fromEpoch=0&toEpoch=4&kind=activated&validator=0", "changes"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"`+tc.field+`":[]`) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
}

func TestActivityHandlersRejectInvalidFilters(t *testing.T) {
	r := activityRouter()
	for _, path := range []string{
		"/slashings?page=-1", "/slashings?page=0", "/slashings?page=1000001", "/slashings?page=99999999999999999999",
		"/slashings?limit=101", "/slashings?limit=0", "/slashings?validator=-1", "/slashings?validator=18446744073709551616",
		"/validator-set-changes?fromEpoch=-1", "/validator-set-changes?toEpoch=18446744073709551615",
		"/validator-set-changes?fromEpoch=4&toEpoch=3", "/validator-set-changes?kind=withdrawn", "/validator-set-changes?fromEpoch=abc",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 400 {
			t.Fatalf("%s = %d", path, w.Code)
		}
	}
}

func TestActivityHandlersPopulatedAndDetail(t *testing.T) {
	oldEvents, oldChanges, oldDetail := getSlashings, getValidatorSetChanges, getValidatorByID
	t.Cleanup(func() { getSlashings, getValidatorSetChanges, getValidatorByID = oldEvents, oldChanges, oldDetail })
	event := models.SlashingEvent{ID: "17:attester:0", Slot: 17, Epoch: 2, Type: "attester", ValidatorIndices: []string{"0", "2"}, Whistleblower: "4", ConflictingRoots: []string{"0xaa", "0xbb"}}
	change := models.ValidatorSetChange{ID: "3:slashed:0", Epoch: 3, Kind: "slashed", ValidatorIndex: "0", Pubkey: "0xcc"}
	getSlashings = func(_ context.Context, q models.ValidatorActivityQuery) (*models.SlashingsResponse, error) {
		return &models.SlashingsResponse{Events: []models.SlashingEvent{event}, Total: 1, Page: q.Page, Limit: q.Limit, SlashedValidatorCount: 2, BackfillComplete: true}, nil
	}
	getValidatorSetChanges = func(_ context.Context, q models.ValidatorActivityQuery) (*models.ValidatorSetChangesResponse, error) {
		return &models.ValidatorSetChangesResponse{Changes: []models.ValidatorSetChange{change}, Total: 1, Page: q.Page, Limit: q.Limit}, nil
	}
	getValidatorByID = func(id string) (*models.ValidatorDetailResponse, error) {
		return &models.ValidatorDetailResponse{Index: id, Slashed: true, SlashingEvents: []models.SlashingEvent{event}, SlashingEventsTotal: 1, ValidatorSetChanges: []models.ValidatorSetChange{change}, ValidatorSetChangesTotal: 1}, nil
	}
	r := activityRouter()
	for _, path := range []string{"/slashings", "/validator-set-changes", "/validator/0"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if path == "/validator/0" {
			if string(fields["slashed"]) != "true" || !strings.Contains(string(fields["slashingEvents"]), `"whistleblower":"4"`) || !strings.Contains(string(fields["validatorSetChanges"]), `"kind":"slashed"`) {
				t.Fatal(w.Body.String())
			}
		}
	}
}

func TestActivityHandlersDatabaseErrorsRemainPrivate(t *testing.T) {
	oldEvents, oldChanges, oldDetail := getSlashings, getValidatorSetChanges, getValidatorByID
	t.Cleanup(func() { getSlashings, getValidatorSetChanges, getValidatorByID = oldEvents, oldChanges, oldDetail })
	getSlashings = func(context.Context, models.ValidatorActivityQuery) (*models.SlashingsResponse, error) {
		return nil, errors.New("private diagnostic")
	}
	getValidatorSetChanges = func(context.Context, models.ValidatorActivityQuery) (*models.ValidatorSetChangesResponse, error) {
		return nil, errors.New("private diagnostic")
	}
	getValidatorByID = func(string) (*models.ValidatorDetailResponse, error) { return nil, errors.New("private diagnostic") }
	r := activityRouter()
	for _, path := range []string{"/slashings", "/validator-set-changes", "/validator/0"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 500 || strings.Contains(w.Body.String(), "private diagnostic") {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	getValidatorByID = func(string) (*models.ValidatorDetailResponse, error) { return nil, db.ErrValidatorNotFound }
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/validator/0", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
