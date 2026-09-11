package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_UpsertUpstream_rejectsUnknownJSONField(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	body := `{"id":"third","name":"Third","type":"http","address":"127.0.0.1:9090","extra":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_ActivateUpstream_rejectsTrailingJSONValue(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/config/activate", bytes.NewBufferString(`{"id":"upstream2"}{}`))
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_ActivateUpstream_rejectsMalformedID(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/config/activate", bytes.NewBufferString(`{"id":"../upstream2"}`))
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_DeleteUpstream_returnsNotFoundForNestedID(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/upstreams/upstream2/nested", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_DeleteUpstream_returnsNotFoundForEncodedSlashInID(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/upstreams/upstream2%2Fnested", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_DeleteUpstream_rejectsMalformedID(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/upstreams/%20upstream2", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_KnownRoute_returnsMethodNotAllowedForWrongMethod(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/config/upstreams", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandler_UnknownRoute_returnsNotFound(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/unknown", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
