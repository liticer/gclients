// Copyright 2017 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build go1.7
// +build go1.7

// Package v1 provides bindings to the Prometheus HTTP API v1:
// http://prometheus.io/docs/querying/api/
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/liticer/gclients/prometheus"

	"github.com/levigross/grequests"
	"github.com/prometheus/common/model"
)

const (
	statusAPIError = 422

	apiPrefix = "/api/v1"

	epQuery       = apiPrefix + "/query"
	epQueryRange  = apiPrefix + "/query_range"
	epLabels      = apiPrefix + "/labels"
	epLabelValues = apiPrefix + "/label/:name/values"
	epSeries      = apiPrefix + "/series"
)

// ErrorType models the different API error types.
type ErrorType string

// Possible values for ErrorType.
const (
	ErrBadData     ErrorType = "bad_data"
	ErrTimeout               = "timeout"
	ErrCanceled              = "canceled"
	ErrExec                  = "execution"
	ErrBadResponse           = "bad_response"
)

// Error is an error returned by the API.
type Error struct {
	Type ErrorType
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Msg)
}

// Range represents a sliced time range.
type Range struct {
	// The boundaries of the time range.
	Start, End time.Time
	// The maximum time between two slices within the boundaries.
	Step time.Duration
}

// API provides bindings for Prometheus's v1 API.
type API interface {
	// Health will check prometheus health.
	Health(ctx context.Context) (int, error)
	// Query performs a query for the given time.
	Query(ctx context.Context, query string, ts time.Time) (model.Value, error)
	// QueryRange performs a query for the given range.
	QueryRange(ctx context.Context, query string, r Range) (model.Value, error)
	// Labels getting label names.
	Labels(ctx context.Context, start, end int64, match string) (model.LabelValues, error)
	// LabelValues performs a query for the values of the given label.
	LabelValues(ctx context.Context, start, end int64, label string) (model.LabelValues, error)
	// Series finding series by label matchers.
	Series(ctx context.Context, start, end int64, match string) ([]model.Metric, error)
	// Proxy request to prometheus endpoint
	Proxy(method string, url string, params map[string]string, data map[string]string) (*grequests.Response, error)
}

// queryResult contains result data for a query.
type queryResult struct {
	Type   model.ValueType `json:"resultType"`
	Result interface{}     `json:"result"`

	// The decoded value.
	v model.Value
}

func (qr *queryResult) UnmarshalJSON(b []byte) error {
	v := struct {
		Type   model.ValueType `json:"resultType"`
		Result json.RawMessage `json:"result"`
	}{}

	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case model.ValScalar:
		var sv model.Scalar
		err = json.Unmarshal(v.Result, &sv)
		qr.v = &sv

	case model.ValVector:
		var vv model.Vector
		err = json.Unmarshal(v.Result, &vv)
		qr.v = vv

	case model.ValMatrix:
		var mv model.Matrix
		err = json.Unmarshal(v.Result, &mv)
		qr.v = mv

	default:
		err = fmt.Errorf("unexpected value type %q", v.Type)
	}
	return err
}

// NewAPI returns a new API for the client.
//
// It is safe to use the returned API from multiple goroutines.
func NewAPI(c prometheus.Client) API {
	return &httpAPI{client: apiClient{c}}
}

type httpAPI struct {
	client prometheus.Client
}

func (h *httpAPI) Health(ctx context.Context) (int, error) {
	u := h.client.URL(epQuery, nil)
	q := u.Query()
	q.Set("query", "ALERTS{}")
	q.Set("time", time.Now().Format(time.RFC3339Nano))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}

	code := 0
	res, _, err := h.client.Do(ctx, req)
	if res != nil {
		code = res.StatusCode
	}
	return code, err
}

func (h *httpAPI) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	u := h.client.URL(epQuery, nil)
	q := u.Query()
	q.Set("query", query)
	q.Set("time", ts.Format(time.RFC3339Nano))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var qres queryResult
	err = json.Unmarshal(body, &qres)

	return qres.v, err
}

func (h *httpAPI) QueryRange(ctx context.Context, query string, r Range) (model.Value, error) {
	u := h.client.URL(epQueryRange, nil)
	q := u.Query()
	if !r.Start.IsZero() {
		q.Set("start", r.Start.Format(time.RFC3339Nano))
	}
	if !r.End.IsZero() {
		q.Set("end", r.End.Format(time.RFC3339Nano))
	}
	q.Set("query", query)
	q.Set("step", strconv.FormatFloat(r.Step.Seconds(), 'f', 3, 64))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var qres queryResult
	err = json.Unmarshal(body, &qres)

	return qres.v, err
}

func (h *httpAPI) Labels(ctx context.Context, start, end int64, match string) (model.LabelValues, error) {
	u := h.client.URL(epLabels, nil)
	q := u.Query()
	if start != 0 {
		q.Set("start", time.Unix(start, 0).Format(time.RFC3339Nano))
	}
	if end != 0 {
		q.Set("end", time.Unix(end, 0).Format(time.RFC3339Nano))
	}
	if match != "" {
		q.Set("match[]", match)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	var labelValues model.LabelValues
	err = json.Unmarshal(body, &labelValues)
	return labelValues, err
}

func (h *httpAPI) LabelValues(ctx context.Context, start, end int64, label string) (model.LabelValues, error) {
	u := h.client.URL(epLabelValues, map[string]string{"name": label})
	q := u.Query()
	if start != 0 {
		q.Set("start", time.Unix(start, 0).Format(time.RFC3339Nano))
	}
	if end != 0 {
		q.Set("end", time.Unix(end, 0).Format(time.RFC3339Nano))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	var labelValues model.LabelValues
	err = json.Unmarshal(body, &labelValues)
	return labelValues, err
}

func (h *httpAPI) Series(ctx context.Context, start, end int64, match string) ([]model.Metric, error) {
	u := h.client.URL(epSeries, nil)
	q := u.Query()
	if start != 0 {
		q.Set("start", time.Unix(start, 0).Format(time.RFC3339Nano))
	}
	if end != 0 {
		q.Set("end", time.Unix(end, 0).Format(time.RFC3339Nano))
	}
	if match != "" {
		q.Set("match[]", match)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	var series []model.Metric
	err = json.Unmarshal(body, &series)
	return series, err
}

func (h *httpAPI) Proxy(method string, url string, params map[string]string, data map[string]string) (*grequests.Response, error) {
	return h.client.Proxy(method, url, params, data)
}

// apiClient wraps a regular client and processes successful API responses.
// Successful also includes responses that errored at the API level.
type apiClient struct {
	prometheus.Client
}

type apiResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType ErrorType       `json:"errorType"`
	Error     string          `json:"error"`
}

func (c apiClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	resp, body, err := c.Client.Do(ctx, req)
	if err != nil {
		return resp, body, err
	}

	code := resp.StatusCode

	if code/100 != 2 && code != statusAPIError {
		return resp, body, &Error{
			Type: ErrBadResponse,
			Msg:  fmt.Sprintf("bad response code %d", resp.StatusCode),
		}
	}

	var result apiResponse

	if err = json.Unmarshal(body, &result); err != nil {
		return resp, body, &Error{
			Type: ErrBadResponse,
			Msg:  err.Error(),
		}
	}

	if (code == statusAPIError) != (result.Status == "error") {
		err = &Error{
			Type: ErrBadResponse,
			Msg:  "inconsistent body for response code",
		}
	}

	if code == statusAPIError && result.Status == "error" {
		err = &Error{
			Type: result.ErrorType,
			Msg:  result.Error,
		}
	}

	return resp, result.Data, err
}
