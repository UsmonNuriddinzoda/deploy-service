package handler

import (
	"deploy-service/db"
	"deploy-service/logger"
	"deploy-service/registry"
	"deploy-service/runner"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type Handler struct {
	log      *logger.Logger
	runner   *runner.Runner
	registry *registry.Registry
	repo     *db.ServiceRepo
}

func NewHandler(log *logger.Logger, r *runner.Runner, reg *registry.Registry, repo *db.ServiceRepo) *Handler {
	return &Handler{log: log, runner: r, registry: reg, repo: repo}
}

// ───────────────────────────── /services ─────────────────────────────

// ServicesHandler — роутер для /services и /services/{name}
func (h *Handler) ServicesHandler(w http.ResponseWriter, r *http.Request) {
	// /services или /services/
	name := strings.TrimPrefix(r.URL.Path, "/services")
	name = strings.Trim(name, "/")

	if name == "" {
		switch r.Method {
		case http.MethodGet:
			h.listServices(w, r)
		case http.MethodPost:
			h.createService(w, r)
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /services/{name}
	switch r.Method {
	case http.MethodGet:
		h.getService(w, r, name)
	case http.MethodPut:
		h.updateService(w, r, name)
	case http.MethodDelete:
		h.deleteService(w, r, name)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /services — список всех сервисов
func (h *Handler) listServices(w http.ResponseWriter, _ *http.Request) {
	list, err := h.registry.All()
	if err != nil {
		h.log.Error("listServices: %v", err)
		jsonError(w, "failed to fetch services", http.StatusInternalServerError)
		return
	}
	jsonOK(w, list)
}

// POST /services — создать сервис
// Body: {"name":"bot","description":"Telegram Bot","script":"/opt/scripts/deploy-bot.sh","container":"bot_container"}
func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Script      string `json:"script"`
		Container   string `json:"container"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Script == "" {
		jsonError(w, "fields 'name' and 'script' are required", http.StatusBadRequest)
		return
	}

	svc, err := h.repo.Create(req.Name, req.Description, req.Script, req.Container)
	if err != nil {
		h.log.Error("createService: %v", err)
		jsonError(w, "failed to create service: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.log.Info("Service created: name=%q script=%q", svc.Name, svc.Script)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(svc)
}

// GET /services/{name} — получить сервис по имени
func (h *Handler) getService(w http.ResponseWriter, _ *http.Request, name string) {
	svc, err := h.repo.GetByName(name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+name+"' not found", http.StatusNotFound)
			return
		}
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, svc)
}

// PUT /services/{name} — обновить сервис
// Body: {"description":"...","script":"...","container":"..."}
func (h *Handler) updateService(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Description string `json:"description"`
		Script      string `json:"script"`
		Container   string `json:"container"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Script == "" {
		jsonError(w, "field 'script' is required", http.StatusBadRequest)
		return
	}

	svc, err := h.repo.Update(name, req.Description, req.Script, req.Container)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+name+"' not found", http.StatusNotFound)
			return
		}
		h.log.Error("updateService: %v", err)
		jsonError(w, "failed to update service", http.StatusInternalServerError)
		return
	}
	h.log.Info("Service updated: name=%q script=%q", svc.Name, svc.Script)
	jsonOK(w, svc)
}

// DELETE /services/{name} — удалить сервис
func (h *Handler) deleteService(w http.ResponseWriter, _ *http.Request, name string) {
	if err := h.repo.Delete(name); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+name+"' not found", http.StatusNotFound)
			return
		}
		h.log.Error("deleteService: %v", err)
		jsonError(w, "failed to delete service", http.StatusInternalServerError)
		return
	}
	h.log.Info("Service deleted: name=%q", name)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "service '" + name + "' deleted"})
}

// ───────────────────────────── /deploy ─────────────────────────────

