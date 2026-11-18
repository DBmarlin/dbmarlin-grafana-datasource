package plugin

import (
	"strings"
	"testing"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const (
	waitRows = `[{"waitevent":"WALSync","waittime":1800000,"waitcount":1800},
	             {"waitevent":"active","waittime":900000,"waitcount":300}]`
	summary = `[{"summary":null,"waittime":3600000,"waitcount":5000}]`
	sqlRows = `[{"sqlhash":"-877168979","sqltext":"select  *\n  from orders","waittime":1800000,"waitcount":1800},
	            {"sqlhash":"2132704","sqltext":"END;","waittime":900000,"waitcount":900}]`
)

func tableFor(t *testing.T, f *fakeArchiver, qm map[string]any) (*data.Frame, []string) {
	t.Helper()
	if qm["queryType"] == nil {
		qm["queryType"] = "activity_table"
	}
	resp := run(t, f, qm)
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if len(resp.Frames) != 1 {
		t.Fatalf("frames %d", len(resp.Frames))
	}
	var names []string
	for _, fl := range resp.Frames[0].Fields {
		names = append(names, fl.Name)
	}
	return resp.Frames[0], names
}

func col(t *testing.T, f *data.Frame, name string) *data.Field {
	t.Helper()
	for _, fl := range f.Fields {
		if fl.Name == name {
			return fl
		}
	}
	t.Fatalf("no column %q in %v", name, f.Fields)
	return nil
}

func withType(q map[string]any) map[string]any {
	q["queryType"] = "activity_table"
	return q
}

func pluginCtx() backend.PluginContext { return backend.PluginContext{} }

func TestWaitEventTable(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/activity/summary":   summary,
		restPath + "/v1/activity/waitevent": waitRows,
	}
	frame, names := tableFor(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "waitevent", "topN": 2,
		"filters": map[string][]string{"database": {"d1"}}})

	if got := strings.Join(names, ","); got != "Wait event,Wait time,% of total,AAS,Wait count,Avg wait,link" {
		t.Errorf("columns %s", got)
	}
	// the list call sorts by wait time and limits to the top N
	list := f.queries[1]
	if list.Get("sort") != "waittime desc" || list.Get("limit") != "2" || list.Get("database") != "d1" {
		t.Errorf("list params %v", list)
	}
	// the share is of the instance total, not of what is shown
	if got := col(t, frame, "% of total").At(0).(float64); got != 50 {
		t.Errorf("share %v, want 50", got)
	}
	if got := col(t, frame, "Wait time").At(0).(float64); got != 1800 {
		t.Errorf("wait time %v s", got)
	}
	// AAS = wait time over the one-hour range
	if got := col(t, frame, "AAS").At(0).(float64); got != 0.5 {
		t.Errorf("AAS %v", got)
	}
	if got := col(t, frame, "Avg wait").At(1).(float64); got != 3000 {
		t.Errorf("avg wait %v ms", got)
	}

	link := col(t, frame, "link").At(0).(string)
	want := "/instances/3/activity?fm=2026-11-03+08:00:00&to=2026-11-03+09:00:00&tz=UTC&waits=WALSync"
	if !strings.HasSuffix(link, want) {
		t.Errorf("link %s", link)
	}
	if cfg := col(t, frame, "Wait event").Config; len(cfg.Links) != 1 || cfg.Links[0].URL != "${__data.fields.link}" {
		t.Errorf("data link %+v", cfg.Links)
	}
	if col(t, frame, "link").Config.Custom["hidden"] != true {
		t.Error("link column should be hidden from the table")
	}
}

func TestModuleLinkHasNoFilter(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/activity/summary": summary,
		restPath + "/v1/activity/module":  `[{"module":"m1","waittime":1000,"waitcount":1}]`,
	}
	frame, _ := tableFor(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "module"})
	if link := col(t, frame, "link").At(0).(string); strings.Contains(link, "m1") {
		t.Errorf("the UI cannot filter by module: %s", link)
	}
}

func TestSQLTable(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/activity/summary": summary,
		restPath + "/v1/activity/sql":     sqlRows,
	}
	frame, names := tableFor(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "sqlhash"})

	if got := strings.Join(names, ","); got != "SQL,SQL text,Wait time,% of total,AAS,Wait count,Avg wait,link" {
		t.Errorf("columns %s", got)
	}
	// Statistics are saved against the normalised statement, so they are never
	// mixed into activity rows.
	for _, p := range f.paths {
		if strings.Contains(p, "/sql/statistic") {
			t.Errorf("unexpected statistics call %s", p)
		}
	}
	if got := col(t, frame, "SQL text").At(0).(string); got != "select * from orders" {
		t.Errorf("sql text %q", got)
	}
	if link := col(t, frame, "link").At(0).(string); !strings.HasSuffix(link,
		"/instances/3/activity/statement/-877168979/activity?fm=2026-11-03+08:00:00&to=2026-11-03+09:00:00&tz=UTC") {
		t.Errorf("link %s", link)
	}
}

func TestSQLTextSetting(t *testing.T) {
	long := strings.Repeat("x", 300)
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/activity/summary": summary,
		restPath + "/v1/activity/sql":     `[{"sqlhash":"1","sqltext":"` + long + `","waittime":1000,"waitcount":1}]`,
	}
	q := map[string]any{"instances": []string{"3"}, "groupBy": "sqlhash"}

	ds := f.datasource(t)
	ds.settings.SQLText = models.SQLTextTruncated
	resp := ds.query(t.Context(), pluginCtx(), dataQuery(t, withType(q)))
	if got := col(t, resp.Frames[0], "SQL text").At(0).(string); len([]rune(got)) != sqlTextTruncated+2 || !strings.HasSuffix(got, " …") {
		t.Errorf("truncated text has %d chars", len([]rune(got)))
	}

	ds.settings.SQLText = models.SQLTextNone
	resp = ds.query(t.Context(), pluginCtx(), dataQuery(t, withType(q)))
	for _, fl := range resp.Frames[0].Fields {
		if fl.Name == "SQL text" {
			t.Error("SQL text column must be omitted when the setting is none")
		}
	}
}

func TestMultipleInstancesAreMergedBiggestFirst(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/activity/summary":   summary,
		restPath + "/v1/activity/waitevent": waitRows,
	}
	frame, names := tableFor(t, f, map[string]any{"instances": []string{"3", "1"}, "groupBy": "waitevent"})
	if names[0] != "Instance" || frame.Fields[0].Len() != 4 {
		t.Fatalf("columns %v rows %d", names, frame.Fields[0].Len())
	}
	if col(t, frame, "Wait time").At(0).(float64) < col(t, frame, "Wait time").At(3).(float64) {
		t.Error("rows should be sorted by wait time, biggest first")
	}
	if len(f.paths) != 4 {
		t.Errorf("want summary + list per instance, got %v", f.paths)
	}
}

func TestTableErrors(t *testing.T) {
	f := newFakeArchiver(t)
	f.fail = map[string]int{restPath + "/v1/activity/summary": 401}
	resp := run(t, f, map[string]any{"queryType": "activity_table", "instances": []string{"3"}})
	if resp.Error == nil || resp.Status != 401 {
		t.Errorf("want 401, got %v %v", resp.Status, resp.Error)
	}
	resp = run(t, f, map[string]any{"queryType": "activity_table", "instances": []string{"3"}, "groupBy": "x; y"})
	if resp.Error == nil || resp.Status != 400 {
		t.Errorf("want 400, got %v %v", resp.Status, resp.Error)
	}
}
