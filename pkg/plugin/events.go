package plugin

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// eventLimit is the most events one archiver call returns here.
const eventLimit = 1000

// severities are DBmarlin's alert severities: 0 OK, 1 Critical, 2 Warning, 3 Info.
var severities = []string{"ok", "critical", "warning", "info"}

// event is one row of /v2/event. Fields the archiver fills from the alert's
// properties arrive as strings, whatever they hold.
type event struct {
	Timeslice    string `json:"timeslice"`
	Event        string `json:"event"`
	HostID       int    `json:"hostid"`
	DatasourceID int    `json:"datasourceid"`
	Entity       string `json:"entity"`
	Name         string `json:"name"`
	Object       string `json:"object"`
	Type         string `json:"type"`
	Statistic    string `json:"statistic"`
	StartedEnded string `json:"startedended"`
	Severity     int    `json:"severity"`
	Threshold    string `json:"threshold"`
	Units        string `json:"units"`
	OldValue     string `json:"oldvalue"`
	NewValue     string `json:"newvalue"`
	Description  string `json:"description"`
}

// events shows DBmarlin alerts, detected changes and custom events (such as
// deployments) for the instances and their hosts as Grafana annotations.
func (d *Datasource) events(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	ids, err := qm.InstanceIDs()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	kinds, err := qm.Kinds()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}

	var rows []event
	truncated := false
	for _, id := range ids {
		hosts, err := d.client.hostIDs(ctx, &models.QueryModel{Instances: []string{id}})
		if err != nil {
			return d.errorResponse(err, false)
		}
		// The archiver ANDs the datasourceid and hostid filters, so an instance's
		// own events and its hosts' events are separate calls.
		calls := []url.Values{{"datasourceid": {id}}}
		if len(hosts) > 0 {
			calls = append(calls, url.Values{"hostid": hosts})
		}
		for _, params := range calls {
			got, err := d.client.eventRows(ctx, q.TimeRange, params)
			if err != nil {
				return d.errorResponse(err, false)
			}
			truncated = truncated || len(got) >= eventLimit
			rows = append(rows, got...)
		}
	}

	resp := backend.DataResponse{Frames: []*data.Frame{eventFrame(rows, kinds)}}
	if truncated {
		notice(&resp, fmt.Sprintf("showing the first %d events of a call; narrow the time range", eventLimit))
	}
	return resp
}

func (c *Client) eventRows(ctx context.Context, tr backend.TimeRange, params url.Values) ([]event, error) {
	params.Set("from", tr.From.UTC().Format(archiverTime))
	params.Set("to", tr.To.UTC().Format(archiverTime))
	params.Set("tz", "UTC")
	params.Set("sort", "timeslice asc")
	params.Set("limit", strconv.Itoa(eventLimit))
	var rows []event
	err := c.Get(ctx, "/v2/event", params, &rows)
	return rows, err
}

// eventFrame builds the frame Grafana maps to annotations by field name:
// time, title, text and tags. Events seen twice (two instances on one host)
// are shown once.
func eventFrame(rows []event, kinds []string) *data.Frame {
	wanted := map[string]bool{}
	for _, k := range kinds {
		wanted[k] = true
	}
	seen := map[event]bool{}
	var kept []event
	for _, e := range rows {
		if _, ok := parseSlice(e.Timeslice); ok && wanted[e.Event] && !seen[e] {
			seen[e] = true
			kept = append(kept, e)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Timeslice < kept[j].Timeslice })

	times := make([]time.Time, len(kept))
	titles := make([]string, len(kept))
	texts := make([]string, len(kept))
	tags := make([]string, len(kept))
	for i, e := range kept {
		times[i], _ = parseSlice(e.Timeslice)
		titles[i], texts[i], tags[i] = e.title(), e.text(), e.tags()
	}
	frame := data.NewFrame("events",
		data.NewField("time", nil, times),
		data.NewField("title", nil, titles),
		data.NewField("text", nil, texts),
		data.NewField("tags", nil, tags))
	return frame
}

func (e event) title() string {
	if e.Event == "alert" && e.StartedEnded != "" {
		return e.Name + " " + e.StartedEnded
	}
	return e.Name
}

func (e event) text() string {
	var parts []string
	switch e.Event {
	case "alert":
		if e.Statistic != "" && e.NewValue != "" {
			parts = append(parts, fmt.Sprintf("%s = %s%s", e.Statistic, e.NewValue, e.Units))
		}
		if e.Threshold != "" {
			parts = append(parts, "threshold "+e.Threshold+e.Units)
		}
	case "change":
		parts = append(parts, strings.TrimSpace(e.Type+" "+e.Object))
		if e.OldValue != "" || e.NewValue != "" {
			parts = append(parts, e.OldValue+" -> "+e.NewValue)
		}
	default: // custom
		if e.Type != "" {
			parts = append(parts, e.Type)
		}
	}
	if e.Description != "" {
		parts = append(parts, e.Description)
	}
	if e.Entity != "" {
		parts = append(parts, "("+e.Entity+")")
	}
	return strings.Join(parts, ": ")
}

// tags are comma separated, which Grafana splits into annotation tags.
func (e event) tags() string {
	tags := []string{e.Event}
	if e.Event == "alert" && e.Severity >= 0 && e.Severity < len(severities) {
		tags = append(tags, severities[e.Severity])
	}
	if e.Entity != "" {
		tags = append(tags, e.Entity)
	}
	return strings.Join(tags, ",")
}
