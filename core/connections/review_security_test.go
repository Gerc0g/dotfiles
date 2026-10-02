package connections

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestEnvironmentResponseRedactsJSONEscapedCredential(t *testing.T) {
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	const token = "fixture/token"
	httpTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(`{"authorization":"Bearer fixture\/token"}`, 200), nil
	})
	result, err := environmentRequest(context.Background(), Environment{Tier: "stage", Kind: "http", BaseURL: "https://stage.example", Requests: []Endpoint{{ID: "headers", Method: "GET", Path: "/headers"}}}, "headers", nil, token)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(result.(map[string]any)["body"].(string)), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body["authorization"], token) {
		t.Fatal("escaped connector credential leaked in task response")
	}
}
