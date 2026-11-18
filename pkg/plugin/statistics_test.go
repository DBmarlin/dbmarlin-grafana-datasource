package plugin

import (
	"context"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"strings"
	"testing"
	"time"
)

const catalogue = `[
 {"statistic":"activesessions","units":"","aggregation":"avg","collect":true},
 {"statistic":"blks_hit","units":"","aggregation":"sum","collect":true},
 {"statistic":"database_size","units":"MB","aggregation":"avg","collect":false}]`

const statRows = `[{"timeslice":"2026-11-03 08:00:00","sum":44.0,"avg":0.5},
                   {"timeslice":"2026-11-03 08:05:00","sum":31.0,"avg":0.25}]`

func statFake(t *testing.T) *fakeArchiver {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/statistic/type": catalogue,
		restPath + "/v1/statistic/time": statRows,
	}
	return f
}

func TestStatisticUsesAvgOrSumByAggregation(t *testing.T) {
	f := statFake(t)
	resp := run(t, f, map[string]any{"queryType": "db_statistic", "instances": []string{"1"},
		"statistics": []string{"activesessions", "BLKS_HIT"}})
	if resp.Error != nil || len(resp.Frames) != 2 {
		t.Fatalf("frames %d err %v", len(resp.Frames), resp.Error)
	}
	if got := resp.Frames[0].Fields[1].At(0).(*float64); *got != 0.5 {
		t.Errorf("avg statistic should use avg, got %v", *got)
	}
	if got := resp.Frames[1].Fields[1].At(0).(*float64); *got != 44 {
		t.Errorf("counter should use sum, got %v", *got)
	}
	if l := resp.Frames[0].Fields[1].Labels; l["instance"] != "pg16" || l["statistic"] != "activesessions" {
		t.Errorf("labels %v", l)
	}
	// the catalogue is asked for the instance's database type, once
	n := 0
	for _, p := range f.paths {
		if strings.HasSuffix(p, "/statistic/type") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("catalogue fetched %d times", n)
	}
}

func TestStatisticRejectsUncollectedAndUnknownNames(t *testing.T) {
	f := statFake(t)
	for _, name := range []string{"database_size", "x'; drop table statistic; --"} {
		resp := run(t, f, map[string]any{"queryType": "db_statistic", "instances": []string{"1"}, "statistics": []string{name}})
		if resp.Error == nil || resp.Status != 400 {
			t.Errorf("%q: want 400, got %v %v", name, resp.Status, resp.Error)
		}
	}
	for _, p := range f.paths {
		if strings.HasSuffix(p, "/statistic/time") {
			t.Error("a name outside the catalogue must never reach the archiver")
		}
	}
	resp := run(t, f, map[string]any{"queryType": "db_statistic", "instances": []string{"1"}})
	if resp.Error == nil || resp.Status != 400 {
		t.Errorf("no statistics: want 400, got %v", resp.Status)
	}
}

