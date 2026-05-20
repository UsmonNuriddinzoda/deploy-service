package handler

import (
	"deploy-service/db"
	"deploy-service/logger"
	"deploy-service/registry"
	"deploy-service/runner"
	"encoding/json"
	"errors"
	"net/http"
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
// Body: {"name":"bot","description":"Telegram Bot","script":"/opt/scripts/deploy-bot.sh"}
func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Script      string `json:"script"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Script == "" {
		jsonError(w, "fields 'name' and 'script' are required", http.StatusBadRequest)
		return
	}

	svc, err := h.repo.Create(req.Name, req.Description, req.Script)
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
// Body: {"description":"...","script":"..."}
func (h *Handler) updateService(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Description string `json:"description"`
		Script      string `json:"script"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Script == "" {
		jsonError(w, "field 'script' is required", http.StatusBadRequest)
		return
	}

	svc, err := h.repo.Update(name, req.Description, req.Script)
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
