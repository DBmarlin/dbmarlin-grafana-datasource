package plugin

import (
	"strings"
	"testing"
)

const eventRowsJSON = `[
 {"timeslice":"2026-11-03 08:30:00","event":"custom","hostid":0,"datasourceid":1,"entity":"pg16","name":"Release 6.11","object":"datasource","type":"Code Release","description":"canary"},
 {"timeslice":"2026-11-03 08:10:00","event":"alert","hostid":0,"datasourceid":1,"entity":"pg16","name":"High sessions","object":"datasource","type":"statistic","statistic":"activesessions","startedended":"started","threshold":"5","units":"","newvalue":"7.5","severity":2},
 {"timeslice":"2026-11-03 08:20:00","event":"change","hostid":0,"datasourceid":1,"entity":"pg16","name":"work_mem","object":"parameter","type":"modified","oldvalue":"4MB","newvalue":"8MB","severity":0}]`

func eventFake(t *testing.T) *fakeArchiver {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/datasource/host": `[{"id":1,"hostid":7}]`,
		restPath + "/v2/event":           eventRowsJSON,
	}
	return f
}

func TestEventsAreMappedToAnnotationFields(t *testing.T) {
	f := eventFake(t)
	resp := run(t, f, map[string]any{"queryType": "events", "instances": []string{"1"}})
	if resp.Error != nil || len(resp.Frames) != 1 {
		t.Fatalf("frames %d err %v", len(resp.Frames), resp.Error)
	}
	fr := resp.Frames[0]
	var names []string
	for _, fl := range fr.Fields {
		names = append(names, fl.Name)
	}
	if strings.Join(names, ",") != "time,title,text,tags" {
		t.Fatalf("fields %v", names)
	}
	// the instance and its host answer with the same rows, which are shown once,
	// oldest first
	if fr.Rows() != 3 {
		t.Fatalf("%d events, want 3", fr.Rows())
	}
	titles := []string{fr.Fields[1].At(0).(string), fr.Fields[1].At(1).(string), fr.Fields[1].At(2).(string)}
	if strings.Join(titles, "|") != "High sessions started|work_mem|Release 6.11" {
		t.Errorf("titles %v", titles)
	}
	if got := fr.Fields[2].At(0).(string); got != "activesessions = 7.5: threshold 5: (pg16)" {
		t.Errorf("alert text %q", got)
	}
	if got := fr.Fields[2].At(1).(string); got != "modified parameter: 4MB -> 8MB: (pg16)" {
		t.Errorf("change text %q", got)
	}
	if got := fr.Fields[3].At(0).(string); got != "alert,warning,pg16" {
		t.Errorf("alert tags %q", got)
	}
	if got := fr.Fields[3].At(2).(string); got != "custom,pg16" {
		t.Errorf("custom tags %q", got)
	}

	// one call for the instance, one for its host
	var own, hosts int
	for i, p := range f.paths {
		if p == restPath+"/v2/event" {
			q := f.queries[i]
			if q.Get("datasourceid") == "1" {
				own++
			}
			if q.Get("hostid") == "7" {
				hosts++
			}
			if q.Get("sort") != "timeslice asc" || q.Get("limit") != "1000" {
				t.Errorf("params %v", q)
			}
		}
	}
	if own != 1 || hosts != 1 {
		t.Errorf("own %d host %d calls", own, hosts)
	}
}

func TestEventKindsFilter(t *testing.T) {
	f := eventFake(t)
	resp := run(t, f, map[string]any{"queryType": "events", "instances": []string{"1"}, "eventKinds": []string{"custom"}})
	if resp.Frames[0].Rows() != 1 || resp.Frames[0].Fields[1].At(0) != "Release 6.11" {
		t.Errorf("only the custom event should remain, got %d", resp.Frames[0].Rows())
	}
	for _, q := range []map[string]any{
		{"queryType": "events"},
		{"queryType": "events", "instances": []string{"1"}, "eventKinds": []string{"nope"}},
	} {
		if r := run(t, f, q); r.Error == nil || r.Status != 400 {
			t.Errorf("%v: want 400, got %v", q, r.Status)
		}
	}
}
