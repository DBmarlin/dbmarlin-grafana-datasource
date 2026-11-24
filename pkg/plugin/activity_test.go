package plugin

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const groupedRows = `[
 {"timeslice":"2026-11-03 08:00:00","waitevent":"WALSync","waittime":30000,"waitcount":30,"avgwaittime":1000},
 {"timeslice":"2026-11-03 08:00:00","waitevent":"other","waittime":60000,"waitcount":20,"avgwaittime":3000,"other":true},
 {"timeslice":"2026-11-03 08:05:00","waitevent":"WALSync","waittime":150000,"waitcount":150,"avgwaittime":1000},
 {"timeslice":"2026-11-03 08:05:00","waitevent":"other","waittime":3000,"waitcount":3,"avgwaittime":1000,"other":true}
]`

func run(t *testing.T, f *fakeArchiver, qm map[string]any) backend.DataResponse {
	t.Helper()
	return f.datasource(t).query(context.Background(), backend.PluginContext{}, dataQuery(t, qm))
}

func values(t *testing.T, f *data.Frame) []float64 {
	t.Helper()
	out := make([]float64, f.Fields[1].Len())
	for i := range out {
		out[i] = f.Fields[1].At(i).(float64)
	}
	return out
}

func TestGroupedRequestAndFrames(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = groupedRows
	resp := run(t, f, map[string]any{
		"instances": []string{"3"}, "groupBy": "waitevent", "topN": 1,
		"filters": map[string][]string{"database": {"a", "b"}, "bogus": {"x"}},
	})
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}

	if f.paths[0] != restPath+"/v1/activity/time/waitevent" {
		t.Errorf("path %s", f.paths[0])
	}
	q := f.queries[0]
	for k, want := range map[string]string{"tz": "UTC", "interval": "300000", "limit": "1", "other": "true",
		"from": "2026-11-03 08:00:00", "to": "2026-11-03 09:00:00", "id": "3"} {
		if got := q.Get(k); got != want {
			t.Errorf("%s=%q, want %q", k, got, want)
		}
	}
	if got := q["database"]; len(got) != 2 {
		t.Errorf("database should repeat, got %v", got)
	}
	if q.Has("bogus") {
		t.Error("unknown filter must not be forwarded")
	}

	// WALSync, then the remainder last and labelled so it can't be confused with a real value.
	if len(resp.Frames) != 2 {
		t.Fatalf("frames %d", len(resp.Frames))
	}
	if n := resp.Frames[0].Name; n != "WALSync" {
		t.Errorf("first frame %q", n)
	}
	other := resp.Frames[1]
	if other.Name != "Other" || other.Fields[1].Labels["other"] != "true" {
		t.Errorf("other frame %q %v", other.Name, other.Fields[1].Labels)
	}
	// AAS = waittime / 300000 ms
	if got := values(t, resp.Frames[0]); got[0] != 0.1 || got[1] != 0.5 {
		t.Errorf("AAS %v", got)
	}
	if got := resp.Frames[0].Fields[0].Config.Interval; got != 300000 {
		t.Errorf("interval %v", got)
	}
}

func TestAASUsesEffectiveInterval(t *testing.T) {
	f := newFakeArchiver(t)
	// Asked for 5s buckets but the archiver answered in 1 minute buckets (non-premium licence).
	f.activity = `[{"timeslice":"2026-11-03 08:01:00","waittime":60000,"waitcount":60},
	                {"timeslice":"2026-11-03 08:02:00","waittime":30000,"waitcount":30}]`
	q := dataQuery(t, map[string]any{"instances": []string{"3"}})
	q.Interval = 5 * time.Second
	resp := f.datasource(t).query(context.Background(), backend.PluginContext{}, q)
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if got := values(t, resp.Frames[0]); got[0] != 1 || got[1] != 0.5 {
		t.Errorf("AAS %v, want [1 0.5] (divided by 60 s, not 5 s)", got)
	}
	if f.queries[0].Get("interval") != "5000" {
		t.Errorf("requested interval %s", f.queries[0].Get("interval"))
	}
}

func TestUngroupedPerInstanceAndAggregated(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = `[{"timeslice":"2026-11-03 08:00:00","waittime":300000,"waitcount":1}]`

	resp := run(t, f, map[string]any{"instances": []string{"3", "1"}})
	if len(f.paths) != 2 || f.paths[0] != restPath+"/v1/activity/time" {
		t.Fatalf("paths %v", f.paths)
	}
	if len(resp.Frames) != 2 || resp.Frames[0].Name != "repo" || resp.Frames[1].Name != "pg16" {
		t.Fatalf("frames %+v", resp.Frames)
	}

	f.paths, f.queries = nil, nil
	resp = run(t, f, map[string]any{"instances": []string{"3", "1"}, "aggregateInstances": true})
	if len(f.paths) != 1 || len(f.queries[0]["id"]) != 2 || len(resp.Frames) != 1 {
		t.Fatalf("aggregate: paths %v ids %v frames %d", f.paths, f.queries[0]["id"], len(resp.Frames))
	}
	if _, ok := resp.Frames[0].Fields[1].Labels["instance"]; ok {
		t.Error("aggregated series must not carry an instance label")
	}
}

