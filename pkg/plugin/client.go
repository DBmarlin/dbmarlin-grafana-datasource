package plugin

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/DBmarlin/dbmarlin-grafana-datasource/pkg/models"
)

// restPath is where nginx proxies the archiver. The plugin only ever issues
// GETs: it never writes back to DBmarlin.
const restPath = "/archiver/rest"

var (
	errUnauthorized = errors.New("DBmarlin rejected the username or password")
	errForbidden    = errors.New("DBmarlin refused access (HTTP 403): the user may not be allowed to see this instance or use this feature")
	errNotDBmarlin  = errors.New("URL does not look like a DBmarlin server")
	errTooLarge     = errors.New("DBmarlin's answer is too large; narrow the time range or reduce top N")
	errUnreadable   = errors.New("DBmarlin sent an answer the plugin cannot read")
)

// statusError is a non-2xx answer from the archiver.
type statusError struct {
	Status int
	Body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("DBmarlin returned HTTP %d: %s", e.Status, e.Body)
}

// Client talks to the archiver REST API with the configured basic auth.
type Client struct {
	settings *models.PluginSettings
	http     *http.Client
	// slots is a semaphore across all panels using this data source.
	slots chan struct{}

	catalogueMu sync.Mutex
	catalogues  map[string]cachedCatalogue

	instanceMu   sync.Mutex
	instanceAt   time.Time
	instanceList []instance
}

func NewClient(settings *models.PluginSettings) (*Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: settings.TLSSkipVerify} //nolint:gosec // opt-in for self-signed DBmarlin certificates
	if ca := settings.Secrets.CACert; ca != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("TLS CA certificate is not valid PEM")
		}
		tlsConfig.RootCAs = pool
	}
	return &Client{
		settings: settings,
		http: &http.Client{
			Timeout:   settings.Timeout(),
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
		slots:      make(chan struct{}, settings.MaxConcurrent),
		catalogues: map[string]cachedCatalogue{},
	}, nil
}

// intervalHeader carries the bucket size, in milliseconds, the archiver
// actually used (DBmarlin 6.12 or later).
const intervalHeader = "X-DBmarlin-Interval"

// maxAnswer is the largest answer the plugin will read, in bytes. A variable
// so a test can lower it.
var maxAnswer = 64 << 20

// Get fetches restPath+path (for example "/v1/datasource") and decodes the JSON
// answer into out. Repeated parameters in query are sent as such, which the
// archiver accepts for every filter.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	_, err := c.GetWithHeader(ctx, path, query, out)
	return err
}

// GetWithHeader is Get that also returns the response headers.
func (c *Client) GetWithHeader(ctx context.Context, path string, query url.Values, out any) (http.Header, error) {
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	u := c.settings.URL + restPath + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.settings.Username != "" {
		req.SetBasicAuth(c.settings.Username, c.settings.Secrets.Password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	// One byte over the limit tells an answer that fits from one that was cut.
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxAnswer)+1))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, errUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return nil, errForbidden
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, &statusError{Status: resp.StatusCode, Body: truncate(string(body), 200)}
	}
	if len(body) > maxAnswer {
		return nil, errTooLarge
	}
	if err := json.Unmarshal(body, out); err != nil {
		if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			// JSON, but not what was expected: a version mismatch, not a wrong URL.
			return nil, fmt.Errorf("%w: %v", errUnreadable, err)
		}
		// An HTML page, for example nginx without the /archiver proxy.
		return nil, errNotDBmarlin
	}
	return resp.Header, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
