package logs

import (
	"context"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/ocserv"
)

func TestJournalLogReader_Available_Success(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: "", ExitCode: 0}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)
	if !r.Available(context.Background()) {
		t.Fatal("expected available when journalctl succeeds")
	}
}

func TestJournalLogReader_Available_Failure(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stderr: "No such unit", ExitCode: 1}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)
	if r.Available(context.Background()) {
		t.Fatal("expected not available when journalctl fails")
	}
}

func TestJournalLogReader_ReadLines_PanelJSON(t *testing.T) {
	journalOutput := strings.Join([]string{
		`{"__REALTIME_TIMESTAMP":"1746612000000000","MESSAGE":"{\"level\":\"info\",\"time\":\"2026-05-07T10:00:00Z\",\"message\":\"hello\",\"component\":\"main\"}"}`,
		`{"__REALTIME_TIMESTAMP":"1746612001000000","MESSAGE":"{\"level\":\"warn\",\"time\":\"2026-05-07T10:00:01Z\",\"message\":\"oops\",\"component\":\"auth\"}"}`,
	}, "\n")

	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: journalOutput, ExitCode: 0}
	r := NewJournalLogReader("ocserv-panel", SourcePanel, runner)

	result, err := r.ReadLines(context.Background(), ReadOpts{Limit: 10, Order: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(result.Lines))
	}
	if result.Lines[0].Level != "warn" {
		t.Errorf("first line level = %q, want warn (desc order)", result.Lines[0].Level)
	}
}

func TestJournalLogReader_ReadLines_CommandArgs(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: "", ExitCode: 0}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)

	_, _ = r.ReadLines(context.Background(), ReadOpts{Limit: 50, Order: "desc"})

	if len(runner.Calls) == 0 {
		t.Fatal("no calls recorded")
	}
	call := runner.Calls[len(runner.Calls)-1]
	if call.Name != "journalctl" {
		t.Fatalf("command = %q, want journalctl", call.Name)
	}
	// Should contain -u <unit> --output=json --no-pager
	args := strings.Join(call.Args, " ")
	if !strings.Contains(args, "-u ocserv") {
		t.Errorf("args missing unit: %v", call.Args)
	}
	if !strings.Contains(args, "--output=json") {
		t.Errorf("args missing --output=json: %v", call.Args)
	}
	if !strings.Contains(args, "--no-pager") {
		t.Errorf("args missing --no-pager: %v", call.Args)
	}
}
