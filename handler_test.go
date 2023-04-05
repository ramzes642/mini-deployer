package main

import (
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunHandler(t *testing.T) {
	Cfg.GitlabToken = "testToken"
	Cfg.Timeout = int64(time.Second)

	handler := runHandler("env", "/env")

	req := httptest.NewRequest("POST", "http://example.com/env", strings.NewReader(`KEY=value`))
	req.Header.Set("X-Gitlab-Token", "testToken")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "POST_KEY=value")
}

func TestPostTestHandler(t *testing.T) {
	someJson := `{"Some": "json"}`

	Cfg.GitlabToken = "testToken"
	Cfg.Timeout = int64(time.Second)

	handler := runHandler("cat", "/cat")

	req := httptest.NewRequest("POST", "http://example.com/cat", strings.NewReader(someJson))
	req.Header.Set("X-Gitlab-Token", "testToken")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), someJson)
}
