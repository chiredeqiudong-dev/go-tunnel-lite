# Go-Tunnel-Lite Agent Guidelines

This document provides guidelines for AI agents working on the Go-Tunnel-Lite project, a lightweight Go-based tunneling tool for exposing internal services to the public internet.

## Build System

### Make Commands
```bash
# Build all binaries (server and client)
make build

# Build server only
make server

# Build client only
make client

# Run all tests
make test

# Run server in development mode
make run-server

# Run client in development mode
make run-client

# Clean build artifacts
make clean

# Show help
make help
```

### Go Commands
```bash
# Run all tests with verbose output
go test ./... -v

# Run tests for a specific package
go test ./internal/client -v

# Run a single test
go test ./internal/client -v -run TestClientAuthSuccess

# Build server binary directly
go build -o bin/go-tunnel-server ./cmd/server

# Build client binary directly
go build -o bin/go-tunnel-client ./cmd/client

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Get dependencies
go mod tidy
```

## Code Style Guidelines

### Project Structure
```
go-tunnel-lite/
├── cmd/                    # Entry points
│   ├── client/            # Client entry point
│   └── server/            # Server entry point
├── internal/              # Internal packages (not for external use)
│   ├── client/            # Client core logic
│   ├── server/            # Server core logic
│   └── pkg/               # Shared internal packages
│       ├── config/        # Configuration parsing
│       ├── connect/       # Connection management
│       ├── log/           # Logging module
│       └── proto/         # Communication protocol
├── configs/               # Configuration files
├── bin/                   # Compiled binaries
└── docs/                  # Documentation
```

### Imports
- Use standard library imports first, then third-party imports, then internal imports
- Group imports with blank lines between groups
- Use absolute import paths for internal packages (github.com/chiredeqiudong-dev/go-tunnel-lite/internal/...)

Example:
```go
import (
    "fmt"
    "net"
    "sync"
    "time"

    "gopkg.in/yaml.v3"

    "github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
    "github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
)
```

### Naming Conventions
- **Packages**: Use lowercase, single-word names (e.g., `config`, `log`, `proto`)
- **Types**: Use PascalCase (e.g., `ServerConfig`, `ClientSession`)
- **Interfaces**: Use PascalCase ending with "er" when appropriate (e.g., `Reader`, `Writer`)
- **Variables**: Use camelCase (e.g., `serverAddr`, `heartbeatInterval`)
- **Constants**: Use PascalCase for exported constants, camelCase for internal (e.g., `MessageTypeAuth`, `maxRetries`)
- **Methods**: Use PascalCase for exported methods, camelCase for internal
- **Test files**: Use `_test.go` suffix (e.g., `server_test.go`)
- **Test functions**: Prefix with `Test` and use PascalCase (e.g., `TestClientAuthSuccess`)

### Error Handling
- Always check errors and handle them appropriately
- Use `fmt.Errorf` with `%w` for wrapping errors
- Return errors from functions rather than logging and continuing
- Use descriptive error messages that explain what went wrong

Example:
```go
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }
    // ...
}
```

### Logging
- Use the project's logging module (`internal/pkg/log`)
- Log levels: `debug`, `info`, `warn`, `error`
- Include context in log messages (e.g., `log.Info("正在连接服务端", "addr", addr)`)
- Use structured logging with key-value pairs

### Concurrency
- Use `sync.Mutex` for protecting shared state
- Use `sync.RWMutex` when reads are more frequent than writes
- Use `sync.WaitGroup` for coordinating goroutines
- Always defer mutex unlocks
- Use channels for communication between goroutines

Example:
```go
type Server struct {
    sessions   map[string]*ClientSession
    sessionsMu sync.RWMutex
    wg         sync.WaitGroup
}

func (s *Server) addSession(id string, session *ClientSession) {
    s.sessionsMu.Lock()
    defer s.sessionsMu.Unlock()
    s.sessions[id] = session
}
```

### Testing
- Write table-driven tests for multiple test cases
- Use `t.Run()` for subtests
- Mock dependencies when testing complex interactions
- Clean up resources in tests (use `defer`)
- Test both success and failure cases

