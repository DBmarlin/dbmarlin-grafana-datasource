package plugin

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// sqlTextTruncated is how many characters the "truncated" SQL text setting keeps.
const sqlTextTruncated = 200

// dimensionTitles are the column titles; the key is also the archiver's name.
var dimensionTitles = map[string]string{
	"waitevent": "Wait event", "sqlhash": "SQL", "session": "Session", "username": "User",
	"database": "Database", "client": "Client", "program": "Program", "module": "Module",
}

// tableRow is one dimension value of one instance.
type tableRow struct {
	instance, instanceID string
	value, sqlText       string
	waitTime, waitCount  float64
	instanceTotal        float64 // the instance's total wait time, for the share
	link                 string
}

// activityTable is the top N dimension values by wait time over the dashboard's
// time range, with a link back to each in the DBmarlin UI.
func (d *Datasource) activityTable(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	ids, err := qm.InstanceIDs()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	dim := qm.GroupBy
	if dim == models.GroupByNone {
		dim = "waitevent"
	}
	if !validDimension(dim) {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("cannot list %q", dim))
	}

	info := d.client.instanceInfo(ctx)
	ln := links{base: d.settings.UIURL, from: q.TimeRange.From, to: q.TimeRange.To}
	topN := min(qm.TopN, models.MaxTopN)
	var rows []tableRow
	var notes []string
	for _, id := range ids {
		got, err := d.instanceRows(ctx, q.TimeRange, qm, id, dim, topN, ln)
		if err != nil {
			return d.errorResponse(err, false)
		}
		for i := range got {
			got[i].instance = instanceLabel(info, id)
			rows = append(rows, got[i])
		}
	}
	// Biggest first across instances.
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].waitTime > rows[j].waitTime })

	var resp backend.DataResponse
	resp.Frames = append(resp.Frames, d.tableFrame(rows, dim, q.TimeRange.To.Sub(q.TimeRange.From), ids, qm))
	if qm.TopN > models.MaxTopN {
		notes = append(notes, fmt.Sprintf("top N is limited to %d", models.MaxTopN))
	}
	for _, n := range notes {
		notice(&resp, n)
	}
	return resp
}

// instanceRows fetches the top values for one instance, each with the
// instance's total wait time for its share. There are deliberately no execution
// counts: activity is sampled once a second, and SQL statistics are saved
// against the normalised statement, which for most databases is not the raw
// statement activity sees, so the two cannot be joined by hash.
func (d *Datasource) instanceRows(ctx context.Context, tr backend.TimeRange, qm *models.QueryModel, id, dim string,
	topN int, ln links) ([]tableRow, error) {
	params := activityParams(tr, []string{id}, qm)

	var summary []map[string]any
	if err := d.client.Get(ctx, "/v1/activity/summary", params, &summary); err != nil {
		return nil, err
	}
	total := 0.0
	if len(summary) > 0 {
		total = num(summary[0]["waittime"])
	}

	path := "/v1/activity/" + dim
	if dim == "sqlhash" {
		path = "/v1/activity/sql"
	}
	params.Set("sort", "waittime desc")
	params.Set("limit", fmt.Sprint(topN))
	var raw []map[string]any
	if err := d.client.Get(ctx, path, params, &raw); err != nil {
		return nil, err
	}

	out := make([]tableRow, 0, len(raw))
	for _, r := range raw {
		row := tableRow{
			instanceID: id,
			value:      str(r[dim]),
			sqlText:    d.sqlText(str(r["sqltext"])),
			waitTime:   num(r["waittime"]),
			waitCount:  num(r["waitcount"]),

			instanceTotal: total,
		}
		if dim == "sqlhash" {
			row.link = ln.statement(id, row.value)
		} else {
			row.link = ln.activity(id, dim, row.value)
		}
		out = append(out, row)
	}

	return out, nil
}

// sqlText applies the data source's SQL text setting.
func (d *Datasource) sqlText(text string) string {
	if d.settings.SQLText == models.SQLTextNone {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if d.settings.SQLText == models.SQLTextTruncated && len([]rune(text)) > sqlTextTruncated {
		return string([]rune(text)[:sqlTextTruncated]) + " …"
	}
	return text
}

func (d *Datasource) tableFrame(rows []tableRow, dim string, window time.Duration, ids []string, qm *models.QueryModel) *data.Frame {
	n := len(rows)
	instances, instanceIDs, values, texts, links := make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	waitTime, share, aas, count, avg := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i, r := range rows {
		instances[i], instanceIDs[i], values[i], texts[i], links[i] = r.instance, r.instanceID, r.value, r.sqlText, r.link
		waitTime[i] = r.waitTime / 1000
		aas[i] = r.waitTime / float64(window.Milliseconds())
		count[i] = r.waitCount
		if r.waitCount > 0 {
			avg[i] = r.waitTime / r.waitCount
		}
		if r.instanceTotal > 0 {
			share[i] = 100 * r.waitTime / r.instanceTotal
		}
	}

	value := field(dimensionTitles[dim], values, "")
	value.Config.Links = []data.DataLink{{Title: "Open in DBmarlin", URL: "${__data.fields.link}", TargetBlank: true}}
	fields := []*data.Field{}
	if len(ids) > 1 {
		fields = append(fields, field("Instance", instances, ""))
	}
	fields = append(fields, value)
	if dim == "sqlhash" && d.settings.SQLText != models.SQLTextNone {
		fields = append(fields, field("SQL text", texts, ""))
	}
	fields = append(fields,
		field("Wait time", waitTime, "s"),
		field("% of total", share, "percent"),
		field("AAS", aas, "short"),
		field("Wait count", count, "short"),
		field("Avg wait", avg, "ms"))
	hidden := field("link", links, "")
	hidden.Config.Custom = map[string]any{"hidden": true}
	fields = append(fields, hidden)
	if len(ids) > 1 {
		// For data links to another dashboard; hidden like link.
		id := field("instance_id", instanceIDs, "")
		id.Config.Custom = map[string]any{"hidden": true}
		fields = append(fields, id)
	}

	frame := data.NewFrame("Top "+dimensionTitles[dim], fields...)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTable}
	return frame
}

func field[T any](name string, values []T, unit string) *data.Field {
	f := data.NewField(name, nil, values)
	f.Config = &data.FieldConfig{Unit: unit}
	return f
}
