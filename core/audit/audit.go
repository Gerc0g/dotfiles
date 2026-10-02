// Package audit records operation outcomes without payloads or credentials.
package audit

import (
	"database/sql"
	"github.com/Gerc0g/dotfiles/core/agentconfig"
	"github.com/Gerc0g/dotfiles/core/world"
	_ "modernc.org/sqlite"
	"os"
	"strings"
	"time"
)

func Record(operation, code string) error {
	// This is a logging schema, not an authorization registry: new domain
	// operations still execute through their owner, but are logged as unknown
	// until given a public audit identity. Payload-shaped strings never persist.
	operation = canonicalOperation(operation)
	switch strings.ToUpper(code) {
	case "OK":
		code = "OK"
	case "STARTED":
		code = "STARTED"
	case "DENIED":
		code = "DENIED"
	default:
		code = "FAILED"
	}
	root := agentconfig.DataRoot()
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	file, err := world.SafePath(root, "control.sqlite")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	f.Close()
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY, at TEXT NOT NULL, operation TEXT NOT NULL, outcome TEXT NOT NULL)`); err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO audit(at,operation,outcome) VALUES(?,?,?)", time.Now().UTC().Format(time.RFC3339Nano), operation, code)
	return err
}

func canonicalOperation(operation string) string {
	switch operation {
	case "system.status", "runtime.list", "runtime.status",
		"agent.get", "agent.save", "agent.preview", "agent.previewImport", "agent.inspect",
		"connections.get", "connections.save", "connections.credential.put", "connections.credential.revoke", "connections.verify", "connections.environment.verify",
		"knowledge.scopes", "knowledge.context", "knowledge.tick", "knowledge.list", "knowledge.resolve", "knowledge.get", "knowledge.save", "knowledge.graph", "knowledge.attachment", "knowledge.jobs", "knowledge.runJob",
		"history.list", "history.get", "history.continuation", "history.attachment",
		"runner.context.get", "runner.entity.show", "runner.entity.update", "runner.document.get", "runner.document.save",
		"runner.memory.capture", "runner.git.read", "runner.git.fetch", "runner.git.push", "runner.git.pr.create", "runner.git.pr.update", "runner.environment.request", "runner.denied":
		return operation
	default:
		return "unknown"
	}
}