Example:
```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name: "valid config",
            config: &Config{Addr: "localhost:8080"},
            wantErr: false,
        },
        {
            name: "missing address",
            config: &Config{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Configuration
- Use YAML for configuration files
- Configuration structs should have YAML tags
- Validate configuration on load
- Provide sensible defaults

Example:
```go
type ServerSettings struct {
    ControlAddr       string        `yaml:"control_addr"`
    Token             string        `yaml:"token"`
    HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
    PublicPorts       []int         `yaml:"public_ports"`
}
```

### Protocol Design
- Use custom binary protocol for client-server communication
- Define message types as constants
- Serialize/deserialize using binary encoding
- Include message length and type in protocol headers

### Documentation
- Use Go doc comments for exported types and functions
- Include Chinese comments for Chinese-speaking developers (this project has Chinese documentation)
- Keep comments concise and focused on the "why" not the "what"
- Update documentation when code changes

### Code Quality
- Run `go fmt` before committing
- Run `go vet` to catch common issues
- Ensure all tests pass before making changes
- Follow the existing code patterns in the codebase

## Development Workflow

1. **Understand the architecture**: Client-server model with custom binary protocol
2. **Check dependencies**: Only `gopkg.in/yaml.v3` for configuration parsing
3. **Run tests**: Always run tests after making changes
4. **Build binaries**: Verify both server and client compile
5. **Test integration**: Run server and client together to verify functionality

## Common Patterns

### Connection Management
- Use `net.Conn` for TCP connections
- Wrap connections with the `connect.Connect` type for message handling
- Handle connection errors gracefully
- Implement heartbeat mechanism for connection health

### Message Handling
- Use the `proto` package for message serialization/deserialization
- Handle different message types with switch statements
- Ensure messages are properly framed with length prefixes

### Configuration Loading
- Load from YAML files using `config.LoadServerConfig` or `config.LoadClientConfig`
- Validate configuration before use
- Provide default values for optional fields

### Data Forwarding
- Use `io.Copy` for socket-to-socket data transfer (recommended)
- Avoid `io.CopyBuffer` for socket-to-socket transfers (prevents zero-copy optimizations)
- Go's `io.Copy` automatically uses Linux `splice(2)` system call for TCP connections
- Zero-copy technique eliminates data copying between kernel and user space
- Results in ~80% reduction in memory allocation and ~3% performance improvement
- Cross-platform compatible: automatically falls back to standard copy on non-Linux systems

Example:
```go
// ✅ Recommended: Zero-copy forwarding
go func() {
    io.Copy(remote, local)
}()
go func() {
    io.Copy(local, remote)
}()

