package plugin

import (
	"sort"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Layouts the archiver uses for timeslice, depending on the tz it was given.
var sliceLayouts = []string{archiverTime, "2006-01-02 15:04:05-07", "2006-01-02 15:04:05.999999-07"}

func parseSlice(s string) (time.Time, bool) {
	for _, layout := range sliceLayouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

type point struct {
	t time.Time
	// raw archiver figures; the metric is applied once the bucket size is known.
	waitTime, waitCount float64
}

type series struct {
	value string // dimension value, or "" when ungrouped
	other bool
	total float64 // wait time over the whole range, to rank by
	pts   []point
	rank  int // 0 is the series with the most wait time
}

// buildFrames turns archiver rows into one frame per series. The metric is
// applied here because AAS needs the bucket size the archiver really used:
// reported is that size from the response header, 0 if the archiver sent none.
func buildFrames(rows []map[string]any, qm *models.QueryModel, instance string, requested, reported time.Duration) []*data.Frame {
	byValue := map[string]*series{}
	var order []*series
	var slices []time.Time

	for _, row := range rows {
		ts, ok := parseSlice(str(row["timeslice"]))
		if !ok {
			continue
		}
		slices = append(slices, ts)
		value := ""
		if qm.GroupBy != models.GroupByNone {
			value = str(row[qm.GroupBy])
		}
		other, _ := row["other"].(bool)
		key := value
		if other {
			key = "\x00other" // never collides with a real value named "other"
		}
		s := byValue[key]
		if s == nil {
			s = &series{value: value, other: other}
			byValue[key] = s
			order = append(order, s)
		}
		s.total += num(row["waittime"])
		s.pts = append(s.pts, point{t: ts, waitTime: num(row["waittime"]), waitCount: num(row["waitcount"])})
	}

	// Trust the archiver when it reports the interval; guess from the rows
	// only for older servers.
	bucket := reported
	if bucket == 0 {
		bucket = effectiveInterval(requested, slices)
	}
	// Biggest first, as the DBmarlin UI lists and stacks them, with the
	// remainder last; ties by value keep the order stable.
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].other != order[j].other {
			return order[j].other
		}
		if order[i].total != order[j].total {
			return order[i].total > order[j].total
		}
		return order[i].value < order[j].value
	})
	for i, s := range order {
		s.rank = i
	}

	frames := make([]*data.Frame, 0, len(order))
	for _, s := range order {
		frames = append(frames, s.frame(qm, instance, bucket))
	}
	return frames
}

func (s *series) frame(qm *models.QueryModel, instance string, bucket time.Duration) *data.Frame {
	times := make([]time.Time, len(s.pts))
	values := make([]float64, len(s.pts))
	for i, p := range s.pts {
		times[i] = p.t
		values[i] = metricValue(qm.Metric, p, bucket)
	}

	labels := data.Labels{}
	if instance != "" {
		labels["instance"] = instance
	}
	name := metricTitle(qm.Metric)
	if qm.GroupBy != models.GroupByNone {
		labels[qm.GroupBy] = s.value
		name = s.value
		if s.other {
			// Mark the remainder so it cannot be mistaken for a real value.
			labels[qm.GroupBy] = "other"
			labels["other"] = "true"
			name = "Other"
		}
	} else if instance != "" {
		name = instance
	}

	timeField := data.NewField("time", nil, times)
	timeField.Config = &data.FieldConfig{Interval: float64(bucket.Milliseconds())}
	valueField := data.NewField(qm.Metric, labels, values)
	valueField.Config = &data.FieldConfig{Unit: metricUnit(qm.Metric), DisplayNameFromDS: name}
	if qm.GroupBy != models.GroupByNone {
		if colour := seriesColour(qm.GroupBy, s.rank, s.other); colour != "" {
			valueField.Config.Color = map[string]any{"mode": "fixed", "fixedColor": colour}
		}
	}

	frame := data.NewFrame(name, timeField, valueField)
	frame.Meta = &data.FrameMeta{Type: data.FrameTypeTimeSeriesMulti}
	return frame
}

func notice(resp *backend.DataResponse, text string) {
	if len(resp.Frames) == 0 {
		return
	}
	resp.Frames[0].AppendNotices(data.Notice{Severity: data.NoticeSeverityInfo, Text: text})
}

func metricValue(metric string, p point, bucket time.Duration) float64 {
	switch metric {
	case models.MetricWaitTime:
		return p.waitTime / 1000 // seconds
	case models.MetricWaitCount:
		return p.waitCount
	case models.MetricAvgWaitTime:
		if p.waitCount == 0 {
			return 0
		}
		return p.waitTime / p.waitCount // ms
	default: // AAS: session-milliseconds over the bucket's milliseconds
		return p.waitTime / float64(bucket.Milliseconds())
	}
}

func metricUnit(metric string) string {
	switch metric {
	case models.MetricWaitTime:
		return "s"
	case models.MetricAvgWaitTime:
		return "ms"
	default:
		return "short"
	}
}

func metricTitle(metric string) string {
	switch metric {
	case models.MetricWaitTime:
		return "Wait time"
	case models.MetricWaitCount:
		return "Wait count"
	case models.MetricAvgWaitTime:
		return "Avg wait time"
	default:
		return "AAS"
	}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}
