package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// fakeArchiver serves canned answers and records the last activity request.
type fakeArchiver struct {
	srv      *httptest.Server
	paths    []string
	queries  []url.Values
	activity string // JSON body for any /activity/time... path
	status   int    // status for activity paths, 200 when zero
	interval string // X-DBmarlin-Interval for activity paths, none when empty

	datasourceHits int // requests for the instance list

	routes map[string]string // body by request path, before the activity fallback
	fail   map[string]int    // status by request path
}

func newFakeArchiver(t *testing.T) *fakeArchiver {
	f := &fakeArchiver{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == restPath+"/v1/datasource" {
			f.datasourceHits++
			_, _ = w.Write([]byte(`[{"id":3,"name":"repo","type":"postgresql","hassqlstatistics":true},{"id":1,"name":"pg16","type":"postgresql","hassqlstatistics":true}]`))
			return
		}
		f.paths = append(f.paths, r.URL.Path)
		f.queries = append(f.queries, r.URL.Query())
		if code := f.fail[r.URL.Path]; code != 0 {
			w.WriteHeader(code)
			_, _ = w.Write([]byte("failed"))
			return
		}
		if body, ok := f.routes[r.URL.Path]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte("not found"))
			return
		}
		if f.interval != "" {
			w.Header().Set(intervalHeader, f.interval)
		}
		_, _ = w.Write([]byte(f.activity))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeArchiver) datasource(t *testing.T) *Datasource {
	settings := &models.PluginSettings{URL: f.srv.URL, MaxConcurrent: 2, Secrets: &models.SecretPluginSettings{}}
	client, err := NewClient(settings)
	if err != nil {
		t.Fatal(err)
	}
	return &Datasource{settings: settings, client: client}
}

func dataQuery(t *testing.T, qm map[string]any) backend.DataQuery {
	raw, err := json.Marshal(qm)
	if err != nil {
		t.Fatal(err)
	}
	to := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	return backend.DataQuery{
		RefID:     "A",
		JSON:      raw,
		Interval:  5 * time.Minute,
		TimeRange: backend.TimeRange{From: to.Add(-time.Hour), To: to},
	}
}

func TestUnknownQueryType(t *testing.T) {
	ds := newFakeArchiver(t).datasource(t)
	resp := ds.query(context.Background(), backend.PluginContext{}, dataQuery(t, map[string]any{"queryType": "nope"}))
	if resp.Error == nil || resp.Status != backend.StatusBadRequest {
		t.Fatalf("want bad request, got %v / %v", resp.Status, resp.Error)
	}
}
