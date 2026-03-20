# AGENTS.md - Linux Server Check

## Overview

This is a Go CLI tool for analyzing Linux server host anomalies. It checks system load, CPU, memory, disk, network, threads, and kernel parameters.

**Module:** `github.com/cuiwin/linux_server_check`  
**Go Version:** 1.21.11

## Build Commands

```bash
# Build the binary
go build -o server_check

# Run the application
./server_check

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o server_check

# Install dependencies
go mod download
go mod tidy
```

## Testing

**No unit tests exist in this codebase.** To add tests:
- Create `*_test.go` files in the same package
- Run a single test: `go test -v ./... -run TestName`
- Run all tests: `go test -v ./...`

## Code Style Guidelines

### Project Structure

```
checker/          # Business logic (loadchecker, memchecker, etc.)
  - *_checker.go  # One file per checker type
cmd/              # CLI commands (Cobra)
  - root.go      # Root command
  - check.go     # Parent check command
  - load.go      # Load check subcommand
reporter/         # Output formatting (kernelreporter, netreporter, etc.)
utils/            # Shared utilities
main.go           # Entry point
```

### Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Files | snake_case | `thread_checker.go`, `disk_io_reporter.go` |
| Structs | PascalCase | `ThreadChecker`, `KernelParams` |
| Fields | PascalCase | `CurrentThreads`, `UsedPercent` |
| Functions | PascalCase (exported), camelCase (unexported) | `GetThreadUsageInfo()`, `getPidMax()` |
| Interfaces | PascalCase with "er" suffix | Not currently used |
| Packages | snake_case or single word | `memchecker`, `utils` |

### Import Organization

Standard library imports first, then third-party, then internal:

```go
import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/fatih/color"
    "github.com/olekukonko/tablewriter"
    "github.com/shirou/gopsutil/v4"
    "github.com/spf13/cobra"

    "github.com/cuiwin/linux_server_check/utils"
)
```

Blank import for embedded files:
```go
import _ "embed"
```

### Error Handling Patterns

**Preferred patterns in this codebase:**

1. Return error up the stack (most common):
```go
func getPidMax() (int, error) {
    data, err := os.ReadFile("/proc/sys/kernel/pid_max")
    if err != nil {
        return 0, err
    }
    // ...
}
```

2. Print error and continue (for non-critical failures):
```go
if err != nil {
    fmt.Printf("获取kernel.pid_max失败: %v\n", err)
}
```

3. Wrap errors with context:
```go
return nil, fmt.Errorf("解析内核模板内容失败: %v", err)
```

4. Early return on error:
```go
if err != nil {
    return nil, err
}
// continue success path
```

### Comments

This codebase uses Chinese comments. Follow the existing pattern:

```go
// Checker 结构体定义
type ThreadChecker struct {}

// 读取/proc/sys/kernel/pid_max
func getPidMax() (int, error) {}
```

### Struct Definitions

```go
type KernelParams struct {
    PidMax         int    `json:"pid_max"`
    ThreadsMax     int    `json:"threads_max"`
    CurrentThreads int    `json:"current_threads"`
    UsedPercent    string
    Status         string
    Desc           string
}
```

### Embed Patterns

For embedded resources:
```go
//go:embed kernel_template.txt
var templateContent []byte
```

## Key Dependencies

- **github.com/spf13/cobra** - CLI framework
- **github.com/shirou/gopsutil/v4** - System information
- **github.com/olekukonko/tablewriter** - Table output
- **github.com/fatih/color** - Colored terminal output

## Command Registration Pattern

Commands are registered in `init()` functions:

```go
func init() {
    rootCmd.AddCommand(checkCmd)
    checkCmd.AddCommand(loadCmd)
    checkCmd.AddCommand(memCmd)
    checkCmd.AddCommand(diskCmd)
    checkCmd.AddCommand(threadCmd)
}
```

## Common Tasks

### Adding a New Checker

1. Create `checker/<name>checker/<name>_checker.go`
2. Define a `*Checker` struct
3. Implement `GetDetail()` and `PrintDetail()` methods
4. Add a new command in `cmd/` that uses the checker

### Adding a New Reporter

1. Create `reporter/<name>reporter/<name>_reporter.go`
2. Define data structures for the report
3. Implement reporting logic
4. Use `tabwriter` for aligned output

## Formatting

Use `go fmt` before committing:
```bash
go fmt ./...
```
