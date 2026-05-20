package runner

import (
	"bufio"
	"bytes"
	"context"
	"io"
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

// LineEvent — одна строка от скрипта
type LineEvent struct {
	Stream string // "stdout" или "stderr"
	Text   string
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

// RunStream запускает скрипт и отправляет строки в канал lines в реальном времени.
// Закрывает канал когда скрипт завершается.
func (r *Runner) RunStream(serviceName, scriptPath string, lines chan<- LineEvent) (bool, error) {
	m := r.getMutex(serviceName)
	if !m.TryLock() {
		lines <- LineEvent{Stream: "stderr", Text: "deploy of '" + serviceName + "' already in progress"}
		close(lines)
		return false, nil
	}
	defer m.Unlock()
	defer close(lines)

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	} else {
		cmd = exec.CommandContext(ctx, "bash", scriptPath)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return false, err
	}

	if err := cmd.Start(); err != nil {
		return false, err
	}

	var wg sync.WaitGroup
	pipe := func(stream string, rc io.ReadCloser) {
		defer wg.Done()
		sc := bufio.NewScanner(rc)
		for sc.Scan() {
			lines <- LineEvent{Stream: stream, Text: sc.Text()}
		}
	}

	wg.Add(2)
	go pipe("stdout", stdoutPipe)
	go pipe("stderr", stderrPipe)
	wg.Wait()

	err = cmd.Wait()
	return err == nil, err
}