type DeployResponse struct {
	Service  string `json:"service"`
	Success  bool   `json:"success"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration string `json:"duration"`
	Message  string `json:"message,omitempty"`
}

// DeployHandler — POST /deploy/{service}
func (h *Handler) DeployHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed, use POST", http.StatusMethodNotAllowed)
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/deploy/")
	serviceName = strings.Trim(serviceName, "/")
	if serviceName == "" {
		jsonError(w, "service name is required: POST /deploy/{service}", http.StatusBadRequest)
		return
	}

	svc, err := h.registry.Get(serviceName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+serviceName+"' not found in registry", http.StatusNotFound)
			return
		}
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.log.Info("Deploy triggered: service=%q script=%q from %s", svc.Name, svc.Script, r.RemoteAddr)

	result, runErr := h.runner.Run(svc.Name, svc.Script)

	resp := DeployResponse{
		Service:  svc.Name,
		Success:  result.Success,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Duration: result.Duration.Round(time.Millisecond).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if result.Success {
		resp.Message = "deploy completed successfully"
		h.log.Info("Deploy of %q finished in %s", svc.Name, resp.Duration)
		w.WriteHeader(http.StatusOK)
	} else {
		resp.Message = "deploy failed"
		h.log.Error("Deploy of %q failed in %s: %v", svc.Name, resp.Duration, runErr)
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// ───────────────────────────── helpers ─────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ───────────────────────────── /logs ─────────────────────────────

// LogsStreamHandler — GET /logs/{service}/stream
// Стримит вывод `docker logs -f --tail=200 {container}` через SSE.
func (h *Handler) LogsStreamHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/logs/")
	path = strings.TrimSuffix(path, "/stream")
	serviceName := strings.Trim(path, "/")
	if serviceName == "" {
		jsonError(w, "service name is required", http.StatusBadRequest)
		return
	}

	svc, err := h.registry.Get(serviceName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+serviceName+"' not found", http.StatusNotFound)
			return
		}
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	container := svc.Container
	if container == "" {
		container = svc.Name
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ctx := r.Context()
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail=200", container)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: failed to start docker logs: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	send := func(stream, text string) {
		text = strings.ReplaceAll(text, "\n", "\\n")
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", stream, text)
		flusher.Flush()
	}

	done := make(chan struct{}, 2)
	readLines := func(rc interface{ Read([]byte) (int, error) }, stream string) {
		buf := make([]byte, 4096)
		var leftover string
		for {
			n, err := rc.Read(buf)
			if n > 0 {
				chunk := leftover + string(buf[:n])
				lines := strings.Split(chunk, "\n")
				leftover = lines[len(lines)-1]
				for _, l := range lines[:len(lines)-1] {
					if l != "" {
						send(stream, l)
					}
				}
			}
			if err != nil {
				if leftover != "" {
					send(stream, leftover)
				}
				break
			}
		}
		done <- struct{}{}
	}

	go readLines(stdout, "stdout")
	go readLines(stderr, "stderr")

	<-done
	<-done
	_ = cmd.Wait()

	_, _ = fmt.Fprintf(w, "event: done\ndata: {\"container\":\"%s\"}\n\n", container)
	flusher.Flush()
}

// ───────────────────────────── /status ─────────────────────────────

// StatusHandler — GET /status/{service}
// Возвращает статус Docker-контейнера.
func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/status/")
	serviceName = strings.Trim(serviceName, "/")
	if serviceName == "" {
		jsonError(w, "service name is required", http.StatusBadRequest)
		return
	}

	svc, err := h.registry.Get(serviceName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+serviceName+"' not found", http.StatusNotFound)
			return
		}
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	container := svc.Container
	if container == "" {
		container = svc.Name
	}

	// docker inspect --format
	out, err := exec.Command("docker", "inspect",
		"--format", `{"status":"{{.State.Status}}","running":{{.State.Running}},"started_at":"{{.State.StartedAt}}","finished_at":"{{.State.FinishedAt}}","exit_code":{{.State.ExitCode}},"image":"{{.Config.Image}}"}`,
		container,
	).Output()

	type StatusResp struct {
		Service    string `json:"service"`
		Container  string `json:"container"`
		Status     string `json:"status"`
		Running    bool   `json:"running"`
		StartedAt  string `json:"started_at"`
		FinishedAt string `json:"finished_at"`
		ExitCode   int    `json:"exit_code"`
		Image      string `json:"image"`
		Error      string `json:"error,omitempty"`
	}

	if err != nil {
		jsonOK(w, StatusResp{
			Service:   serviceName,
			Container: container,
			Status:    "not found",
			Error:     "container not found or docker error: " + err.Error(),
		})
		return
	}

	// Парсим вывод docker inspect
	var inner struct {
		Status     string `json:"status"`
		Running    bool   `json:"running"`
		StartedAt  string `json:"started_at"`
		FinishedAt string `json:"finished_at"`
		ExitCode   int    `json:"exit_code"`
		Image      string `json:"image"`
	}
	raw := strings.TrimSpace(string(out))
	if err := json.Unmarshal([]byte(raw), &inner); err != nil {
		jsonOK(w, StatusResp{Service: serviceName, Container: container, Status: "parse error", Error: err.Error()})
		return
	}

	jsonOK(w, StatusResp{
		Service:    serviceName,
		Container:  container,
		Status:     inner.Status,
		Running:    inner.Running,
		StartedAt:  inner.StartedAt,
		FinishedAt: inner.FinishedAt,
		ExitCode:   inner.ExitCode,
		Image:      inner.Image,
	})
}

// ───────────────────────────── /deploy/stream ─────────────────────────────

// DeployStreamHandler — GET /deploy/{service}/stream
// Стримит вывод скрипта в реальном времени через SSE.
func (h *Handler) DeployStreamHandler(w http.ResponseWriter, r *http.Request) {
	// Поддержка только GET
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем имя сервиса из /deploy/{service}/stream
	path := strings.TrimPrefix(r.URL.Path, "/deploy/")
	path = strings.TrimSuffix(path, "/stream")
	serviceName := strings.Trim(path, "/")
	if serviceName == "" {
		jsonError(w, "service name is required", http.StatusBadRequest)
		return
	}

	svc, err := h.registry.Get(serviceName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			jsonError(w, "service '"+serviceName+"' not found", http.StatusNotFound)
			return
		}
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// SSE заголовки
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	h.log.Info("Stream deploy triggered: service=%q script=%q", svc.Name, svc.Script)

	lines := make(chan runner.LineEvent, 64)
	start := time.Now()

	doneCh := make(chan struct {
		success bool
		err     error
	}, 1)
	go func() {
		success, runErr := h.runner.RunStream(svc.Name, svc.Script, lines)
		doneCh <- struct {
			success bool
			err     error
		}{success, runErr}
	}()

	// Читаем строки и отправляем клиенту
	for line := range lines {
		event := "stdout"
		if line.Stream == "stderr" {
			event = "stderr"
		}
		text := strings.ReplaceAll(line.Text, "\n", "\\n")
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, text)
		flusher.Flush()

		// Проверяем отключение клиента
		select {
		case <-r.Context().Done():
			return
		default:
		}
	}

	result := <-doneCh
	duration := time.Since(start).Round(time.Millisecond)

	status := "success"
	if !result.success {
		status = "error"
	}
	_, _ = fmt.Fprintf(w, "event: done\ndata: {\"success\":%v,\"duration\":\"%s\",\"status\":\"%s\"}\n\n",
		result.success, duration, status)
	flusher.Flush()

	h.log.Info("Stream deploy of %q finished in %s success=%v", svc.Name, duration, result.success)
}
