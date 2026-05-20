package runner

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

type Result struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
	Success  bool
}

type Runner struct {
	mu      sync.Map // map[serviceName]*sync.Mutex
	timeout time.Duration
}

func New(timeout time.Duration) *Runner {
	return &Runner{timeout: timeout}
}

func (r *Runner) getMutex(serviceName string) *sync.Mutex {
	val, _ := r.mu.LoadOrStore(serviceName, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// Run executes the given script for the named service.
// Only one deploy per service can run at a time.
func (r *Runner) Run(serviceName, scriptPath string) (*Result, error) {
	m := r.getMutex(serviceName)
	if !m.TryLock() {
		return &Result{
			Stderr:  "deploy of '" + serviceName + "' already in progress, please try again later",
			Success: false,
		}, nil
	}
	defer m.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	} else {
		cmd = exec.CommandContext(ctx, "bash", scriptPath)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Success:  err == nil,
	}, err
}
