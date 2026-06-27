package web

import (
	"net/http"
	"time"

	"git-statistics/internal/metrics"
	"git-statistics/internal/storage"
)

func parseFilter(r *http.Request) (storage.Filter, error) {
	f := storage.Filter{
		RepoSlug: r.URL.Query().Get("repo"),
		AuthorID: r.URL.Query().Get("author"),
	}
	if from := r.URL.Query().Get("from"); from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err != nil {
			return storage.Filter{}, err
		}
		f.From = t
	}
	if to := r.URL.Query().Get("to"); to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err != nil {
			return storage.Filter{}, err
		}
		f.To = t
	}
	return f, nil
}

type filterFormData struct {
	Repos          []string
	SelectedRepo   string
	SelectedAuthor string
	From           string
	To             string
}

func (h *Handler) filterForm(r *http.Request) filterFormData {
	q := r.URL.Query()
	return filterFormData{
		Repos:          h.Repos,
		SelectedRepo:   q.Get("repo"),
		SelectedAuthor: q.Get("author"),
		From:           q.Get("from"),
		To:             q.Get("to"),
	}
}

type activityPageData struct {
	Filter filterFormData
	Rows   []metrics.AuthorActivity
}

func (h *Handler) handleActivity(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := metrics.CommitsPerAuthor(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := activityPageData{Filter: h.filterForm(r), Rows: rows}
	if err := h.templates.ExecuteTemplate(w, "activity.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type deliveryFlowRow struct {
	Title            string
	CycleTimeHours   float64
	FirstReviewHours float64
	InReviewHours    float64
}

type deliveryFlowPageData struct {
	Filter             filterFormData
	Summary            *metrics.PRSummaryStats
	Distributions      *metrics.DistributionMetrics
	BreakdownsByRepo   []metrics.BreakdownRow
	BreakdownsByAuthor []metrics.BreakdownRow
	Rows               []deliveryFlowRow
}

func (h *Handler) handleDeliveryFlow(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	summary, err := metrics.SummaryStats(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	distributions, err := metrics.Distributions(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	repoBreakdown, err := metrics.BreakdownByRepository(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	authorBreakdown, err := metrics.BreakdownByAuthor(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	flows, err := metrics.DeliveryFlow(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows := make([]deliveryFlowRow, 0, len(flows))
	for _, f := range flows {
		rows = append(rows, deliveryFlowRow{
			Title:            f.Title,
			CycleTimeHours:   f.CycleTime.Hours(),
			FirstReviewHours: f.TimeToFirstReview.Hours(),
			InReviewHours:    f.TimeInReview.Hours(),
		})
	}

	data := deliveryFlowPageData{
		Filter:             h.filterForm(r),
		Summary:            summary,
		Distributions:      distributions,
		BreakdownsByRepo:   repoBreakdown,
		BreakdownsByAuthor: authorBreakdown,
		Rows:               rows,
	}
	if err := h.templates.ExecuteTemplate(w, "delivery_flow.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type churnPageData struct {
	Filter filterFormData
	Rows   []metrics.FileChurn
}

func (h *Handler) handleChurn(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := metrics.ChurnHotspots(h.Store, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := churnPageData{Filter: h.filterForm(r), Rows: rows}
	if err := h.templates.ExecuteTemplate(w, "churn.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	h.Scheduler.TriggerNow()
	redirectTo := r.Header.Get("Referer")
	if redirectTo == "" {
		redirectTo = "/activity"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}
