package plugin

import (
	"context"
	"strconv"
	"time"
)

type instance struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	// HasSQLStatistics is whether DBmarlin collects SQL statistics for it.
	HasSQLStatistics bool `json:"hassqlstatistics"`
}

// instances lists the instances the configured DBmarlin user can see.
func (c *Client) instances(ctx context.Context) ([]instance, error) {
	var list []instance
	err := c.Get(ctx, "/v1/datasource", nil, &list)
	return list, err
}

// instanceTTL is how long the instance list is reused by instanceInfo, so a
// dashboard of many panels does not list the instances once per panel.
const instanceTTL = 30 * time.Second

// instanceInfo maps instance id to the instance, for labels and for the
// database type some endpoints need. A failure only costs the friendly names:
// callers fall back to the id.
func (c *Client) instanceInfo(ctx context.Context) map[string]instance {
	info := map[string]instance{}
	c.instanceMu.Lock()
	defer c.instanceMu.Unlock()
	if time.Since(c.instanceAt) >= instanceTTL || c.instanceList == nil {
		list, err := c.instances(ctx)
		if err != nil {
			return info
		}
		c.instanceList, c.instanceAt = list, time.Now()
	}
	list := c.instanceList
	for _, i := range list {
		info[strconv.Itoa(i.ID)] = i
	}
	return info
}

func instanceLabel(info map[string]instance, id string) string {
	if n := info[id].Name; n != "" {
		return n
	}
	return id
}
