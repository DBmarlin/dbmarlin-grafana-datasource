package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func newTestDatasource(t *testing.T, url, password string) *Datasource {
	t.Helper()
	jsonData, _ := json.Marshal(map[string]any{"url": url, "username": "grafana"})
	ds, err := NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{"password": password},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ds.(*Datasource)
}

func TestCheckHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		switch {
		case r.URL.Path != "/archiver/rest/v1/datasource":
			_, _ = w.Write([]byte("<html>nginx</html>"))
		case user != "grafana" || pass != "secret":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			_, _ = w.Write([]byte(`[{"id":1},{"id":2}]`))
		}
	}))
	defer server.Close()

	tests := []struct {
		name     string
		url      string
		password string
		status   backend.HealthStatus
		message  string
	}{
		{"ok", server.URL, "secret", backend.HealthStatusOk, "2 instances"},
		{"bad password", server.URL, "wrong", backend.HealthStatusError, "rejected the username or password"},
		{"not dbmarlin", server.URL + "/other", "secret", backend.HealthStatusError, "does not look like a DBmarlin server"},
		{"unreachable", "http://127.0.0.1:1", "secret", backend.HealthStatusError, "Cannot reach DBmarlin at http://127.0.0.1:1"},
		{"no url", "", "secret", backend.HealthStatusError, "Enter the DBmarlin URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := newTestDatasource(t, tt.url, tt.password).CheckHealth(context.Background(), &backend.CheckHealthRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != tt.status || !strings.Contains(res.Message, tt.message) {
				t.Errorf("got %v %q, want %v containing %q", res.Status, res.Message, tt.status, tt.message)
			}
		})
	}
}
