package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCheckDBNeedsInitialization(t *testing.T) {
	previousDB, previousLog := global.GVA_DB, global.GVA_LOG
	global.GVA_DB, global.GVA_LOG = nil, zap.NewNop()
	t.Cleanup(func() { global.GVA_DB, global.GVA_LOG = previousDB, previousLog })

	router := gin.New()
	(&InitRouter{}).InitInitRouter(router.Group(""))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/init/checkdb", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var result struct {
		Code int
		Data struct {
			NeedInit bool
		}
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || !result.Data.NeedInit {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}
