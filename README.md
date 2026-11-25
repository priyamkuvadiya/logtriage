# logtriage

A small Go CLI that filters log files down to the lines at one log level, so you can find the errors in a large log without scrolling through everything else.

## Install

```bash
go install github.com/priyamkuvadiya/logtriage@latest
```

This puts the `logtriage` binary in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

## Usage

Pass a file as an argument, or pipe input through stdin:

```bash
logtriage -level ERROR sample.log
logtriage -level ERROR < sample.log
cat sample.log | logtriage -level ERROR
```

Example output (using `sample.log` from this repo):

```text
2026-11-08T10:00:09Z ERROR db timeout
2026-11-08T10:00:15Z ERROR disk full
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-level` | `ERROR` | Log level to match: `DEBUG`, `INFO`, `WARN` or `ERROR` (case-insensitive, so `-level info` works); only lines at that level are printed |

## Development

```bash
go build ./... && go test ./... && go vet ./...
go run . -level ERROR sample.log
```
