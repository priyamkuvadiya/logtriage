package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var validLevels = map[string]bool{
	"DEBUG": true,
	"INFO":  true,
	"WARN":  true,
	"ERROR": true,
}

func triage(
	ctx context.Context,
	r io.Reader,
	w io.Writer,
	level string,
	total *atomic.Int64,
	flushEachLine bool,
) (int, error) {
	var flusher interface{ Flush() error }
	if flushEachLine {
		f, ok := w.(interface{ Flush() error })
		if !ok {
			return 0, errors.New("line flushing requested for a writer that cannot flush")
		}
		flusher = f
	}

	scanner := bufio.NewScanner(r)
	matches := 0

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return matches, err
		}

		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if _, err := time.Parse(time.RFC3339, fields[0]); err != nil {
			continue
		}
		if fields[1] != level {
			continue
		}

		if _, err := fmt.Fprintln(w, line); err != nil {
			return matches, err
		}
		matches++
		total.Add(1)

		if flusher != nil {
			if err := flusher.Flush(); err != nil {
				return matches, err
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return matches, err
	}
	return matches, scanner.Err()
}

func processFile(
	ctx context.Context,
	path string,
	w io.Writer,
	level string,
	total *atomic.Int64,
	flushEachLine bool,
) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return 0, pathErr.Err
		}
		return 0, err
	}

	matches, readErr := triage(ctx, f, w, level, total, flushEachLine)
	closeErr := f.Close()

	if readErr != nil {
		return matches, readErr
	}
	if closeErr != nil {
		return matches, closeErr
	}
	return matches, nil
}

type workerResult struct {
	err      error
	hadError bool
}

func worker(
	ctx context.Context,
	level string,
	paths []string,
	total *atomic.Int64,
	flushEachLine bool,
) workerResult {
	out := bufio.NewWriter(os.Stdout)
	hadError := false

	reportError := func(format string, args ...any) {
		if err := out.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "logtriage: stdout: %v\n", err)
			hadError = true
		}
		fmt.Fprintf(os.Stderr, format, args...)
		hadError = true
	}

	finishInterrupted := func(err error) workerResult {
		if flushErr := out.Flush(); flushErr != nil {
			fmt.Fprintf(os.Stderr, "logtriage: stdout: %v\n", flushErr)
			hadError = true
		}
		return workerResult{err: err, hadError: hadError}
	}

	if len(paths) == 0 {
		_, err := triage(ctx, os.Stdin, out, level, total, flushEachLine)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return finishInterrupted(err)
			}
			reportError("logtriage: stdin: %v\n", err)
			return workerResult{err: err, hadError: hadError}
		}
	} else {
		for _, path := range paths {
			_, err := processFile(ctx, path, out, level, total, flushEachLine)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return finishInterrupted(err)
				}
				reportError("logtriage: %s: %v\n", path, err)
			}
		}
	}

	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "logtriage: stdout: %v\n", err)
		hadError = true
	}

	return workerResult{hadError: hadError}
}

func run() int {
	levelFlag := flag.String("level", "ERROR", "log level to match")
	flag.Parse()

	level := strings.ToUpper(*levelFlag)
	if !validLevels[level] {
		fmt.Fprintf(os.Stderr,
			"logtriage: invalid level %q; use DEBUG, INFO, WARN, or ERROR\n",
			*levelFlag,
		)
		return 2
	}

	info, statErr := os.Stdout.Stat()
	flushEachLine := statErr == nil && info.Mode()&os.ModeCharDevice != 0

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	var total atomic.Int64
	done := make(chan workerResult, 1)
	paths := flag.Args()

	go func() {
		done <- worker(ctx, level, paths, &total, flushEachLine)
	}()

	select {
	case result := <-done:
		if ctx.Err() != nil || errors.Is(result.err, context.Canceled) {
			stop()
			fmt.Fprintf(os.Stderr, "logtriage: interrupted after %d matches\n", total.Load())
			return 130
		}
		if result.err != nil || result.hadError {
			return 2
		}
		if total.Load() == 0 {
			return 1
		}
		return 0

	case <-ctx.Done():
		stop()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		fmt.Fprintf(os.Stderr, "logtriage: interrupted after %d matches\n", total.Load())
		return 130
	}
}

func main() {
	os.Exit(run())
}
