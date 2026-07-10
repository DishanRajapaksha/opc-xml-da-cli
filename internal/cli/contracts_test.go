package cli

import (
	"context"
	"testing"
)

func TestSnapshotFormatContract(t *testing.T) {
	for _, format := range []string{"table", "text", "json", "csv"} {
		if err := validateSnapshotFormat(format); err != nil {
			t.Fatalf("snapshot format %q rejected: %v", format, err)
		}
	}
	if err := validateSnapshotFormat("jsonl"); err == nil {
		t.Fatal("snapshot commands must reject jsonl")
	}
}

func TestStreamFormatContract(t *testing.T) {
	for _, format := range []string{"text", "jsonl", "csv"} {
		if err := validateWatchFormat(format); err != nil {
			t.Fatalf("stream format %q rejected: %v", format, err)
		}
	}
	for _, format := range []string{"table", "json"} {
		if err := validateWatchFormat(format); err == nil {
			t.Fatalf("stream format %q must be rejected", format)
		}
	}
}

func TestSharedExitCodeContract(t *testing.T) {
	if got := mapRunError(context.DeadlineExceeded); got != exitTimeout {
		t.Fatalf("timeout exit code = %d, want %d", got, exitTimeout)
	}
	if exitRequestError != 4 || exitWriteRejected != 7 || exitTimeout != 8 || exitOutputError != 9 {
		t.Fatal("shared exit-code contract changed")
	}
}
