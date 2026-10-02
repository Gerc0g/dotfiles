// Package control is the owner-only HQ transport boundary. Task capabilities
// are separate and prebound to a company by runner; they never enter Dispatch.
package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/Gerc0g/dotfiles/core/agentconfig"
	"github.com/Gerc0g/dotfiles/core/audit"
	"github.com/Gerc0g/dotfiles/core/connections"
	"github.com/Gerc0g/dotfiles/core/knowledge"
	"github.com/Gerc0g/dotfiles/core/runner"
)

type Request struct {
	Version   int             `json:"v"`
	Operation string          `json:"operation"`
	Args      json.RawMessage `json:"args"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Response struct {
	Version int      `json:"v"`
	OK      bool     `json:"ok"`
	Result  any      `json:"result,omitempty"`
	Error   *Failure `json:"error,omitempty"`
}

var codePattern = regexp.MustCompile(`HQ_[A-Z][A-Z0-9_]+`)

func Dispatch(r Request) Response {
	response := Response{Version: 1}
	// Persist the intent before any action. Known audit storage failures must
	// not allow a mutation that will subsequently be reported as rejected.
	if err := audit.Record(r.Operation, "STARTED"); err != nil {
		return Response{Version: 1, Error: &Failure{Code: "HQ_AUDIT_UNAVAILABLE", Message: "HQ_AUDIT_UNAVAILABLE"}}
	}
	var result any
	var err error
	if r.Version != 1 || !json.Valid(r.Args) || len(bytes.TrimSpace(r.Args)) == 0 || bytes.TrimSpace(r.Args)[0] != '{' {
		err = errors.New("HQ_REQUEST_INVALID")
	} else {
		switch {
		case r.Operation == "runtime.list":
			result, err = runner.List()
		case r.Operation == "runtime.status":
			var tasks []runner.Status
			tasks, err = runner.List()
			result = map[string]any{"tasks": tasks, "limits": map[string]any{"tasks": runner.MaxTasks, "perCompany": 1, "memory": runner.TaskMemory, "cpus": runner.TaskCPUs, "pids": runner.TaskPIDs, "diskBytes": runner.TaskDiskBytes, "diskEnforcement": "watchdog"}}
		case r.Operation == "system.status":
			result = map[string]any{"version": 1, "provider": "codex", "areas": []string{"work", "wikipedia", "settings"}, "privateTransport": true}
		case strings.HasPrefix(r.Operation, "agent."):
			result, err = agentconfig.Dispatch(r.Operation, r.Args)
		case strings.HasPrefix(r.Operation, "connections."):
			result, err = connections.Dispatch(r.Operation, r.Args)
		case strings.HasPrefix(r.Operation, "knowledge."), strings.HasPrefix(r.Operation, "history."):
			result, err = knowledge.Dispatch(r.Operation, r.Args)
		default:
			err = errors.New("HQ_OPERATION_UNKNOWN")
		}
	}
	code := "OK"
	if err != nil {
		code = codePattern.FindString(err.Error())
		if code == "" {
			code = "HQ_OPERATION_FAILED"
		}
		response.Error = &Failure{Code: code, Message: code}
	} else {
		response.OK = true
		response.Result = result
	}
	// Audit only operation identity, outcome and time. Raw arguments can contain
	// tokens, private notes or prompts and must never become a second secret store.
	if auditErr := audit.Record(r.Operation, code); auditErr != nil {
		if response.OK {
			response.OK = false
			response.Result = nil
			response.Error = &Failure{Code: "HQ_OPERATION_APPLIED_AUDIT_UNAVAILABLE", Message: "HQ_OPERATION_APPLIED_AUDIT_UNAVAILABLE"}
		}
	}
	return response
}

func Serve(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var r Request
	if err := decoder.Decode(&r); err != nil {
		return json.NewEncoder(output).Encode(Response{Version: 1, Error: &Failure{Code: "HQ_REQUEST_INVALID", Message: "HQ_REQUEST_INVALID"}})
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return json.NewEncoder(output).Encode(Response{Version: 1, Error: &Failure{Code: "HQ_REQUEST_INVALID", Message: "HQ_REQUEST_INVALID"}})
	}
	return json.NewEncoder(output).Encode(Dispatch(r))
}
