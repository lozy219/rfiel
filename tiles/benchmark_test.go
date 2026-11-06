package tiles

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func BenchmarkTileProcessZoom10(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/tile/:z/:x/:y", Process)

	req, _ := http.NewRequest("GET", "/tile/10/909/403", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTileProcessZoom14(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/tile/:z/:x/:y", Process)

	req, _ := http.NewRequest("GET", "/tile/14/14553/6457", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkTileProcessEmpty(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/tile/:z/:x/:y", Process)

	req, _ := http.NewRequest("GET", "/tile/10/0/0", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func TestSessionAndZoom15Endpoints(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/new_session/:t1/:t2", NewSession)
	r.GET("/tile/:z/:x/:y", Process)
	r.GET("/tile_session/:s/:z/:x/:y", ProcessSession)

	// Verify Zoom 15 works on main session
	wZ15 := httptest.NewRecorder()
	reqZ15, _ := http.NewRequest("GET", "/tile/15/29106/12914", nil)
	r.ServeHTTP(wZ15, reqZ15)
	if wZ15.Code != http.StatusOK || wZ15.Body.Len() == 0 {
		t.Fatalf("expected 200 non-empty MVT at zoom 15, got status=%d len=%d", wZ15.Code, wZ15.Body.Len())
	}

	// Verify NewSession returns a JS-safe session_id (<= 9007199254740991) and serves year tiles
	wSess := httptest.NewRecorder()
	reqSess, _ := http.NewRequest("GET", "/new_session/1704067200/1735689599", nil)
	r.ServeHTTP(wSess, reqSess)
	if wSess.Code != http.StatusOK {
		t.Fatalf("expected 200 from /new_session, got %d", wSess.Code)
	}
	var resp struct {
		SessionID int64 `json:"session_id"`
		Cached    bool  `json:"cached"`
	}
	if err := json.Unmarshal(wSess.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode session json: %v", err)
	}
	if resp.SessionID <= 0 || resp.SessionID > 9007199254740991 {
		t.Fatalf("session_id %d is not a safe JS positive integer", resp.SessionID)
	}

	wTile := httptest.NewRecorder()
	reqTile, _ := http.NewRequest("GET", fmt.Sprintf("/tile_session/%d/10/909/403", resp.SessionID), nil)
	r.ServeHTTP(wTile, reqTile)
	if wTile.Code != http.StatusOK || wTile.Body.Len() == 0 {
		t.Fatalf("expected 200 non-empty MVT for year session %d, got status=%d len=%d", resp.SessionID, wTile.Code, wTile.Body.Len())
	}
}
