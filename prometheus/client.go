// Copyright 2015 The Prometheus Authors
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

// Package api provides clients for the HTTP APIs.
package prometheus

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/levigross/grequests"
)

// DefaultRoundTripper is used if no RoundTripper is set in Config.
var DefaultRoundTripper http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSHandshakeTimeout: 10 * time.Second,
	TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
}

// Config defines configuration parameters for a new client.
type Config struct {
	// The address of the Prometheus to connect to.
	Address string

	// The bearer token for the Prometheus to connect to.
	BearerToken string

	// The username of basic auth for the Prometheus to connect to.
	Username string

	// The password of basic auth for the Prometheus to connect to.
	Password string

	// The timeout for the Prometheus to connect to.
	Timeout int

	// RoundTripper is used by the Client to drive HTTP requests. If not
	// provided, DefaultRoundTripper will be used.
	RoundTripper http.RoundTripper
}

func (cfg *Config) roundTripper() http.RoundTripper {
	if cfg.RoundTripper == nil {
		return DefaultRoundTripper
	}
	return cfg.RoundTripper
}

// Client is the interface for an API client.
type Client interface {
	URL(ep string, args map[string]string) *url.URL
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
	Proxy(method string, url string, params map[string]string, data map[string]string) (*grequests.Response, error)
}

// NewClient returns a new Client.
//
// It is safe to use the returned Client from multiple goroutines.
func NewClient(cfg Config) (Client, error) {
	u, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/")

	return &httpClient{
		endpoint:    u,
		bearerToken: cfg.BearerToken,
		username:    cfg.Username,
		password:    cfg.Password,
		timeout:     cfg.Timeout,
		client:      http.Client{Transport: cfg.roundTripper()},
	}, nil
}

type httpClient struct {
	endpoint    *url.URL
	username    string
	password    string
	bearerToken string
	timeout     int
	client      http.Client
}

func (c *httpClient) URL(ep string, args map[string]string) *url.URL {
	p := path.Join(c.endpoint.Path, ep)

	for arg, val := range args {
		arg = ":" + arg
		p = strings.Replace(p, arg, val, -1)
	}

	u := *c.endpoint
	u.Path = p

	return &u
}

func (c *httpClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	} else if c.bearerToken != "" {
		req.Header.Add("Authorization", "Bearer "+c.bearerToken)
	}
	resp, err := c.client.Do(req)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	if err != nil {
		return nil, nil, err
	}

	var body []byte
	done := make(chan struct{})
	go func() {
		body, err = io.ReadAll(resp.Body)
		close(done)
	}()

	select {
	case <-ctx.Done():
		err = resp.Body.Close()
		<-done
		if err == nil {
			err = ctx.Err()
		}
	case <-done:
	}

	return resp, body, err
}

func (c *httpClient) Proxy(method string, url string, params map[string]string, data map[string]string) (*grequests.Response, error) {
	url = c.endpoint.String() + url
	var err error
	var response *grequests.Response
	requestOptions := &grequests.RequestOptions{
		Data:               data,
		Params:             params,
		RequestTimeout:     time.Duration(c.timeout) * time.Second,
		InsecureSkipVerify: true,
	}
	if c.username != "" && c.password != "" {
		requestOptions.Auth = []string{c.username, c.password}
	} else if c.bearerToken != "" {
		requestOptions.Headers = map[string]string{"Authorization": fmt.Sprintf("Bearer %s", c.bearerToken)}
	}

	switch method {
	case http.MethodGet:
		response, err = grequests.Get(url, requestOptions)
	case http.MethodPost:
		response, err = grequests.Post(url, requestOptions)
	case http.MethodPut:
		response, err = grequests.Put(url, requestOptions)
	case http.MethodDelete:
		response, err = grequests.Delete(url, requestOptions)
	default:
		return nil, fmt.Errorf("oh no, method %s has not been implemented", method)
	}

	return response, err
}