// ❌ Avoid: Prevents zero-copy optimizations
buf := make([]byte, 128*1024)
go func() {
    io.CopyBuffer(remote, local, buf)
}()
```

## Security Considerations

- Always validate input (ports, addresses, tokens)
- Use token-based authentication
- Implement port whitelisting on server
- Handle connection timeouts and limits
- Log security-relevant events

## Performance Guidelines

- Use connection pooling where appropriate
- Implement backoff for reconnection attempts
- Use buffered channels for high-throughput scenarios
- Profile before optimizing
- **Use `io.Copy` for data forwarding instead of `io.CopyBuffer`**
- **Leverage Go's standard library optimizations (e.g., Linux splice system call)**
- **Prefer zero-copy techniques when transferring data between sockets**

## Troubleshooting

If tests fail:
1. Check if mock servers are properly cleaned up
2. Verify port availability for tests
3. Check for race conditions in concurrent code
4. Run tests with `-race` flag to detect data races

If build fails:
1. Run `go mod tidy` to ensure dependencies are correct
2. Check Go version compatibility (requires Go 1.25+)
3. Verify all required files are present

If zero-copy optimization shows no benefit:
1. Verify you are on Linux (splice is Linux-specific)
2. Use `io.Copy` instead of `io.CopyBuffer`
3. Check that both endpoints are TCP sockets
4. Run performance benchmarks to verify improvements

## Project Philosophy: Lightweight & High Performance

This project follows a **minimalist design philosophy** with these core principles:

### Lightweight Principles
- **Keep it simple**: Avoid over-engineering and unnecessary complexity
- **Minimal dependencies**: Only use third-party packages when absolutely necessary (currently only `gopkg.in/yaml.v3`)
- **Single binary**: Compile to a single executable for easy deployment
- **Small footprint**: Optimize for memory usage and binary size
- **No bloat features**: Focus on core functionality, avoid feature creep

### High Performance Principles
- **Performance first**: Prioritize performance over convenience in critical paths
- **Zero allocation**: Prefer zero-allocation patterns in hot paths
- **Efficient data structures**: Use maps for O(1) lookups, avoid linear searches
- **Binary protocols**: Use efficient binary encoding instead of text-based formats
- **Connection reuse**: Implement connection pooling and keep-alive mechanisms
- **Batch processing**: Batch operations to reduce overhead
- **Zero-copy data forwarding**: Use `io.Copy` for socket-to-socket data transfer (leverages Linux splice)
- **Standard library optimizations**: Trust Go's standard library to use optimal system calls

### What to Avoid
❌ **Avoid**: Unnecessary abstractions and indirections
❌ **Avoid**: Heavy-weight frameworks that add overhead
❌ **Avoid**: Feature bloat that compromises performance
❌ **Avoid**: Complex dependency trees
❌ **Avoid**: Premature optimization without profiling
❌ **Avoid**: Adding features that can be handled by external tools

### What to Encourage
✅ **Encourage**: Simple, direct solutions
✅ **Encourage**: Leverage Go's standard library
✅ **Encourage**: Write efficient algorithms first
✅ **Encourage**: Profile and measure performance
✅ **Encourage**: Keep code readable but fast
✅ **Encourage**: Remove code that doesn't add value
✅ **Encourage**: Optimize for the common case

### Decision Framework
When making design decisions, ask:
1. **Is it necessary?** Does it solve a real problem?
2. **Is it simple?** Can it be implemented without complexity?
3. **Is it performant?** Does it impact critical paths?
4. **Is it lightweight?** Does it add minimal overhead?
5. **Can users do without it?** Can external tools handle this?

If the answer to any of these questions is "no", consider removing or simplifying the feature.

## Notes for AI Agents

- This is a Chinese-developed project with Chinese documentation
- The codebase uses a mix of English and Chinese comments
- Follow existing patterns rather than introducing new conventions
- When in doubt, look at similar code in the codebase
- The project aims to be lightweight with minimal dependencies
- **Always prioritize simplicity and performance over features**
- **Think twice before adding new features or dependencies**
- **Less is more: if you can remove code, do it**

## Zero-Copy Optimization

### Overview
The project has implemented zero-copy data forwarding using Linux's `splice(2)` system call through Go's standard `io.Copy` function. This optimization significantly reduces memory allocation and improves performance.

### Key Points

1. **Use `io.Copy` for Data Forwarding**
   - Automatically leverages Linux `splice(2)` system call
   - Transfers data directly between kernel sockets
   - No user-space memory allocation required

2. **Avoid `io.CopyBuffer` for Socket Transfers**
   - Custom buffers prevent zero-copy optimizations
   - Increases memory allocation by ~4x
   - Only use buffers for non-socket streams

3. **Performance Results** (Linux platform)
   - Memory allocation: -79.75% (164 KB → 33 KB per operation)
   - Performance: +3.44% improvement
   - System calls: ~75% reduction

4. **Cross-Platform Compatibility**
   - Linux: Uses `splice(2)` for optimal performance
   - Windows/macOS: Automatically falls back to standard copy
   - No performance penalty on non-Linux systems

5. **Implementation Pattern**
```go
// ✅ Correct: Zero-copy forwarding
func forwardData(src, dst net.Conn) {
    go func() {
        io.Copy(dst, src)  // Uses splice on Linux
    }()
    go func() {
        io.Copy(src, dst)  // Uses splice on Linux
    }()
}

// ❌ Avoid: Prevents zero-copy
func forwardDataWithBuffer(src, dst net.Conn) {
    buf := make([]byte, 128*1024)
    go func() {
        io.CopyBuffer(dst, src, buf)  // No splice optimization
    }()
    go func() {
        io.CopyBuffer(src, dst, buf)  // No splice optimization
    }()
}
```

### Technical Details

**splice(2) System Call**:
- Allows data to move between two file descriptors without user-space copying
- Requires Linux kernel 2.6.17 or later
- Only works with pipes, sockets, and certain file types

**Go Standard Library**:
- `io.Copy` detects `WriterTo` interface on `*net.TCPConn`
- Linux TCP connections implement `WriterTo` using `splice(2)`
- Fallback to standard copy for unsupported types or platforms

### References
- Implementation: `internal/client/client.go` (proxyData function)
- Tests: `internal/client/zerocopy_test.go`
- Server proxy: `internal/server/proxy.go`
- Performance report: `PERFORMANCE_REPORT.md`
- Detailed summary: `ZERO_COPY_SUMMARY.md`