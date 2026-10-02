package connections

import (
	"reflect"
	"testing"
)

func TestEnvironmentHostsIncludeOtherCompaniesWithoutGitHosts(t *testing.T) {
	c := setup(t)
	c.Environments = []Environment{{ID: "prod", Tier: "prod", Kind: "http", BaseURL: "https://PROD.example:8443", Requests: []Endpoint{}}}
	if _, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "config": c}); err != nil {
		t.Fatal(err)
	}
	c.CompanyID = "beta"
	c.Repositories = []Repository{}
	c.Environments[0].BaseURL = "https://other-prod.example"
	if _, err := call(t, "connections.save", map[string]any{"companyId": "beta", "config": c}); err != nil {
		t.Fatal(err)
	}
	got, err := EnvironmentHosts()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"other-prod.example", "prod.example"}) {
		t.Fatalf("environment bypass hosts missing: %v", got)
	}
}
