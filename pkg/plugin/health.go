package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// CheckHealth backs "Save & test": it checks the connection, the credentials
// and that the user can see some instances.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.URL == "" {
		return healthError("Enter the DBmarlin URL"), nil
	}

	var instances []struct{}
	err := d.client.Get(ctx, "/v1/datasource", nil, &instances)
	var status *statusError
	switch {
	case errors.Is(err, errUnauthorized), errors.Is(err, errForbidden), errors.Is(err, errNotDBmarlin), errors.Is(err, errUnreadable):
		return healthError(err.Error()), nil
	case errors.As(err, &status):
		return healthError(status.Error()), nil
	case err != nil:
		return healthError(fmt.Sprintf("Cannot reach DBmarlin at %s: %v", d.settings.URL, err)), nil
	case len(instances) == 0:
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Connected, but this user can see no instances (check the user's tags)",
		}, nil
	}
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: fmt.Sprintf("Connected to DBmarlin: %d instances visible", len(instances)),
	}, nil
}

func healthError(msg string) *backend.CheckHealthResult {
	return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: msg}
}
