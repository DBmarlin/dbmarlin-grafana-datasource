package plugin

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// statisticFrame is one statistic's time series for one instance or host.
// Missing buckets stay missing (nil), not zero.
func statisticFrame(name string, labels data.Labels, unit string, bucket time.Duration,
	times []time.Time, values []*float64) *data.Frame {
	timeField := data.NewField("time", nil, times)
	timeField.Config = &data.FieldConfig{Interval: float64(bucket.Milliseconds())}
	valueField := data.NewField(name, labels, values)
	valueField.Config = &data.FieldConfig{Unit: unit, DisplayNameFromDS: name}
	frame := data.NewFrame(name, timeField, valueField)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
	return frame
}

// dbStatistic charts database statistics, one series per statistic per instance.
func (d *Datasource) dbStatistic(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	ids, err := qm.InstanceIDs()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	names, err := qm.StatisticNames()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}

	info := d.client.instanceInfo(ctx)
	interval := queryInterval(q)
	var resp backend.DataResponse
	for _, id := range ids {
		dbType := info[id].Type
		if dbType == "" {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("instance %s was not found", id))
		}
		catalogue, err := d.client.statisticCatalogue(ctx, dbType)
		if err != nil {
			return d.errorResponse(err, false)
		}
		for _, name := range names {
			// Only names from the catalogue are forwarded: the archiver puts the
			// statistic name into its SQL.
			stat, ok := findStatistic(catalogue, name)
			if !ok {
				return backend.ErrDataResponse(backend.StatusBadRequest,
					fmt.Sprintf("%q is not a statistic DBmarlin collects for %s", name, dbType))
			}
			frame, err := d.client.statisticSeries(ctx, q.TimeRange, interval, id, instanceLabel(info, id), stat)
			if err != nil {
				return d.errorResponse(err, false)
			}
			resp.Frames = append(resp.Frames, frame)
		}
	}
	return resp
}

// statisticSeries fetches /statistic/time. Averaged statistics use avg;
// counters use the sum per time slice, as the DBmarlin UI and MCP server do.
func (c *Client) statisticSeries(ctx context.Context, tr backend.TimeRange, interval time.Duration,
	id, instanceName string, stat statisticType) (*data.Frame, error) {
	params := url.Values{}
	params.Set("from", tr.From.UTC().Format(archiverTime))
	params.Set("to", tr.To.UTC().Format(archiverTime))
	params.Set("tz", "UTC")
	params.Set("id", id)
	params.Set("statistic", stat.Statistic)
	params.Set("interval", strconv.FormatInt(interval.Milliseconds(), 10))
	limit := rowLimit(tr, interval)
	params.Set("limit", strconv.Itoa(limit)) // the archiver stops at 500 rows otherwise

	var rows []map[string]any
	header, err := c.GetWithHeader(ctx, "/v1/statistic/time", params, &rows)
	if err != nil {
		return nil, err
	}
	bucket := reportedInterval(header)
	if bucket == 0 {
		bucket = interval
	}

	field := "sum"
	if stat.Aggregation == "avg" {
		field = "avg"
	}
	var times []time.Time
	var values []*float64
	for _, row := range rows {
		t, ok := parseSlice(str(row["timeslice"]))
		if !ok {
			continue
		}
		times = append(times, t)
		if v, ok := row[field].(float64); ok {
			values = append(values, &v)
		} else {
			values = append(values, nil)
		}
	}
	name := stat.Statistic
	frame := statisticFrame(name, data.Labels{"instance": instanceName, "instance_id": id, "statistic": stat.Statistic},
		grafanaUnit(stat.Units), bucket, times, values)
	noticeIfCut(frame, len(rows), limit)
	return frame, nil
}

