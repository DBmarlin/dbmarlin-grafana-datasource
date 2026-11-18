package models

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	QueryActivityTimeseries = "activity_timeseries"
	QueryActivityTable      = "activity_table"
	QueryActivitySummary    = "activity_summary"
	QueryDBStatistic        = "db_statistic"
	QueryHostStatistic      = "host_statistic"
	QueryEvents             = "events"
	QuerySQLStatistics      = "sql_statistics"

	MaxInstances  = 20
	MaxTopN       = 50
	MaxStatistics = 10
	DefaultTopN   = 10

	DefaultSQLOrderBy = "duration"

	GroupByNone = "none"

	MetricAAS         = "aas"
	MetricWaitTime    = "waittime"
	MetricWaitCount   = "waitcount"
	MetricAvgWaitTime = "avgwaittime"
)

// GroupDimensions are the dimensions of the archiver's activity/time/{dimension}
// endpoint (DBmarlin 6.11 or later).
var GroupDimensions = []string{
	"waitevent", "sqlhash", "session", "username", "database", "client", "program", "module",
}

// QueryModel is the one JSON model shared by every query type. Template
// variables are already expanded by the frontend, so multi-value variables
// arrive as repeated entries.
type QueryModel struct {
	QueryType          string              `json:"queryType"`
	Instances          []string            `json:"instances"`
	AggregateInstances bool                `json:"aggregateInstances"`
	GroupBy            string              `json:"groupBy"`
	TopN               int                 `json:"topN"`
	IncludeOther       *bool               `json:"includeOther"`
	Metric             string              `json:"metric"`
	Filters            map[string][]string `json:"filters"`

	// db_statistic: names from the instance type's statistic catalogue.
	// sql_statistic: extra columns from the SQL statistic catalogue.
	Statistics []string `json:"statistics"`
	// sql_statistics: the statistic to rank by; DefaultSQLOrderBy when empty.
	OrderBy string `json:"orderBy"`
	// host_statistic: host ids, or empty for the hosts of the instances.
	Hosts       []string `json:"hosts"`
	HostMetrics []string `json:"hostMetrics"`

	// events: any of alert, change, custom; empty means all.
	EventKinds []string `json:"eventKinds"`
}

// ParseQuery decodes a query and fills in defaults.
func ParseQuery(raw json.RawMessage) (*QueryModel, error) {
	q := QueryModel{}
	if err := json.Unmarshal(raw, &q); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if q.QueryType == "" {
		q.QueryType = QueryActivityTimeseries
	}
	if q.GroupBy == "" {
		q.GroupBy = GroupByNone
	}
	if q.Metric == "" {
		q.Metric = MetricAAS
	}
	if q.TopN <= 0 {
		q.TopN = DefaultTopN
	}
	return &q, nil
}

// Other reports whether the remainder outside the top N is kept as one series.
func (q *QueryModel) Other() bool {
	return q.IncludeOther == nil || *q.IncludeOther
}

// InstanceIDs returns the instance ids, which must be numeric by now.
func (q *QueryModel) InstanceIDs() ([]string, error) {
	if len(q.Instances) == 0 {
		return nil, fmt.Errorf("select at least one instance")
	}
	if len(q.Instances) > MaxInstances {
		return nil, fmt.Errorf("a query can use at most %d instances", MaxInstances)
	}
	ids := make([]string, 0, len(q.Instances))
	for _, s := range q.Instances {
		if _, err := strconv.Atoi(s); err != nil {
			return nil, fmt.Errorf("instance %q is not an instance id", s)
		}
		ids = append(ids, s)
	}
	return ids, nil
}

// HostMetricNames are the utilisation percentages host_statistic offers.
var HostMetricNames = []string{"cpuutilisation", "memoryutilisation", "ioutilisation", "diskutilisation"}

// StatisticNames returns the requested statistic names, at least one and at
// most MaxStatistics.
func (q *QueryModel) StatisticNames() ([]string, error) {
	if len(q.Statistics) == 0 {
		return nil, fmt.Errorf("select at least one statistic")
	}
	if len(q.Statistics) > MaxStatistics {
		return nil, fmt.Errorf("a query can use at most %d statistics", MaxStatistics)
	}
	return q.Statistics, nil
}

// SQLOrderBy is the statistic the SQL statistics table is ranked by.
func (q *QueryModel) SQLOrderBy() string {
	if q.OrderBy == "" {
		return DefaultSQLOrderBy
	}
	return q.OrderBy
}

// SQLColumns returns the extra statistic columns, at most MaxStatistics.
func (q *QueryModel) SQLColumns() ([]string, error) {
	if len(q.Statistics) > MaxStatistics {
		return nil, fmt.Errorf("a query can use at most %d statistics", MaxStatistics)
	}
	return q.Statistics, nil
}

// HostIDs returns the explicitly chosen host ids, which must be numeric.
func (q *QueryModel) HostIDs() ([]string, error) {
	if len(q.Hosts) > MaxInstances {
		return nil, fmt.Errorf("a query can use at most %d hosts", MaxInstances)
	}
	for _, s := range q.Hosts {
		if _, err := strconv.Atoi(s); err != nil {
			return nil, fmt.Errorf("host %q is not a host id", s)
		}
	}
	return q.Hosts, nil
}

// Metrics returns the host metrics to chart: the four utilisations by default.
func (q *QueryModel) Metrics() ([]string, error) {
	if len(q.HostMetrics) == 0 {
		return HostMetricNames, nil
	}
	for _, m := range q.HostMetrics {
		ok := false
		for _, known := range HostMetricNames {
			ok = ok || m == known
		}
		if !ok {
			return nil, fmt.Errorf("unknown host metric %q", m)
		}
	}
	return q.HostMetrics, nil
}

// EventKinds are the kinds of event the archiver reports.
var EventKinds = []string{"alert", "change", "custom"}

// Kinds returns the event kinds to show; all of them by default.
func (q *QueryModel) Kinds() ([]string, error) {
	if len(q.EventKinds) == 0 {
		return EventKinds, nil
	}
	for _, k := range q.EventKinds {
		ok := false
		for _, known := range EventKinds {
			ok = ok || k == known
		}
		if !ok {
			return nil, fmt.Errorf("unknown event kind %q", k)
		}
	}
	return q.EventKinds, nil
}
