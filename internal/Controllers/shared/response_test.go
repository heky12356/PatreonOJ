package shared

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIErrorResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	APIError(c, 400, "bad request")

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if int(body["code"].(float64)) != 400 {
		t.Fatalf("expected code=400, got %v", body["code"])
	}
	if body["msg"] != "bad request" || body["error"] != "bad request" {
		t.Fatalf("unexpected msg/error: %+v", body)
	}
}

func TestAPIOKResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	APIOK(c, 200, "ok", gin.H{"data": "x", "code": 999})

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if int(body["code"].(float64)) != 200 {
		t.Fatalf("expected code=200, got %v", body["code"])
	}
	if body["msg"] != "ok" {
		t.Fatalf("expected msg=ok, got %v", body["msg"])
	}
	if body["data"] != "x" {
		t.Fatalf("expected data=x, got %v", body["data"])
	}
}
