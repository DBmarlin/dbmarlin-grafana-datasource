package plugin

import (
	"context"
	"errors"
	"testing"
	"time"
)

func clientGet(t *testing.T, f *fakeArchiver, path string) error {
	t.Helper()
	var out []map[string]any
	return f.datasource(t).client.Get(context.Background(), path, nil, &out)
}

func TestForbiddenIsNotACredentialsError(t *testing.T) {
	f := newFakeArchiver(t)
	f.fail = map[string]int{restPath + "/v1/activity/summary": 403}
	resp := run(t, f, map[string]any{"queryType": "activity_table", "instances": []string{"3"}})
	if resp.Error == nil || resp.Status != 403 {
		t.Fatalf("want 403, got %v %v", resp.Status, resp.Error)
	}
	if !errors.Is(clientGet(t, f, "/v1/activity/summary"), errForbidden) {
		t.Error("a 403 should be errForbidden, not errUnauthorized")
	}
}

func TestAnswerThatCannotBeRead(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{
		restPath + "/v1/html":  "<html>nginx</html>",
		restPath + "/v1/cut":   `[{"waittime": 1`,
		restPath + "/v1/types": `{"id": "not a list"}`,
	}
	if err := clientGet(t, f, "/v1/html"); !errors.Is(err, errNotDBmarlin) {
		t.Errorf("an HTML page means the URL is wrong, got %v", err)
	}
	for _, path := range []string{"/v1/cut", "/v1/types"} {
		if err := clientGet(t, f, path); !errors.Is(err, errUnreadable) || errors.Is(err, errNotDBmarlin) {
			t.Errorf("%s: JSON the plugin cannot read is not a wrong URL, got %v", path, err)
		}
	}
}

func TestAnswerLargerThanTheLimit(t *testing.T) {
	f := newFakeArchiver(t)
	f.routes = map[string]string{restPath + "/v1/big": `[{"a":1},{"a":2},{"a":3}]`}
	defer func(n int) { maxAnswer = n }(maxAnswer)

	maxAnswer = len(f.routes[restPath+"/v1/big"])
	if err := clientGet(t, f, "/v1/big"); err != nil {
		t.Errorf("an answer exactly at the limit is fine, got %v", err)
	}
	maxAnswer--
	if err := clientGet(t, f, "/v1/big"); !errors.Is(err, errTooLarge) {
		t.Errorf("an answer over the limit must say so, not look like bad JSON, got %v", err)
	}
}

func TestInstanceListIsReusedBriefly(t *testing.T) {
	f := newFakeArchiver(t)
	client := f.datasource(t).client
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if info := client.instanceInfo(ctx); info["3"].Name != "repo" {
			t.Fatalf("instance 3 not found: %v", info)
		}
	}
	if f.datasourceHits != 1 {
		t.Errorf("listed the instances %d times for 5 queries, want 1", f.datasourceHits)
	}

	client.instanceAt = time.Now().Add(-2 * instanceTTL)
	client.instanceInfo(ctx)
	if f.datasourceHits != 2 {
		t.Errorf("an expired list should be fetched again, got %d fetches", f.datasourceHits)
	}
}
