// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package m3admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"

	"github.com/gogo/protobuf/proto"
	retryhttp "github.com/hashicorp/go-retryablehttp"
	pkgerrors "github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	m3EnvironmentHeader = "Cluster-Environment-Name"
)

var (
	// ErrNotOk indicates that HTTP status was not Ok
	ErrNotOk = errors.New("status not ok")

	// ErrNotFound indicates that HTTP status was not found
	ErrNotFound = errors.New("status not found")

	// ErrMethodNotAllowed indicates that HTTP status was method not allowed
	ErrMethodNotAllowed = errors.New("method not allowed")
)

// Client is an m3admin client.
type Client interface {
	DoHTTPRequest(action, url string, data *bytes.Buffer, opts ...RequestOption) (*http.Response, error)
	DoHTTPJSONPBRequest(action, url string, request, response proto.Message, opts ...RequestOption) error
}

type client struct {
	client      *retryhttp.Client
	logger      *zap.Logger
	environment string
}

type nullLogger struct{}

func (nullLogger) Printf(string, ...interface{}) {}

// NewClient returns a new m3admin client.
func NewClient(clientOpts ...Option) Client {
	opts := &options{}
	for _, o := range clientOpts {
		o.execute(opts)
	}

	client := &client{
		client:      opts.client,
		logger:      opts.logger,
		environment: opts.environment,
	}

	if client.client == nil {
		client.client = retryhttp.NewClient()
	}
	if client.logger == nil {
		client.logger = zap.NewNop()
	}

	// We do our own request logging, silence their logger.
	client.client.Logger = nullLogger{}
	client.client.ErrorHandler = retryhttp.PassthroughErrorHandler

	return client
}

// DoHTTPRequest is a simple helper for HTTP requests
func (c *client) DoHTTPRequest(
	action, url string,
	data *bytes.Buffer,
	options ...RequestOption,
) (*http.Response, error) {
	l := c.logger.With(zap.String("action", action), zap.String("url", url))
	opts := &reqOptions{}
	for _, o := range options {
		o.execute(opts)
	}

	var request *retryhttp.Request
	var err error

	// retryhttp type switches on the data parameter, if data is types as
	// *bytes.Buffer but nil it will panic
	if data == nil {
		request, err = retryhttp.NewRequest(action, url, nil)
		if err != nil {
			return nil, err
		}
	} else {
		request, err = retryhttp.NewRequest(action, url, data)
		if err != nil {
			return nil, err
		}
	}

	if opts.headers != nil {
		for k, v := range opts.headers {
			request.Header.Add(k, v)
		}
	}

	request.Header.Add("Content-Type", "application/json")
	if c.environment != "" {
		request.Header.Add(m3EnvironmentHeader, c.environment)
	}

	if l.Core().Enabled(zapcore.DebugLevel) {
		dump, err := httputil.DumpRequest(request.Request, true)
		if err != nil {
			l = l.With(zap.String("requestDumpError", err.Error()))
		} else {
			l = l.With(zap.ByteString("requestDump", dump))
		}
	}

	response, err := c.client.Do(request)
	if err != nil {
		l.Debug("request error", zap.Error(err))
		return nil, err
	}

	l = l.With(zap.String("status", response.Status))

	// If in debug mode, dump the entire request+response (coordinator error
	// messages are included in it).
	if l.Core().Enabled(zapcore.DebugLevel) {
		dump, err := httputil.DumpResponse(response, true)
		if err != nil {
			l = l.With(zap.String("responseDumpError", err.Error()))
		} else {
			l = l.With(zap.ByteString("responseDump", dump))
		}
	}

	l.Debug("response received")

	code := response.StatusCode
	if code >= 200 && code < 300 {
		return response, nil
	}

	// attempt to parse our error message
	errMsg, err := parseResponseError(response)
	if err != nil {
		l.Debug("error parsing error response", zap.Error(err))
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, pkgerrors.WithMessage(ErrNotFound, errMsg)
	}

	if response.StatusCode == http.StatusMethodNotAllowed {
		return nil, pkgerrors.WithMessage(ErrMethodNotAllowed, errMsg)
	}

	return nil, pkgerrors.WithMessage(ErrNotOk, errMsg)
}

// DoHTTPJSONPBRequest is a helper for performing a request and
// parsing the response as a JSONPB message into the response.
// Both request and response are optional and can be emitted if
// not wanting to either send or receive message.
func (c *client) DoHTTPJSONPBRequest(
	action, url string,
	request proto.Message,
	response proto.Message,
	opts ...RequestOption,
) error {
	var data *bytes.Buffer
	if request != nil {
		data = bytes.NewBuffer(nil)
		if err := JSONPBMarshal(data, request); err != nil {
			return err
		}
	}

	r, err := c.DoHTTPRequest(action, url, data, opts...)
	if err != nil {
		return err
	}

	defer func() {
		ioutil.ReadAll(r.Body)
		r.Body.Close()
	}()

	if response == nil {
		// Discard the body since nothing to decode into.
		return nil
	}

	return JSONPBUnmarshal(r.Body, response)
}

func parseResponseError(r *http.Response) (string, error) {
	defer func() {
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()
	}()

	respErr := struct {
		Error string `json:"error"`
	}{}

	err := json.NewDecoder(r.Body).Decode(&respErr)
	if err != nil {
		return "", err
	}

	return respErr.Error, nil
}
