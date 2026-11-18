package plugin

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// archiverTime is the archiver's from/to format; the plugin always sends tz=UTC.
const archiverTime = "2006-01-02 15:04:05"

// filterParams are the filters passed through to the archiver as repeated
// parameters. Anything else in the query is ignored, not forwarded.
var filterParams = []string{
	"waitevent", "database", "username", "client", "program", "module", "session", "sqlhash", "tag",
}

// request is one archiver call: either a single instance or, when instances
// are aggregated, all of them.
type request struct {
	ids   []string
	label string // instance label for the series, "" when aggregated
}

func (d *Datasource) activityTimeseries(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	ids, err := qm.InstanceIDs()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	grouped := qm.GroupBy != models.GroupByNone
	if grouped && !validDimension(qm.GroupBy) {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("cannot group by %q", qm.GroupBy))
	}

	info := d.client.instanceInfo(ctx)
	requests := []request{{ids: ids}}
	if !qm.AggregateInstances {
		requests = requests[:0]
		for _, id := range ids {
			requests = append(requests, request{ids: []string{id}, label: instanceLabel(info, id)})
		}
	}

	interval := queryInterval(q)
	var resp backend.DataResponse
	var totals []float64 // wait time per instance, to rank them by
	topN := min(qm.TopN, models.MaxTopN)
	for _, r := range requests {
		rows, used, err := d.client.activityRows(ctx, q.TimeRange, interval, qm, r, topN)
		if err != nil {
			return d.errorResponse(err, grouped)
		}
		frames := buildFrames(rows, qm, r.label, interval, used)
		if len(r.ids) == 1 {
			// The id, for data links into another dashboard; the name alone cannot
			// be turned back into the instance variable's value.
			for _, fr := range frames {
				fr.Fields[1].Labels["instance_id"] = r.ids[0]
			}
		}
		if !grouped && len(frames) > 0 {
			noticeIfCut(frames[0], len(rows), rowLimit(q.TimeRange, interval))
			totals = append(totals, waitTotal(rows))
		}
		resp.Frames = append(resp.Frames, frames...)
	}
	if !grouped && !qm.AggregateInstances && len(totals) == len(resp.Frames) {
		rankInstances(resp.Frames, totals)
	}
	if qm.TopN > models.MaxTopN {
		notice(&resp, fmt.Sprintf("top N is limited to %d", models.MaxTopN))
	}
	return resp
}

// activitySummary is one value per instance over the whole time range, for
// panels that rank or compare instances. Unlike a series reduced in the panel,
// the value does not depend on the chart interval: wait time is the total, and
// AAS the true average over the range.
func (d *Datasource) activitySummary(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	ids, err := qm.InstanceIDs()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	info := d.client.instanceInfo(ctx)
	span := q.TimeRange.To.Sub(q.TimeRange.From)
	var resp backend.DataResponse
	var totals []float64
	for _, id := range ids {
		var summary []map[string]any
		if err := d.client.Get(ctx, "/v1/activity/summary", activityParams(q.TimeRange, []string{id}, qm), &summary); err != nil {
			return d.errorResponse(err, false)
		}
		p := point{t: q.TimeRange.To}
		if len(summary) > 0 {
			p.waitTime, p.waitCount = num(summary[0]["waittime"]), num(summary[0]["waitcount"])
		}
		s := &series{pts: []point{p}}
		fr := s.frame(qm, instanceLabel(info, id), span)
		fr.Fields[1].Labels["instance_id"] = id
		resp.Frames = append(resp.Frames, fr)
		totals = append(totals, p.waitTime)
	}
	rankInstances(resp.Frames, totals)
	return resp
}

// activityRows fetches /activity/time or /activity/time/{dimension} for one
// request, with the bucket size the archiver says it used (0 if it did not).
func (c *Client) activityRows(ctx context.Context, tr backend.TimeRange, interval time.Duration,
	qm *models.QueryModel, r request, topN int) ([]map[string]any, time.Duration, error) {
	params := activityParams(tr, r.ids, qm)
	params.Set("interval", strconv.FormatInt(interval.Milliseconds(), 10))

	path := "/v1/activity/time"
	if qm.GroupBy == models.GroupByNone {
		// one row per bucket; without a limit the archiver stops at 500 rows
		params.Set("limit", strconv.Itoa(rowLimit(tr, interval)))
	} else {
		path += "/" + qm.GroupBy
		params.Set("limit", strconv.Itoa(topN))
		params.Set("other", strconv.FormatBool(qm.Other()))
	}

	var rows []map[string]any
	header, err := c.GetWithHeader(ctx, path, params, &rows)
	return rows, reportedInterval(header), err
}

// activityParams are the parameters every activity request shares: the time
// range in UTC, the instances, and the filters (repeated, as the archiver
// accepts for each).
func activityParams(tr backend.TimeRange, ids []string, qm *models.QueryModel) url.Values {
	params := url.Values{}
	params.Set("from", tr.From.UTC().Format(archiverTime))
	params.Set("to", tr.To.UTC().Format(archiverTime))
	params.Set("tz", "UTC")
	for _, id := range ids {
		params.Add("id", id)
	}
	for _, name := range filterParams {
		for _, v := range qm.Filters[name] {
			params.Add(name, v)
		}
	}
	return params
}

func validDimension(dim string) bool {
	for _, d := range models.GroupDimensions {
		if d == dim {
			return true
		}
	}
	return false
}

// queryInterval is Grafana's interval snapped to the archiver's ladder.
func queryInterval(q backend.DataQuery) time.Duration {
	interval := q.Interval
	if interval <= 0 && q.MaxDataPoints > 0 {
		interval = q.TimeRange.To.Sub(q.TimeRange.From) / time.Duration(q.MaxDataPoints)
	}
	return snapInterval(interval)
}

func waitTotal(rows []map[string]any) float64 {
	var total float64
	for _, row := range rows {
		total += num(row["waittime"])
	}
	return total
}

// rankInstances lists the instances biggest first and colours them as the
// DBmarlin instances screen does, so a fleet reads the same in both.
func rankInstances(frames []*data.Frame, totals []float64) {
	order := make([]int, len(frames))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return totals[order[a]] > totals[order[b]] })
	ranked := make([]*data.Frame, len(frames))
	for rank, i := range order {
		ranked[rank] = frames[i]
		if colour := instanceColour(rank); colour != "" {
			cfg := frames[i].Fields[1].Config
			cfg.Color = map[string]any{"mode": "fixed", "fixedColor": colour}
		}
	}
	copy(frames, ranked)
}
