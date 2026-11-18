package plugin

import (
	"net/url"
	"time"
)

// uiFilters maps an activity dimension to the DBmarlin activity page's filter
// parameter. Others, such as module, cannot be filtered on in the page, so the
// link goes to the unfiltered activity page.
var uiFilters = map[string]string{
	"waitevent": "waits",
	"database":  "databases",
	"session":   "sessions",
	"client":    "clients",
	"username":  "users",
	"program":   "programs",
}

// linkTime is the fm/to format of DBmarlin UI links, which is also what its
// alert emails use. The plugin always queries in UTC, so tz is UTC.
const linkTime = "2006-01-02+15:04:05"

// links builds DBmarlin UI deep links for one query's time range.
type links struct {
	base     string
	from, to time.Time
}

func (l links) query() string {
	return "fm=" + l.from.UTC().Format(linkTime) + "&to=" + l.to.UTC().Format(linkTime) + "&tz=UTC"
}

func (l links) activity(id, dimension, value string) string {
	link := l.base + "/instances/" + id + "/activity?" + l.query()
	if param, ok := uiFilters[dimension]; ok && value != "" {
		link += "&" + param + "=" + url.QueryEscape(value)
	}
	return link
}

// statement links to a statement's page, on its activity tab.
func (l links) statement(id, sqlhash string) string {
	return l.statementTab(id, sqlhash, "activity")
}

// statementTab links to one tab of a statement's page, such as sql-statistics.
func (l links) statementTab(id, sqlhash, tab string) string {
	return l.base + "/instances/" + id + "/activity/statement/" + url.PathEscape(sqlhash) + "/" + tab + "?" + l.query()
}
