package plugin

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// sqlStatRow is one normalised statement of one instance. Statistics are saved
// against the normalised statement, so these rows are never joined to activity
// rows, which carry the raw statement and wait times.
type sqlStatRow struct {
	instance, instanceID string
	hash, text, link     string
	values               map[string]*float64 // by statistic name
	avgDuration          *float64
}

// sqlStatistics is the top N statements by a SQL statistic (duration by
// default) over the dashboard's time range, with a link back to each in the
// DBmarlin UI.
func (d *Datasource) sqlStatistics(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	ids, err := qm.InstanceIDs()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	extra, err := qm.SQLColumns()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}

	info := d.client.instanceInfo(ctx)
	ln := links{base: d.settings.UIURL, from: q.TimeRange.From, to: q.TimeRange.To}
	orderBy := qm.SQLOrderBy()
	topN := min(qm.TopN, models.MaxTopN)

	units := map[string]string{"executions": "", "duration": "ms"}
	var rows []sqlStatRow
	var notes []string
	var shown []string // the statistic columns, in display order
	for _, id := range ids {
		in, ok := info[id]
		if !ok {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("instance %s was not found", id))
		}
		if !in.HasSQLStatistics {
			notes = append(notes, fmt.Sprintf("DBmarlin does not collect SQL statistics for %s", in.Name))
			continue
		}
		catalogue, err := d.client.sqlStatisticCatalogue(ctx, in.Type)
		if err != nil {
			return d.errorResponse(err, false)
		}
		// Only names from the catalogue are forwarded: the archiver puts the
		// sort statistic into its SQL.
		order, ok := findStatistic(catalogue, orderBy)
		if !ok {
			return backend.ErrDataResponse(backend.StatusBadRequest,
				fmt.Sprintf("%q is not a SQL statistic DBmarlin collects for %s", orderBy, in.Type))
		}
		columns := []statisticType{order}
		for _, name := range extra {
			stat, ok := findStatistic(catalogue, name)
			if !ok {
				return backend.ErrDataResponse(backend.StatusBadRequest,
					fmt.Sprintf("%q is not a SQL statistic DBmarlin collects for %s", name, in.Type))
			}
			columns = append(columns, stat)
		}
		for _, s := range columns {
			if _, seen := units[s.Statistic]; !seen {
				units[s.Statistic] = grafanaUnit(s.Units)
			}
			shown = appendOnce(shown, s.Statistic)
		}

		got, err := d.statementRows(ctx, q.TimeRange, id, in, order.Statistic, topN, columns, ln)
		if err != nil {
			return d.errorResponse(err, false)
		}
		for i := range got {
			got[i].instance = instanceLabel(info, id)
		}
		rows = append(rows, got...)
	}

	// Biggest first across instances.
	sort.SliceStable(rows, func(i, j int) bool { return orderValue(rows[i], orderBy) > orderValue(rows[j], orderBy) })

	var resp backend.DataResponse
	resp.Frames = append(resp.Frames, d.sqlStatisticsFrame(rows, shown, units, len(ids) > 1))
	if qm.TopN > models.MaxTopN {
		notes = append(notes, fmt.Sprintf("top N is limited to %d", models.MaxTopN))
	}
	for _, n := range notes {
		notice(&resp, n)
	}
	return resp
}

// statementRows fetches the top statements of one instance.
func (d *Datasource) statementRows(ctx context.Context, tr backend.TimeRange, id string, in instance, order string,
	topN int, columns []statisticType, ln links) ([]sqlStatRow, error) {
	params := url.Values{}
	params.Set("from", tr.From.UTC().Format(archiverTime))
	params.Set("to", tr.To.UTC().Format(archiverTime))
	params.Set("tz", "UTC")
	params.Set("id", id)
	params.Set("type", in.Type)
	params.Set("sort", order+" desc")
	params.Set("limit", strconv.Itoa(topN))
	var raw []map[string]any
	if err := d.client.Get(ctx, "/v2/sql/statistic/sql", params, &raw); err != nil {
		return nil, err
	}

	out := make([]sqlStatRow, 0, len(raw))
	for _, r := range raw {
		row := sqlStatRow{
			instanceID: id,
			hash:       str(r["sqlhash"]),
			text:       d.sqlText(str(r["sqltext"])),
			values:     map[string]*float64{},
		}
		// Executions and duration are shown for every statement; the archiver
		// derives the average duration itself.
		for _, name := range append([]string{"executions", "duration"}, statNames(columns)...) {
			row.values[name] = optNum(r[name])
		}
		row.avgDuration = optNum(r["avgduration"])
		row.link = ln.statementTab(id, row.hash, "sql-statistics")
		out = append(out, row)
	}
	return out, nil
}

func (d *Datasource) sqlStatisticsFrame(rows []sqlStatRow, shown []string, units map[string]string, multi bool) *data.Frame {
	n := len(rows)
	instances, instanceIDs, hashes, texts, links := make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	avg := make([]*float64, n)
	for i, r := range rows {
		instances[i], instanceIDs[i], hashes[i], texts[i], links[i] = r.instance, r.instanceID, r.hash, r.text, r.link
		avg[i] = r.avgDuration
	}

	value := field("SQL", hashes, "")
	value.Config.Links = []data.DataLink{{Title: "Open in DBmarlin", URL: "${__data.fields.link}", TargetBlank: true}}
	fields := []*data.Field{}
	if multi {
		fields = append(fields, field("Instance", instances, ""))
	}
	fields = append(fields, value)
	if d.settings.SQLText != models.SQLTextNone {
		fields = append(fields, field("SQL text", texts, ""))
	}

	// Executions, duration and its average always come first.
	names := appendOnce(appendOnce(nil, "executions"), "duration")
	for _, s := range shown {
		names = appendOnce(names, s)
	}
	for _, name := range names {
		col := make([]*float64, n)
		for i, r := range rows {
			col[i] = r.values[name]
		}
		fields = append(fields, field(columnTitle(name), col, units[name]))
		if name == "duration" {
			fields = append(fields, field("Avg duration", avg, "ms"))
		}
	}

	hidden := field("link", links, "")
	hidden.Config.Custom = map[string]any{"hidden": true}
	fields = append(fields, hidden)
	if multi {
		id := field("instance_id", instanceIDs, "")
		id.Config.Custom = map[string]any{"hidden": true}
		fields = append(fields, id)
	}

	frame := data.NewFrame("Top SQL statistics", fields...)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTable}
	return frame
}

// columnTitle capitalises the two fixed columns and keeps the catalogue name
// of every other statistic, which is what DBmarlin shows for it.
func columnTitle(name string) string {
	switch name {
	case "executions":
		return "Executions"
	case "duration":
		return "Duration"
	}
	return name
}

func statNames(columns []statisticType) []string {
	names := make([]string, 0, len(columns))
	for _, c := range columns {
		names = append(names, c.Statistic)
	}
	return names
}

func appendOnce(list []string, name string) []string {
	for _, s := range list {
		if s == name {
			return list
		}
	}
	return append(list, name)
}

// orderValue is a row's value of the ranking statistic; missing sorts last.
func orderValue(r sqlStatRow, name string) float64 {
	for k, v := range r.values {
		if v != nil && strings.EqualFold(k, name) {
			return *v
		}
	}
	return -1
}

// optNum is num that keeps a null (a statistic not collected in the range) as
// nil instead of zero.
func optNum(v any) *float64 {
	if v == nil {
		return nil
	}
	f := num(v)
	return &f
}