func TestStatisticUnits(t *testing.T) {
	for in, want := range map[string]string{"": "short", "%": "percent", "MB": "mbytes", "ms": "ms", "cs": "suffix:cs"} {
		if got := grafanaUnit(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

const hostRows = `[{"timeslice":"2026-11-03 08:00:00","cpuutilisation":30.5,"memoryutilisation":37,"ioutilisation":48,"diskutilisation":null},
                   {"timeslice":"2026-11-03 08:05:00","cpuutilisation":29,"memoryutilisation":37,"ioutilisation":40,"diskutilisation":null}]`

func hostFake(t *testing.T) *fakeArchiver {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/datasource/host":     `[{"id":1,"hostid":7}]`,
		restPath + "/v1/host":                `[{"id":7,"name":"docker-linux","password":"secret"}]`,
		restPath + "/v2/host/statistic/time": hostRows,
	}
	f.interval = "300000"
	return f
}

func TestHostMetricsFollowTheInstance(t *testing.T) {
	f := hostFake(t)
	resp := run(t, f, map[string]any{"queryType": "host_statistic", "instances": []string{"1"},
		"hostMetrics": []string{"cpuutilisation", "diskutilisation"}})
	if resp.Error != nil || len(resp.Frames) != 2 {
		t.Fatalf("frames %d err %v", len(resp.Frames), resp.Error)
	}
	cpu := resp.Frames[0].Fields[1]
	if l := cpu.Labels; l["host"] != "docker-linux" || l["metric"] != "cpuutilisation" || cpu.Config.Unit != "percent" {
		t.Errorf("labels %v unit %s", l, cpu.Config.Unit)
	}
	if *cpu.At(0).(*float64) != 30.5 {
		t.Errorf("cpu %v", *cpu.At(0).(*float64))
	}
	// a null metric stays a gap, not zero
	if resp.Frames[1].Fields[1].At(0).(*float64) != nil {
		t.Error("null disk utilisation should be a gap")
	}
	var hostCall string
	for i, p := range f.paths {
		if strings.HasSuffix(p, "/host/statistic/time") {
			hostCall = f.queries[i].Get("id")
		}
	}
	if hostCall != "7" {
		t.Errorf("queried host %q, want the instance's host 7", hostCall)
	}
}

func TestHostMetricsDefaultsAndErrors(t *testing.T) {
	f := hostFake(t)
	resp := run(t, f, map[string]any{"queryType": "host_statistic", "hosts": []string{"7"}})
	if resp.Error != nil || len(resp.Frames) != 4 {
		t.Fatalf("default is the four utilisations, got %d frames, %v", len(resp.Frames), resp.Error)
	}
	for _, q := range []map[string]any{
		{"queryType": "host_statistic"},
		{"queryType": "host_statistic", "hosts": []string{"x"}},
		{"queryType": "host_statistic", "hosts": []string{"7"}, "hostMetrics": []string{"drop"}},
	} {
		if r := run(t, f, q); r.Error == nil || r.Status != 400 {
			t.Errorf("%v: want 400, got %v", q, r.Status)
		}
	}
	f.routes[restPath+"/v1/datasource/host"] = `[]`
	if r := run(t, f, map[string]any{"queryType": "host_statistic", "instances": []string{"1"}}); r.Status != 404 {
		t.Errorf("no linked host: want 404, got %v", r.Status)
	}
}

// The archiver cuts every answer at 500 rows unless told otherwise, and sorts
// by time, so a long range lost its recent end. dataQuery is one hour at 5
// minutes: 12 buckets.
func TestTimeSeriesCallsAskForEnoughRows(t *testing.T) {
	sf, hf, af := statFake(t), hostFake(t), newFakeArchiver(t)
	af.activity = `[]`
	run(t, sf, map[string]any{"queryType": "db_statistic", "instances": []string{"1"}, "statistics": []string{"blks_hit"}})
	run(t, hf, map[string]any{"queryType": "host_statistic", "hosts": []string{"7"}})
	run(t, af, map[string]any{"instances": []string{"3"}})
	run(t, af, map[string]any{"instances": []string{"3"}, "groupBy": "waitevent", "topN": 7})

	find := func(f *fakeArchiver, suffix string) string {
		for i, p := range f.paths {
			if strings.HasSuffix(p, suffix) {
				return f.queries[i].Get("limit")
			}
		}
		return "none"
	}
	for name, got := range map[string]string{
		"statistic/time": find(sf, "/statistic/time"),
		"host time":      find(hf, "/host/statistic/time"),
		"activity/time":  find(af, "/v1/activity/time"),
	} {
		if got != "14" { // 12 buckets, plus the partial ones at either end
			t.Errorf("%s asked for limit %q, want 14", name, got)
		}
	}
	// grouped, limit means the number of values, not rows
	if got := af.queries[len(af.queries)-1].Get("limit"); got != "7" {
		t.Errorf("grouped limit %q, want the top N 7", got)
	}
}

func TestRowLimitCapAndNotice(t *testing.T) {
	f := statFake(t)
	f.routes[restPath+"/v1/statistic/time"] = statRows // 2 rows
	q := dataQuery(t, map[string]any{"queryType": "db_statistic", "instances": []string{"1"}, "statistics": []string{"blks_hit"}})
	q.TimeRange.From = q.TimeRange.To.Add(-5 * time.Minute) // 1 bucket + 2 = limit 3: not cut by 2 rows
	resp := f.datasource(t).query(context.Background(), backend.PluginContext{}, q)
	if n := len(resp.Frames[0].Meta.Notices); n != 0 {
		t.Errorf("unexpected notice: %v", resp.Frames[0].Meta.Notices)
	}
	q.TimeRange.From = q.TimeRange.To.Add(-time.Minute) // limit 2 = rows returned: possibly cut
	resp = f.datasource(t).query(context.Background(), backend.PluginContext{}, q)
	if len(resp.Frames[0].Meta.Notices) == 0 {
		t.Error("a full answer should warn that it may be cut")
	}
	hour := backend.TimeRange{From: q.TimeRange.To.Add(-time.Hour), To: q.TimeRange.To}
	if got := rowLimit(hour, time.Millisecond); got != maxRows {
		t.Errorf("limit %d, want the cap %d", got, maxRows)
	}
}
