package analytics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"my_feed_system/internal/response"
)

func TestReportAcceptsAllowedEvent(t *testing.T) {
	w, c := newReportContext(t, ReportRequest{
		VisitorID: "anon-123",
		Events: []Event{{
			Name:     "page_view",
			Page:     "/",
			ClientTS: 1,
			Properties: map[string]any{
				"name":     "home",
				"password": "should-drop",
			},
		}},
	})

	NewHandler().Report(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != response.Success {
		t.Fatalf("code = %s, want success", body.Code)
	}
}

func TestReportRejectsUnknownEvent(t *testing.T) {
	w, c := newReportContext(t, ReportRequest{
		VisitorID: "anon-123",
		Events:    []Event{{Name: "drop_table", Page: "/"}},
	})

	NewHandler().Report(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestReportRejectsEmptyBatch(t *testing.T) {
	w, c := newReportContext(t, ReportRequest{VisitorID: "anon-123"})

	NewHandler().Report(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func newReportContext(t *testing.T, req ReportRequest) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	httpReq := httptest.NewRequest(http.MethodPost, "/event/report", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	c.Request = httpReq
	return w, c
}
