package accesslog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBuildRequestParams_query(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1&sort=name&tag=a&tag=b", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	params := buildRequestParams(c)

	query, ok := params["query"].(map[string]interface{})
	if !ok {
		t.Fatalf("query params missing: %#v", params)
	}
	if query["page"] != "1" {
		t.Fatalf("page = %v, want 1", query["page"])
	}
	if query["sort"] != "name" {
		t.Fatalf("sort = %v, want name", query["sort"])
	}
	tag, ok := query["tag"].([]string)
	if !ok || len(tag) != 2 || tag[0] != "a" || tag[1] != "b" {
		t.Fatalf("tag = %v, want [a b]", query["tag"])
	}
}

func TestBuildRequestParams_queryWithBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/users?dry_run=true", strings.NewReader(`{"name":"Jane"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	cacheRequestBody(c)

	params := buildRequestParams(c)

	query, ok := params["query"].(map[string]interface{})
	if !ok || query["dry_run"] != "true" {
		t.Fatalf("query = %v, want dry_run=true", params["query"])
	}
	body, ok := params["body"].(map[string]interface{})
	if !ok || body["name"] != "Jane" {
		t.Fatalf("body = %v, want name=Jane", params["body"])
	}
}

func TestBuildRequestParams_pathParams(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/users/42?active=true", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("42")

	params := buildRequestParams(c)

	query, ok := params["query"].(map[string]interface{})
	if !ok || query["active"] != "true" {
		t.Fatalf("query = %v, want active=true", params["query"])
	}

	pathParams, ok := params["path_params"].(map[string]interface{})
	if !ok {
		t.Fatalf("path params missing: %#v", params)
	}
	if pathParams["id"] != "42" {
		t.Fatalf("id = %v, want 42", pathParams["id"])
	}
}
