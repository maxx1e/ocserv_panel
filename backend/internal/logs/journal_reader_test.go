package logs

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestJournalLogReader_buildArgs_SinceUntilIncludeUTCSuffix(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)

	from := time.Date(2026, 5, 8, 15, 49, 15, 0, time.UTC)
	to := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	args := r.buildArgs(ReadOpts{
		Limit: 50,
		From:  from,
		To:    to,
	})

	joined := strings.Join(args, " ")
	wantSince := "--since=2026-05-08 15:49:15 UTC"
	wantUntil := "--until=2026-05-09 12:00:00 UTC"

	if !strings.Contains(joined, wantSince) {
		t.Errorf("args = %q, want to contain %q", joined, wantSince)
	}
	if !strings.Contains(joined, wantUntil) {
		t.Errorf("args = %q, want to contain %q", joined, wantUntil)
	}
}

func TestJournalLogReader_buildArgs_NonUTCTimeConvertedToUTC(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)

	// Simulate a time in +04:00 timezone
	loc := time.FixedZone("TEST+4", 4*3600)
	// 15:49:15 in +04:00 is 11:49:15 UTC
	from := time.Date(2026, 5, 8, 15, 49, 15, 0, loc)

	args := r.buildArgs(ReadOpts{
		Limit: 50,
		From:  from,
	})

	joined := strings.Join(args, " ")
	wantSince := "--since=2026-05-08 11:49:15 UTC"

	if !strings.Contains(joined, wantSince) {
		t.Errorf("args = %q, want to contain %q (non-UTC time should be converted to UTC)", joined, wantSince)
	}
}

func TestJournalLogReader_buildArgs_ZeroTimeOmitted(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)

	args := r.buildArgs(ReadOpts{
		Limit: 50,
	})

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--since") {
		t.Errorf("args should not contain --since when From is zero, got: %q", joined)
	}
	if strings.Contains(joined, "--until") {
		t.Errorf("args should not contain --until when To is zero, got: %q", joined)
	}
}

func TestJournalLogReader_buildArgs_UTCTimestampFormat(t *testing.T) {
	runner := ocserv.NewFakeCommandRunner()
	runner.Results["journalctl"] = ocserv.CommandResult{Stdout: "", ExitCode: 0}
	r := NewJournalLogReader("ocserv", SourceOcserv, runner)

	from := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	to := time.Date(2026, 1, 15, 20, 45, 59, 0, time.UTC)

	_, _ = r.ReadLines(context.Background(), ReadOpts{Limit: 10, From: from, To: to})

	if len(runner.Calls) == 0 {
		t.Fatal("no calls recorded")
	}

	call := runner.Calls[len(runner.Calls)-1]
	foundSince := false
	foundUntil := false
	for _, arg := range call.Args {
		if arg == "--since=2026-01-15 08:30:00 UTC" {
			foundSince = true
		}
		if arg == "--until=2026-01-15 20:45:59 UTC" {
			foundUntil = true
		}
	}
	if !foundSince {
		t.Errorf("expected --since=2026-01-15 08:30:00 UTC in args %v", call.Args)
	}
	if !foundUntil {
		t.Errorf("expected --until=2026-01-15 20:45:59 UTC in args %v", call.Args)
	}
}
