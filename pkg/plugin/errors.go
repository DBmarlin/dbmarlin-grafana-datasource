package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// errorResponse maps an archiver error to a Grafana response with the right
// error source, so Grafana's plugin health metrics stay meaningful.
func (d *Datasource) errorResponse(err error, grouped bool) backend.DataResponse {
	var se *statusError
	switch {
	case errors.Is(err, errUnauthorized):
		return backend.ErrDataResponseWithSource(backend.StatusUnauthorized, backend.ErrorSourceDownstream, err.Error())
	case errors.Is(err, errForbidden):
		return backend.ErrDataResponseWithSource(backend.StatusForbidden, backend.ErrorSourceDownstream, err.Error())
	case errors.Is(err, errNotDBmarlin), errors.Is(err, errTooLarge), errors.Is(err, errUnreadable):
		return backend.ErrDataResponseWithSource(backend.StatusBadGateway, backend.ErrorSourceDownstream, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return backend.ErrDataResponseWithSource(backend.StatusTimeout, backend.ErrorSourceDownstream,
			fmt.Sprintf("DBmarlin did not answer within %d s; narrow the time range or reduce top N",
				int(d.settings.Timeout().Seconds())))
	case errors.As(err, &se) && se.Status == http.StatusNotFound && grouped:
		return backend.ErrDataResponseWithSource(backend.StatusNotFound, backend.ErrorSourceDownstream,
			"grouping needs DBmarlin 6.11 or later")
	case errors.As(err, &se):
		return backend.ErrDataResponseWithSource(backend.Status(se.Status), backend.ErrorSourceDownstream, se.Error())
	default:
		return backend.ErrDataResponseWithSource(backend.StatusBadGateway, backend.ErrorSourceDownstream,
			fmt.Sprintf("Cannot reach DBmarlin at %s: %v", d.settings.URL, err))
	}
}
