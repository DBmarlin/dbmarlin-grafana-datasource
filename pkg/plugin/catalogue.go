package plugin

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// catalogueTTL is how long a database type's statistic catalogue is reused.
// It only changes when DBmarlin is upgraded, and it is large.
const catalogueTTL = 10 * time.Minute

// statisticType is one entry of the statistic catalogue.
type statisticType struct {
	Statistic   string `json:"statistic"`
	Units       string `json:"units"`
	Description string `json:"description"`
	Aggregation string `json:"aggregation"`
	Cumulative  bool   `json:"cumulative"`
	Collect     bool   `json:"collect"`
}

type cachedCatalogue struct {
	at   time.Time
	list []statisticType
}

// statisticCatalogue returns the statistics DBmarlin collects for a database
// type. The archiver puts a statistic name into its SQL, so the plugin only
// forwards names found here.
func (c *Client) statisticCatalogue(ctx context.Context, dbType string) ([]statisticType, error) {
	return c.catalogue(ctx, "db:"+dbType, "/v1/statistic/type", dbType)
}

// sqlStatisticCatalogue is the same for the SQL statistics (executions,
// duration, reads...), which are a separate set per database type.
func (c *Client) sqlStatisticCatalogue(ctx context.Context, dbType string) ([]statisticType, error) {
	return c.catalogue(ctx, "sql:"+dbType, "/v1/sql/statistic/type", dbType)
}

func (c *Client) catalogue(ctx context.Context, key, path, dbType string) ([]statisticType, error) {
	c.catalogueMu.Lock()
	cached, ok := c.catalogues[key]
	c.catalogueMu.Unlock()
	if ok && time.Since(cached.at) < catalogueTTL {
		return cached.list, nil
	}

	var all []statisticType
	if err := c.Get(ctx, path, url.Values{"type": {dbType}}, &all); err != nil {
		return nil, err
	}
	collected := all[:0]
	for _, s := range all {
		if s.Collect {
			collected = append(collected, s)
		}
	}

	c.catalogueMu.Lock()
	c.catalogues[key] = cachedCatalogue{at: time.Now(), list: collected}
	c.catalogueMu.Unlock()
	return collected, nil
}

// findStatistic looks a name up in the catalogue, ignoring case.
func findStatistic(catalogue []statisticType, name string) (statisticType, bool) {
	name = strings.TrimSpace(name)
	for _, s := range catalogue {
		if strings.EqualFold(s.Statistic, name) {
			return s, true
		}
	}
	return statisticType{}, false
}