func TestMetrics(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = `[{"timeslice":"2026-11-03 08:00:00","waittime":90000,"waitcount":30},
	                {"timeslice":"2026-11-03 08:05:00","waittime":90000,"waitcount":30}]`
	for metric, want := range map[string]float64{"waittime": 90, "waitcount": 30, "avgwaittime": 3000, "aas": 0.3} {
		resp := run(t, f, map[string]any{"instances": []string{"3"}, "metric": metric})
		if got := values(t, resp.Frames[0])[0]; got != want {
			t.Errorf("%s = %v, want %v", metric, got, want)
		}
	}
}

func TestTopNCapNotice(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = groupedRows
	resp := run(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "waitevent", "topN": 500})
	if got := f.queries[0].Get("limit"); got != "50" {
		t.Errorf("limit %s, want 50", got)
	}
	if len(resp.Frames[0].Meta.Notices) != 1 {
		t.Error("expected a notice that top N was capped")
	}
}

func TestOtherOff(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = `[]`
	run(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "username", "includeOther": false})
	if got := f.queries[0].Get("other"); got != "false" {
		t.Errorf("other=%s", got)
	}
}

func TestErrors(t *testing.T) {
	f := newFakeArchiver(t)
	f.status = 404
	resp := run(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "waitevent"})
	if resp.Error == nil || resp.Error.Error() != "grouping needs DBmarlin 6.12 or later" {
		t.Errorf("grouped 404: %v", resp.Error)
	}
	if resp.ErrorSource != backend.ErrorSourceDownstream {
		t.Errorf("source %v", resp.ErrorSource)
	}

	resp = run(t, f, map[string]any{"instances": []string{"3"}})
	if resp.Error == nil || resp.Status != 404 {
		t.Errorf("ungrouped 404 should pass through: %v %v", resp.Status, resp.Error)
	}

	for name, qm := range map[string]map[string]any{
		"no instances": {},
		"bad instance": {"instances": []string{"$x"}},
		"bad group":    {"instances": []string{"3"}, "groupBy": "sql; drop"},
		"too many":     {"instances": make([]string, 21)},
	} {
		if r := run(t, f, qm); r.Error == nil || r.Status != backend.StatusBadRequest {
			t.Errorf("%s: want bad request, got %v %v", name, r.Status, r.Error)
		}
	}
}

func TestReportedIntervalBeatsGuess(t *testing.T) {
	f := newFakeArchiver(t)
	// One bucket that happens to sit on the hour: guessed from the rows alone
	// this looks like a forced 1 hour, so AAS would be 12x too low.
	f.activity = `[{"timeslice":"2026-11-03 08:00:00","waittime":90000,"waitcount":30}]`
	f.interval = "300000"
	resp := run(t, f, map[string]any{"instances": []string{"3"}})
	if got := values(t, resp.Frames[0])[0]; got != 0.3 {
		t.Errorf("AAS %v, want 0.3", got)
	}
	if got := resp.Frames[0].Fields[0].Config.Interval; got != 300000 {
		t.Errorf("field interval %v", got)
	}
}

func TestReportedIntervalWhenCoarsened(t *testing.T) {
	f := newFakeArchiver(t)
	// Asked for 5 s, the archiver says it used 1 hour: rows are not needed to tell.
	f.activity = `[{"timeslice":"2026-11-03 08:00:00","waittime":1800000,"waitcount":10}]`
	f.interval = "3600000"
	q := dataQuery(t, map[string]any{"instances": []string{"3"}})
	q.Interval = 5 * time.Second
	resp := f.datasource(t).query(context.Background(), backend.PluginContext{}, q)
	if got := values(t, resp.Frames[0])[0]; got != 0.5 {
		t.Errorf("AAS %v, want 0.5", got)
	}
}

func TestBadReportedIntervalFallsBack(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = `[{"timeslice":"2026-11-03 08:01:00","waittime":60000,"waitcount":60},
	                {"timeslice":"2026-11-03 08:02:00","waittime":30000,"waitcount":30}]`
	for _, bad := range []string{"abc", "0", "-5"} {
		f.interval = bad
		q := dataQuery(t, map[string]any{"instances": []string{"3"}})
		q.Interval = 5 * time.Second
		resp := f.datasource(t).query(context.Background(), backend.PluginContext{}, q)
		if got := values(t, resp.Frames[0]); got[0] != 1 {
			t.Errorf("header %q: AAS %v, want the guessed 1-minute result", bad, got)
		}
	}
}

