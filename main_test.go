package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTriage(t *testing.T) {
	input := strings.Join([]string{
		"2026-11-08T10:00:00Z ERROR db timeout",
		"notatime ERROR x",
		"malformed",
		"2026-11-08T10:00:00Z error x",
	}, "\n") + "\n"

	var out bytes.Buffer
	var total atomic.Int64

	got, err := triage(
		context.Background(),
		strings.NewReader(input),
		&out,
		"ERROR",
		&total,
		false,
	)
	if err != nil {
		t.Fatalf("triage returned unexpected error: %v", err)
	}

	const wantOutput = "2026-11-08T10:00:00Z ERROR db timeout\n"
	if got != 1 {
		t.Errorf("matches = %d, want 1", got)
	}
	if out.String() != wantOutput {
		t.Errorf("output = %q, want %q", out.String(), wantOutput)
	}
	if total.Load() != 1 {
		t.Errorf("total matches = %d, want 1", total.Load())
	}
}

func TestTriageAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	var total atomic.Int64

	got, err := triage(
		ctx,
		strings.NewReader("2026-11-08T10:00:00Z ERROR db timeout\n"),
		&out,
		"ERROR",
		&total,
		false,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
	if got != 0 {
		t.Errorf("matches = %d, want 0", got)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want empty", out.String())
	}
}

func TestTriageFlushesEachLine(t *testing.T) {
	input := "2026-11-08T10:00:00Z ERROR db timeout\n"

	var out bytes.Buffer
	writer := bufio.NewWriter(&out)
	var total atomic.Int64

	got, err := triage(
		context.Background(),
		strings.NewReader(input),
		writer,
		"ERROR",
		&total,
		true,
	)
	if err != nil {
		t.Fatalf("triage returned unexpected error: %v", err)
	}

	const wantOutput = "2026-11-08T10:00:00Z ERROR db timeout\n"
	if got != 1 {
		t.Errorf("matches = %d, want 1", got)
	}
	if out.String() != wantOutput {
		t.Errorf("output = %q, want %q", out.String(), wantOutput)
	}
}
