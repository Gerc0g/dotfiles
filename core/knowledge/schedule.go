package knowledge

// ScheduledReport deliberately contains only job identities and outcomes. Full
// lint findings and private note contents belong in the scoped owner journal.
type ScheduledReport struct {
	Jobs   []ScheduledJob `json:"jobs"`
	Failed int            `json:"failed"`
}
type ScheduledJob struct {
	Scope  Scope  `json:"scope"`
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
}

func runScheduledJobs() (ScheduledReport, error) {
	report := ScheduledReport{Jobs: []ScheduledJob{}}
	scopes, err := listScopes()
	if err != nil {
		return report, err
	}
	for _, scope := range scopes {
		if scope.Kind != "research" && scope.Kind != "company" {
			continue
		}
		root, err := openScope(scope)
		if err != nil {
			report.Jobs = append(report.Jobs, ScheduledJob{Scope: scope, Kind: "scope", Status: "failed"})
			report.Failed++
			continue
		}
		kinds := []string{"lint", "index", "review"}
		if scope.Kind == "company" {
			kinds = append(kinds, "hot")
		}
		for _, kind := range kinds {
			job, err := runJob(root, Request{Scope: scope, Kind: kind})
			summary := ScheduledJob{Scope: scope, Kind: kind, ID: job.ID, Status: job.Status}
			if err != nil || job.Status == "failed" {
				summary.Status = "failed"
				report.Failed++
			}
			report.Jobs = append(report.Jobs, summary)
		}
		root.Close()
	}
	return report, nil
}
