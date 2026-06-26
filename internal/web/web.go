package web

import (
	"embed"
	"html/template"
	"net/http"

	"git-statistics/internal/scheduler"
	"git-statistics/internal/storage"
)

//go:embed templates/*.html
var templateFS embed.FS

type Handler struct {
	Store     *storage.Store
	Scheduler *scheduler.Scheduler
	Repos     []string
	templates *template.Template
}

func NewHandler(store *storage.Store, sched *scheduler.Scheduler, repos []string) *Handler {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	return &Handler{Store: store, Scheduler: sched, Repos: repos, templates: tmpl}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/activity", http.StatusFound)
	})
	mux.HandleFunc("/activity", h.handleActivity)
	mux.HandleFunc("/delivery-flow", h.handleDeliveryFlow)
	mux.HandleFunc("/churn", h.handleChurn)
	mux.HandleFunc("/sync", h.handleSync)
	return mux
}