func TestSeriesAreRankedAndColouredLikeTheUI(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = `[
	 {"timeslice":"2026-11-03 08:00:00","waitevent":"small","waittime":1000,"waitcount":1},
	 {"timeslice":"2026-11-03 08:00:00","waitevent":"big","waittime":9000,"waitcount":9},
	 {"timeslice":"2026-11-03 08:00:00","waitevent":"zzz","other":true,"waittime":50,"waitcount":1}]`
	resp := run(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "waitevent"})

	var names []string
	for _, fr := range resp.Frames {
		names = append(names, fr.Name)
	}
	// biggest first, as the UI lists them; the remainder last
	if got := strings.Join(names, ","); got != "big,small,Other" {
		t.Fatalf("order %s", got)
	}
	want := []string{waitColours[0], waitColours[1], otherColour}
	for i, fr := range resp.Frames {
		if got := fr.Fields[1].Config.Color["fixedColor"]; got != want[i] {
			t.Errorf("%s colour %v, want %s", fr.Name, got, want[i])
		}
	}

	// dimensions the UI has no palette for are left to Grafana
	resp = run(t, f, map[string]any{"instances": []string{"3"}, "groupBy": "username"})
	if resp.Frames[0].Fields[1].Config.Color != nil {
		t.Error("username series should not be coloured")
	}
}

func TestInstancesAreRankedAndColouredLikeTheInstancesScreen(t *testing.T) {
	f := newFakeArchiver(t)
	// "pg16" (id 1) has the most time, whichever order they are asked in
	f.activity = `[{"timeslice":"2026-11-03 08:00:00","waittime":1000,"waitcount":1}]`
	f.routes = map[string]string{restPath + "/v1/activity/time": f.activity}
	resp := run(t, f, map[string]any{"instances": []string{"3", "1"}})
	if len(resp.Frames) != 2 {
		t.Fatalf("frames %d", len(resp.Frames))
	}
	for i, fr := range resp.Frames {
		if got := fr.Fields[1].Config.Color["fixedColor"]; got != instanceColours[i] {
			t.Errorf("frame %d colour %v, want %s", i, got, instanceColours[i])
		}
	}

	// combined instances are a single series with no instance identity
	resp = run(t, f, map[string]any{"instances": []string{"3", "1"}, "aggregateInstances": true})
	if resp.Frames[0].Fields[1].Config.Color != nil {
		t.Error("aggregated series should not take an instance colour")
	}
}

func TestSeriesAndTablesCarryTheInstanceID(t *testing.T) {
	f := newFakeArchiver(t)
	f.activity = `[{"timeslice":"2026-11-03 08:00:00","waittime":1000,"waitcount":1}]`
	resp := run(t, f, map[string]any{"instances": []string{"3", "1"}})
	for i, want := range []string{"3", "1"} {
		if got := resp.Frames[i].Fields[1].Labels["instance_id"]; got != want {
			t.Errorf("frame %d instance_id %q, want %s", i, got, want)
		}
	}
	if _, ok := run(t, f, map[string]any{"instances": []string{"3", "1"}, "aggregateInstances": true}).Frames[0].Fields[1].Labels["instance_id"]; ok {
		t.Error("a combined series belongs to no single instance")
	}

	f.routes = map[string]string{
		restPath + "/v1/activity/summary":   summary,
		restPath + "/v1/activity/waitevent": waitRows,
	}
	frame, _ := tableFor(t, f, map[string]any{"instances": []string{"3", "1"}, "groupBy": "waitevent"})
	if got := col(t, frame, "instance_id").At(0).(string); got != "3" && got != "1" {
		t.Errorf("table instance_id %q", got)
	}
}

func TestActivitySummaryIsTheWholeRange(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{restPath + "/v1/activity/summary": summary} // 3600000 ms over the hour
	for metric, want := range map[string]float64{"aas": 1, "waittime": 3600} {
		resp := run(t, f, map[string]any{"queryType": "activity_summary", "instances": []string{"3"}, "metric": metric})
		if resp.Error != nil {
			t.Fatal(resp.Error)
		}
		fr := resp.Frames[0]
		if got := values(t, fr); len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want %v", metric, got, want)
		}
		if fr.Fields[1].Labels["instance_id"] != "3" {
			t.Errorf("labels %v", fr.Fields[1].Labels)
		}
	}
}
