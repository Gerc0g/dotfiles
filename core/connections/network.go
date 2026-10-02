package connections

import (
	"errors"
	"sort"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// EnvironmentHosts returns application hosts that the task's general Internet
// proxy must deny, across companies and research alike. Only the bound broker
// may access them. Git hosting is excluded: public code needs no company token.
func EnvironmentHosts() ([]string, error) {
	tree, err := world.ScanDefault()
	if err != nil {
		return nil, err
	}
	if len(tree.Warnings()) > 0 {
		return nil, errors.New("cannot establish the complete environment network denylist")
	}
	unique := map[string]bool{}
	for _, company := range tree.Companies() {
		c, err := snapshot(company.Slug)
		if err != nil {
			return nil, err
		}
		for _, environment := range c.Environments {
			u, err := validURL(environment.BaseURL)
			if err != nil {
				return nil, err
			}
			unique[strings.ToLower(u.Hostname())] = true
		}
	}
	hosts := make([]string, 0, len(unique))
	for host := range unique {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts, nil
}