// hostRef is a monitored host; /v1/host rows also hold credentials, which the
// plugin never decodes.
type hostRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// hostIDs returns the explicitly chosen hosts, or the hosts of the instances.
func (c *Client) hostIDs(ctx context.Context, qm *models.QueryModel) ([]string, error) {
	if explicit, err := qm.HostIDs(); err != nil || len(explicit) > 0 {
		return explicit, err
	}
	ids, _ := qm.InstanceIDs() // validated by the caller
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		var links []struct {
			HostID int `json:"hostid"`
		}
		if err := c.Get(ctx, "/v1/datasource/host", url.Values{"id": {id}}, &links); err != nil {
			return nil, err
		}
		for _, l := range links {
			h := strconv.Itoa(l.HostID)
			if !seen[h] {
				seen[h] = true
				out = append(out, h)
			}
		}
	}
	return out, nil
}

// hostStatistic charts host utilisation, one series per host per metric.
func (d *Datasource) hostStatistic(ctx context.Context, q backend.DataQuery, qm *models.QueryModel) backend.DataResponse {
	metrics, err := qm.Metrics()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if _, err := qm.HostIDs(); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, err.Error())
	}
	if len(qm.Hosts) == 0 {
		if _, err := qm.InstanceIDs(); err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, "select a host or an instance")
		}
	}
	hosts, err := d.client.hostIDs(ctx, qm)
	if err != nil {
		return d.errorResponse(err, false)
	}
	if len(hosts) == 0 {
		return backend.ErrDataResponse(backend.StatusNotFound,
			"No monitored host is linked to the selected instances. Host metrics need DBmarlin host monitoring.")
	}

	names := d.client.hostNames(ctx)
	interval := queryInterval(q)
	var resp backend.DataResponse
	for _, host := range hosts {
		frames, err := d.client.hostSeries(ctx, q.TimeRange, interval, host, firstNonEmpty(names[host], host), metrics)
		if err != nil {
			return d.errorResponse(err, false)
		}
		resp.Frames = append(resp.Frames, frames...)
	}
	return resp
}

// hostNames maps host id to name; a failure only costs the friendly names.
func (c *Client) hostNames(ctx context.Context) map[string]string {
	out := map[string]string{}
	var hosts []hostRef
	if err := c.Get(ctx, "/v1/host", nil, &hosts); err != nil {
		return out
	}
	for _, h := range hosts {
		out[strconv.Itoa(h.ID)] = h.Name
	}
	return out
}

// hostSeries fetches /v2/host/statistic/time, whose rows are wide (one column
// per metric), and returns one frame per requested metric.
func (c *Client) hostSeries(ctx context.Context, tr backend.TimeRange, interval time.Duration,
	host, hostName string, metrics []string) ([]*data.Frame, error) {
	params := url.Values{}
	params.Set("from", tr.From.UTC().Format(archiverTime))
	params.Set("to", tr.To.UTC().Format(archiverTime))
	params.Set("tz", "UTC")
	params.Set("id", host)
	params.Set("interval", strconv.FormatInt(interval.Milliseconds(), 10))
	limit := rowLimit(tr, interval)
	params.Set("limit", strconv.Itoa(limit)) // the archiver stops at 500 rows otherwise

	var rows []map[string]any
	header, err := c.GetWithHeader(ctx, "/v2/host/statistic/time", params, &rows)
	if err != nil {
		return nil, err
	}
	bucket := reportedInterval(header)
	if bucket == 0 {
		bucket = interval
	}

	var frames []*data.Frame
	for _, metric := range metrics {
		var times []time.Time
		var values []*float64
		for _, row := range rows {
			t, ok := parseSlice(str(row["timeslice"]))
			if !ok {
				continue
			}
			times = append(times, t)
			if v, ok := row[metric].(float64); ok {
				values = append(values, &v)
			} else {
				values = append(values, nil)
			}
		}
		frames = append(frames, statisticFrame(metric, data.Labels{"host": hostName, "metric": metric},
			"percent", bucket, times, values))
	}
	if len(frames) > 0 {
		noticeIfCut(frames[0], len(rows), limit)
	}
	return frames, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
