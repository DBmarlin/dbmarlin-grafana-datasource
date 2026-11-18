package plugin

import (
	"strings"
	"testing"
)

const sqlCatalogue = `[
 {"statistic":"duration","units":"ms","collect":true},
 {"statistic":"executions","units":"","collect":true},
 {"statistic":"shared_blks_read","units":"","collect":true},
 {"statistic":"rows","units":"","collect":false}]`

const sqlStatRows = `[
 {"sqlhash":"-1694601452","sqltext":"UPDATE  t\nSET a = $1","executions":209970,"duration":134592.9,"avgduration":0.64,"shared_blks_read":5},
 {"sqlhash":"1303804285","sqltext":"UPDATE b","executions":50,"duration":26306.6,"avgduration":526.1,"shared_blks_read":null}]`

func sqlStatFake(t *testing.T) *fakeArchiver {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/sql/statistic/type": sqlCatalogue,
		restPath + "/v2/sql/statistic/sql":  sqlStatRows,
	}
	return f
}

func TestSQLStatisticsTable(t *testing.T) {
	f := sqlStatFake(t)
	frame, names := tableFor(t, f, map[string]any{"queryType": "sql_statistics", "instances": []string{"1"},
		"orderBy": "SHARED_BLKS_READ", "topN": 2, "statistics": []string{"executions"}})

	if got := strings.Join(names, ","); got != "SQL,SQL text,Executions,Duration,Avg duration,shared_blks_read,link" {
		t.Errorf("columns %s", got)
	}
	// the call goes to the SQL statistics endpoint with the database type, never to activity
	call := f.queries[len(f.queries)-1]
	if f.paths[len(f.paths)-1] != restPath+"/v2/sql/statistic/sql" || call.Get("type") != "postgresql" ||
		call.Get("sort") != "shared_blks_read desc" || call.Get("limit") != "2" || call.Get("id") != "1" {
		t.Errorf("params %v on %v", call, f.paths)
	}
	for _, p := range f.paths {
		if strings.Contains(p, "/activity/") {
			t.Errorf("unexpected activity call %s", p)
		}
	}
	if got := col(t, frame, "Duration").Config.Unit; got != "ms" {
		t.Errorf("duration unit %q", got)
	}
	if got := *col(t, frame, "Executions").At(0).(*float64); got != 209970 {
		t.Errorf("executions %v", got)
	}
	// a statistic not collected in the range stays empty, not zero
	if got := col(t, frame, "shared_blks_read").At(1).(*float64); got != nil {
		t.Errorf("want null, got %v", *got)
	}
	if got := col(t, frame, "SQL text").At(0).(string); got != "UPDATE t SET a = $1" {
		t.Errorf("sql text %q", got)
	}
	if link := col(t, frame, "link").At(0).(string); !strings.HasSuffix(link,
		"/instances/1/activity/statement/-1694601452/sql-statistics?fm=2026-11-03+08:00:00&to=2026-11-03+09:00:00&tz=UTC") {
		t.Errorf("link %s", link)
	}
}

func TestSQLStatisticsDefaultsToDuration(t *testing.T) {
	f := sqlStatFake(t)
	tableFor(t, f, map[string]any{"queryType": "sql_statistics", "instances": []string{"1"}})
	if got := f.queries[len(f.queries)-1].Get("sort"); got != "duration desc" {
		t.Errorf("sort %q", got)
	}
}

func TestSQLStatisticsWhitelistsNames(t *testing.T) {
	f := sqlStatFake(t)
	for _, q := range []map[string]any{
		{"orderBy": "rows"}, // not collected
		{"orderBy": "x; drop table sql; --"},
		{"statistics": []string{"nope"}},
	} {
		q["queryType"], q["instances"] = "sql_statistics", []string{"1"}
		resp := run(t, f, q)
		if resp.Error == nil || resp.Status != 400 {
			t.Errorf("%v: want 400, got %v %v", q, resp.Status, resp.Error)
		}
	}
	for _, p := range f.paths {
		if strings.HasSuffix(p, "/v2/sql/statistic/sql") {
			t.Error("a name outside the catalogue must never reach the archiver")
		}
	}
}

func TestSQLStatisticsMultipleInstances(t *testing.T) {
	f := sqlStatFake(t)
	frame, names := tableFor(t, f, map[string]any{"queryType": "sql_statistics", "instances": []string{"1", "3"}, "orderBy": "executions"})
	if names[0] != "Instance" || names[len(names)-1] != "instance_id" || frame.Fields[0].Len() != 4 {
		t.Fatalf("columns %v rows %d", names, frame.Fields[0].Len())
	}
	// ranked across instances, biggest first
	if *col(t, frame, "Executions").At(0).(*float64) < *col(t, frame, "Executions").At(3).(*float64) {
		t.Error("rows should be sorted by the ranking statistic, biggest first")
	}
	n := 0
	for _, p := range f.paths {
		if strings.HasSuffix(p, "/v1/sql/statistic/type") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("catalogue fetched %d times, want once per database type", n)
	}
}

func TestSQLStatisticsErrors(t *testing.T) {
	f := sqlStatFake(t)
	resp := run(t, f, map[string]any{"queryType": "sql_statistics", "instances": []string{"99"}})
	if resp.Error == nil || resp.Status != 400 {
		t.Errorf("unknown instance: want 400, got %v", resp.Status)
	}
	f.fail = map[string]int{restPath + "/v2/sql/statistic/sql": 401}
	resp = run(t, f, map[string]any{"queryType": "sql_statistics", "instances": []string{"1"}})
	if resp.Error == nil || resp.Status != 401 {
		t.Errorf("want 401, got %v %v", resp.Status, resp.Error)
	}
}
