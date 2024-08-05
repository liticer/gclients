// Copyright 2018 The Prometheus Authors
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

package alertmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/api"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/types"
)

const (
	apiPrefix = "/api/v2"

	epStatus   = apiPrefix + "/status"
	epSilence  = apiPrefix + "/silence/:id"
	epSilences = apiPrefix + "/silences"
	epAlerts   = apiPrefix + "/alerts"
)

// ServerStatus represents the status of the AlertManager endpoint.
type ServerStatus struct {
	Config        *config.Config    `json:"config"`
	VersionInfo   map[string]string `json:"versionInfo"`
	Uptime        time.Time         `json:"uptime"`
	ClusterStatus *ClusterStatus    `json:"cluster"`
}

// PeerStatus represents the status of a peer in the cluster.
type PeerStatus struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// ClusterStatus represents the status of the cluster.
type ClusterStatus struct {
	Name   string       `json:"name"`
	Status string       `json:"status"`
	Peers  []PeerStatus `json:"peers"`
}

// apiClient wraps a regular client and processes successful API responses.
// Successful also includes responses that errored at the API level.
type apiClient struct {
	api.Client
}

type clientError struct {
	code int
	msg  string
}

func (e *clientError) Error() string {
	return fmt.Sprintf("%s (code: %d)", e.msg, e.code)
}

func (c apiClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	req.Header.Set("Content-Type", "application/json")
	resp, body, err := c.Client.Do(ctx, req)
	if err != nil {
		return resp, body, err
	}

	if resp.StatusCode != http.StatusOK {
		err = &clientError{
			code: resp.StatusCode,
		}
	}

	return resp, body, err
}

// StatusAPI provides bindings for the Alertmanager's status API.
type StatusAPI interface {
	// Health will check prometheus health.
	Health(ctx context.Context) (int, error)
	// Get returns the server's configuration, version, uptime and cluster information.
	Get(ctx context.Context) (*ServerStatus, error)
}

// NewStatusAPI returns a status API client.
func NewStatusAPI(c api.Client) StatusAPI {
	return &httpStatusAPI{client: apiClient{c}}
}

type httpStatusAPI struct {
	client api.Client
}

func (h *httpStatusAPI) Health(ctx context.Context) (int, error) {
	u := h.client.URL(epStatus, nil)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}

	code := 0
	res, _, err := h.client.Do(ctx, req)
	if res != nil {
		code = res.StatusCode
	}
	return code, err
}

func (h *httpStatusAPI) Get(ctx context.Context) (*ServerStatus, error) {
	u := h.client.URL(epStatus, nil)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var ss *ServerStatus
	err = json.Unmarshal(body, &ss)

	return ss, err
}

// AlertAPI provides bindings for the Alertmanager's alert API.
type AlertAPI interface {
	// List returns all the active alerts.
	List(ctx context.Context, filter []string, receiver string, silenced, inhibited, active, unprocessed bool) ([]*ExtendedAlert, error)
	// Push sends a list of alerts to the Alertmanager.
	Push(ctx context.Context, alerts ...Alert) error
}

// Alert represents an alert as expected by the AlertManager's push alert API.
type Alert struct {
	Labels       LabelSet  `json:"labels"`
	Annotations  LabelSet  `json:"annotations"`
	StartsAt     time.Time `json:"startsAt,omitempty"`
	EndsAt       time.Time `json:"endsAt,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
	GeneratorURL string    `json:"generatorURL"`
}

// ExtendedAlert represents an alert as returned by the AlertManager's list alert API.
type ExtendedAlert struct {
	Alert
	Status      types.AlertStatus `json:"status"`
	Receivers   []Receiver        `json:"receivers"`
	Fingerprint string            `json:"fingerprint"`
}

// Receiver represents a receiver.
type Receiver struct {
	Name string `json:"name"`
}

// LabelSet represents a collection of label names and values as a map.
type LabelSet map[LabelName]LabelValue

// LabelName represents the name of a label.
type LabelName string

// LabelValue represents the value of a label.
type LabelValue string

// NewAlertAPI returns a new AlertAPI for the client.
func NewAlertAPI(c api.Client) AlertAPI {
	return &httpAlertAPI{client: apiClient{c}}
}

type httpAlertAPI struct {
	client api.Client
}

func (h *httpAlertAPI) List(ctx context.Context, filter []string, receiver string, silenced, inhibited, active, unprocessed bool) ([]*ExtendedAlert, error) {
	u := h.client.URL(epAlerts, nil)
	params := url.Values{}
	for _, f := range filter {
		params.Add("filter", f)
	}
	params.Add("silenced", fmt.Sprintf("%t", silenced))
	params.Add("inhibited", fmt.Sprintf("%t", inhibited))
	params.Add("active", fmt.Sprintf("%t", active))
	params.Add("unprocessed", fmt.Sprintf("%t", unprocessed))
	params.Add("receiver", receiver)
	u.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	_, body, err := h.client.Do(ctx, req) // ignoring warnings.
	if err != nil {
		return nil, err
	}

	var alts []*ExtendedAlert
	err = json.Unmarshal(body, &alts)

	return alts, err
}

func (h *httpAlertAPI) Push(ctx context.Context, alerts ...Alert) error {
	u := h.client.URL(epAlerts, nil)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&alerts); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), &buf)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	_, _, err = h.client.Do(ctx, req)
	return err
}

// SilenceAPI provides bindings for the Alertmanager's silence API.
type SilenceAPI interface {
	// Get returns the silence associated with the given ID.
	Get(ctx context.Context, id string) (*types.Silence, error)
	// Set updates or creates the given silence and returns its ID.
	Set(ctx context.Context, sil types.Silence) (string, error)
	// Expire expires the silence with the given ID.
	Expire(ctx context.Context, id string) error
	// List returns silences matching the given filter.
	List(ctx context.Context, filter []string) ([]*types.Silence, error)
}

// NewSilenceAPI returns a new SilenceAPI for the client.
func NewSilenceAPI(c api.Client) SilenceAPI {
	return &httpSilenceAPI{client: apiClient{c}}
}

type httpSilenceAPI struct {
	client api.Client
}

func (h *httpSilenceAPI) Get(ctx context.Context, id string) (*types.Silence, error) {
	u := h.client.URL(epSilence, map[string]string{
		"id": id,
	})

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var sil types.Silence
	err = json.Unmarshal(body, &sil)

	return &sil, err
}

func (h *httpSilenceAPI) Expire(ctx context.Context, id string) error {
	u := h.client.URL(epSilence, map[string]string{
		"id": id,
	})

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	_, _, err = h.client.Do(ctx, req)
	return err
}

func (h *httpSilenceAPI) Set(ctx context.Context, sil types.Silence) (string, error) {
	u := h.client.URL(epSilences, nil)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&sil); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), &buf)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return "", err
	}

	var res struct {
		SilenceID string `json:"silenceId"`
	}
	err = json.Unmarshal(body, &res)

	return res.SilenceID, err
}

func (h *httpSilenceAPI) List(ctx context.Context, filter []string) ([]*types.Silence, error) {
	u := h.client.URL(epSilences, nil)
	params := url.Values{}
	for _, f := range filter {
		params.Add("filter", f)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var sils []*types.Silence
	err = json.Unmarshal(body, &sils)

	return sils, err
}
