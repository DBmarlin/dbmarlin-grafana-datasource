package plugin

import (
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// maxRows is the most time buckets one call asks for. Grafana picks the
// interval from the panel width, so this is only reached with a tiny minimum
// interval over a very long range.
const maxRows = 100000

// rowLimit is the limit to send on a time series call. The archiver cuts every
// answer at 500 rows unless told otherwise, and sorts by time, so without this
// a range with more than 500 buckets silently loses its recent end. There is
// one row per bucket, and the archiver never uses a finer bucket than asked.
func rowLimit(tr backend.TimeRange, interval time.Duration) int {
	n := int(tr.To.Sub(tr.From)/interval) + 2
	return min(max(n, 1), maxRows)
}

// noticeIfCut warns on the frame when the answer reached the limit, so a cut
// is never silent.
func noticeIfCut(frame *data.Frame, rows, limit int) {
	if frame != nil && rows >= limit {
		frame.AppendNotices(data.Notice{Severity: data.NoticeSeverityWarning,
			Text: fmt.Sprintf("showing the first %d points; narrow the time range or use a larger interval", limit)})
	}
}
