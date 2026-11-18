package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

// CallResource serves the query editor's drop-downs and template variables.
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resources.CallResource(ctx, req, sender)
}

func newResourceHandler(d *Datasource) backend.CallResourceHandler {
	mux := http.NewServeMux()
	mux.HandleFunc("/instances", d.handleInstances)
	mux.HandleFunc("/statistics", d.handleStatistics)
	mux.HandleFunc("/sqlstatistics", d.handleSQLStatistics)
	mux.HandleFunc("/hosts", d.handleHosts)
	return httpadapter.New(mux)
}

type option struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

func (d *Datasource) handleInstances(w http.ResponseWriter, r *http.Request) {
	list, err := d.client.instances(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	options := make([]option, 0, len(list))
	for _, i := range list {
		options = append(options, option{Label: i.Name, Value: strconv.Itoa(i.ID)})
	}
	writeOptions(w, options)
}

func writeOptions(w http.ResponseWriter, options []option) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(options)
}

// handleStatistics lists the statistics collected for ?instance=<id>'s database
// type; the description is for the picker's hint.
func (d *Datasource) handleStatistics(w http.ResponseWriter, r *http.Request) {
	d.writeCatalogue(w, r, d.client.statisticCatalogue)
}

// handleSQLStatistics lists the SQL statistics collected for ?instance=<id>'s
// database type, for ranking and for extra columns.
func (d *Datasource) handleSQLStatistics(w http.ResponseWriter, r *http.Request) {
	d.writeCatalogue(w, r, d.client.sqlStatisticCatalogue)
}

// writeCatalogue answers with the statistic catalogue, fetched by catalogue,
// for the database type of ?instance=<id>.
func (d *Datasource) writeCatalogue(w http.ResponseWriter, r *http.Request,
	catalogue func(ctx context.Context, dbType string) ([]statisticType, error)) {
	dbType := d.client.instanceInfo(r.Context())[r.URL.Query().Get("instance")].Type
	if dbType == "" {
		http.Error(w, "unknown instance", http.StatusNotFound)
		return
	}
	statistics, err := catalogue(r.Context(), dbType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	options := make([]option, 0, len(statistics))
	for _, s := range statistics {
		options = append(options, option{Label: s.Statistic, Value: s.Statistic, Description: s.Description})
	}
	writeOptions(w, options)
}

// handleHosts lists the hosts of ?instance=<id>, or every host without it.
func (d *Datasource) handleHosts(w http.ResponseWriter, r *http.Request) {
	names := d.client.hostNames(r.Context())
	ids := make([]string, 0, len(names))
	if instance := r.URL.Query().Get("instance"); instance != "" {
		var err error
		if ids, err = d.client.hostIDs(r.Context(), &models.QueryModel{Instances: []string{instance}}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	} else {
		for id := range names {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}
	options := make([]option, 0, len(ids))
	for _, id := range ids {
		options = append(options, option{Label: firstNonEmpty(names[id], id), Value: id})
	}
	writeOptions(w, options)
}
