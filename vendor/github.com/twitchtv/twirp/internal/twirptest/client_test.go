// Copyright 2018 Twitch Interactive, Inc.  All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the License is
// located at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package twirptest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"
)

// reqInspector is a tool to check inspect HTTP Requests as they pass
// through an http.Client. It implements the http.RoundTripper
// interface by calling its callback, and then using the default
// RoundTripper.
type reqInspector struct {
	callback func(*http.Request)
}

func (i *reqInspector) RoundTrip(r *http.Request) (*http.Response, error) {
	i.callback(r)
	return http.DefaultTransport.RoundTrip(r)
}

func TestClientSuccessAndErrorResponses(t *testing.T) {
	// Service that succeeds only if requested size is 1, otherwise errors
	h := PickyHatmaker(1)
	s := httptest.NewServer(NewHaberdasherServer(h, nil))
	defer s.Close()

	// Clients
	protoCli := NewHaberdasherProtobufClient(s.URL, &http.Client{})
	jsonCli := NewHaberdasherJSONClient(s.URL, &http.Client{})
	ctx := context.Background()
	var resp *Hat
	var err error

	// Test proto success
	resp, err = protoCli.MakeHat(ctx, &Size{Inches: 1})
	if err != nil {
		t.Fatalf("Proto client method returned unexpected error: %s", err)
	}
	if resp == nil {
		t.Fatalf("Proto client method expected to return non-nil response, but it is nil")
	}

	// Test proto failure
	resp, err = protoCli.MakeHat(ctx, &Size{Inches: 666})
	if err == nil {
		t.Fatalf("Proto client method expected to fail, but error is nil")
	}
	if resp != nil {
		t.Fatalf("Proto client method expected to return nil response on error, but returned non-nil")
	}

	// Test json success
	resp, err = jsonCli.MakeHat(ctx, &Size{Inches: 1})
	if err != nil {
		t.Fatalf("JSON client method returned unexpected error: %s", err)
	}
	if resp == nil {
		t.Fatalf("JSON client method expected to return non-nil response, but it is nil")
	}

	// Test json failure
	resp, err = jsonCli.MakeHat(ctx, &Size{Inches: 666})
	if err == nil {
		t.Fatalf("JSON client method expected to fail, but error is nil")
	}
	if resp != nil {
		t.Fatalf("JSON client method expected to return nil response on error, but returned non-nil")
	}
}

func TestClientSetsRequestContext(t *testing.T) {
	// Start up a server just so we can make a working client later.
	h := PickyHatmaker(1)
	s := httptest.NewServer(NewHaberdasherServer(h, nil))
	defer s.Close()

	// Make an *http.Client that validates that the key-value is present
	// in the context.
	httpClient := &http.Client{
		Transport: &reqInspector{
			callback: func(req *http.Request) {
				ctx := req.Context()

				pkgName, exists := twirp.PackageName(ctx)
				if !exists {
					t.Error("packageName not found in context")
					return
				}
				if pkgName != "twirp.internal.twirptest" {
					t.Errorf("packageName has wrong value, have=%s, want=%s", pkgName, "twirp.internal.twirptest")
				}

				serviceName, exists := twirp.ServiceName(ctx)
				if !exists {
					t.Error("serviceName not found in context")
					return
				}
				if serviceName != "Haberdasher" {
					t.Errorf("serviceName has wrong value, have=%s, want=%s", pkgName, "Haberdasher")
				}

				methodName, exists := twirp.MethodName(ctx)
				if !exists {
					t.Error("methodName not found in context")
					return
				}
				if methodName != "MakeHat" {
					t.Errorf("methodName has wrong value, have=%s, want=%s", pkgName, "Haberdasher")
				}
			},
		},
	}

	// Test the JSON client and the Protobuf client.
	client := NewHaberdasherJSONClient(s.URL, httpClient)

	_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Errorf("MakeHat err=%s", err)
	}

	client = NewHaberdasherProtobufClient(s.URL, httpClient)

	_, err = client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Errorf("MakeHat err=%s", err)
	}
}

func TestClientSetsAcceptHeader(t *testing.T) {
	// Start up a server just so we can make a working client later.
	h := PickyHatmaker(1)
	s := httptest.NewServer(NewHaberdasherServer(h, nil))
	defer s.Close()

	// Make an *http.Client that validates that the correct accept header is present
	// in the request.
	httpClient := &http.Client{
		Transport: &reqInspector{
			callback: func(req *http.Request) {
				if req.Header.Get("Accept") != "application/json" {
					t.Error("Accept header not found in req")
					return
				}
			},
		},
	}

	// Test the JSON client
	client := NewHaberdasherJSONClient(s.URL, httpClient)

	_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Errorf("MakeHat err=%s", err)
	}

	// Make an *http.Client that validates that the correct accept header is present
	// in the request.
	httpClient = &http.Client{
		Transport: &reqInspector{
			callback: func(req *http.Request) {
				if req.Header.Get("Accept") != "application/protobuf" {
					t.Error("Accept header not found in req")
					return
				}
			},
		},
	}

	// test the Protobuf client.
	client = NewHaberdasherProtobufClient(s.URL, httpClient)

	_, err = client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Errorf("MakeHat err=%s", err)
	}
}

// If a server returns a 3xx response, give a clear error message
func TestClientRedirectError(t *testing.T) {
	testcase := func(code int, clientMaker func(string, HTTPClient, ...twirp.ClientOption) Haberdasher) func(*testing.T) {
		return func(t *testing.T) {
			// Make a server that redirects all requests
			redirecter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "http://bogus/notreal", code)
			})
			s := httptest.NewServer(redirecter)
			defer s.Close()

			client := clientMaker(s.URL, http.DefaultClient)
			_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
			if err == nil {
				t.Fatal("MakeHat err=nil, expected an error because redirects aren't allowed")
			}
			if twerr, ok := err.(twirp.Error); !ok {
				t.Fatalf("expected twirp.Error typed err, have=%T", err)
			} else {
				// error message should mention the code
				if !strings.Contains(twerr.Error(), strconv.Itoa(code)) {
					t.Errorf("expected error message to mention the status code, but its missing: %q", twerr)
				}
				// error message should mention the redirect location
				if !strings.Contains(twerr.Error(), "http://bogus/notreal") {
					t.Errorf("expected error message to mention the redirect location, but its missing: %q", twerr)
				}
				// error meta should include http_error_from_intermediary
				if twerr.Meta("http_error_from_intermediary") != "true" {
					t.Errorf("expected error.Meta('http_error_from_intermediary') to be %q, but found %q", "true", twerr.Meta("http_error_from_intermediary"))
				}
				// error meta should include status
				if twerr.Meta("status_code") != strconv.Itoa(code) {
					t.Errorf("expected error.Meta('status_code') to be %q, but found %q", code, twerr.Meta("status_code"))
				}
				// error meta should include location
				if twerr.Meta("location") != "http://bogus/notreal" {
					t.Errorf("expected error.Meta('location') to be the redirect from intermediary, but found %q", twerr.Meta("location"))
				}
			}
		}
	}

	// It's important to test all redirect codes because Go actually handles them differently. 302 and
	// 303 get automatically redirected, even POSTs. The others do not (although this may change in
	// go1.8). We want all of them to have the same output.
	t.Run("json client", func(t *testing.T) {
		for code := 300; code <= 308; code++ {
			t.Run(strconv.Itoa(code), testcase(code, NewHaberdasherJSONClient))
		}
	})
	t.Run("protobuf client", func(t *testing.T) {
		for code := 300; code <= 308; code++ {
			t.Run(strconv.Itoa(code), testcase(code, NewHaberdasherProtobufClient))
		}
	})
}

func TestClientWithHooks(t *testing.T) {
	tests := []struct {
		desc                       string
		in                         *Size
		requestPreparedError       error
		wantRequestPreparedCalled  bool
		wantResponseReceivedCalled bool
		wantErrorCalled            bool
	}{
		{
			desc:                       "calls ResponseReceived and RequestPrepared hooks but not Error for successful calls",
			in:                         &Size{Inches: 1},
			wantRequestPreparedCalled:  true,
			wantResponseReceivedCalled: true,
			wantErrorCalled:            false,
		},
		{
			desc:                       "calls RequestPrepared and Error hooks for errored calls",
			in:                         &Size{Inches: 666},
			wantRequestPreparedCalled:  true,
			wantResponseReceivedCalled: false,
			wantErrorCalled:            true,
		},
		{
			desc:                       "calls RequestPrepared and Error hooks for error in hook",
			in:                         &Size{Inches: 1},
			wantRequestPreparedCalled:  true,
			requestPreparedError:       errors.New("test"),
			wantResponseReceivedCalled: false,
			wantErrorCalled:            true,
		},
	}

	for _, tt := range tests {
		h := PickyHatmaker(1)
		s := httptest.NewServer(NewHaberdasherServer(h, nil))
		defer s.Close()
		t.Run(tt.desc, func(t *testing.T) {
			requestPreparedCalled := false
			responseReceivedCalled := false
			errorCalled := false

			hooks := &twirp.ClientHooks{
				RequestPrepared: func(ctx context.Context, req *http.Request) (context.Context, error) {
					requestPreparedCalled = true
					return ctx, tt.requestPreparedError
				},
				ResponseReceived: func(ctx context.Context) {
					responseReceivedCalled = true
				},
				Error: func(ctx context.Context, err twirp.Error) {
					errorCalled = true
				},
			}

			// Clients
			protoCli := NewHaberdasherProtobufClient(s.URL, &http.Client{}, twirp.WithClientHooks(hooks))
			ctx := context.Background()

			_, err := protoCli.MakeHat(ctx, tt.in)
			if tt.wantErrorCalled && err == nil {
				t.Error("unexpected nil error")
			}
			if !tt.wantErrorCalled && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantRequestPreparedCalled != requestPreparedCalled {
				t.Errorf("unexpected value for requestPreparedCalled: got %t, want %t", requestPreparedCalled, tt.wantRequestPreparedCalled)
			}

			if tt.wantResponseReceivedCalled != responseReceivedCalled {
				t.Errorf("unexpected value for responseReceivedCalled: got %t, want %t", responseReceivedCalled, tt.wantResponseReceivedCalled)
			}

			if tt.wantErrorCalled != errorCalled {
				t.Errorf("unexpected value for errorCalled: got %t, want %t", errorCalled, tt.wantErrorCalled)
			}

			requestPreparedCalled = false
			responseReceivedCalled = false
			errorCalled = false

			jsonCli := NewHaberdasherJSONClient(s.URL, &http.Client{}, twirp.WithClientHooks(hooks))
			_, err = jsonCli.MakeHat(ctx, tt.in)
			if tt.wantErrorCalled && err == nil {
				t.Error("unexpected nil error")
			}
			if !tt.wantErrorCalled && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantRequestPreparedCalled != requestPreparedCalled {
				t.Errorf("unexpected value for requestPreparedCalled: got %t, want %t", requestPreparedCalled, tt.wantRequestPreparedCalled)
			}

			if tt.wantResponseReceivedCalled != responseReceivedCalled {
				t.Errorf("unexpected value for responseReceivedCalled: got %t, want %t", responseReceivedCalled, tt.wantResponseReceivedCalled)
			}

			if tt.wantErrorCalled != errorCalled {
				t.Errorf("unexpected value for errorCalled: got %t, want %t", errorCalled, tt.wantErrorCalled)
			}
		})
	}
}

func TestClientIntermediaryErrors(t *testing.T) {
	testcase := func(body string, code int, expectedErrorCode twirp.ErrorCode, clientMaker func(string, HTTPClient, ...twirp.ClientOption) Haberdasher) func(*testing.T) {
		return func(t *testing.T) {
			// Make a server that returns invalid twirp error responses,
			// simulating a network intermediary.
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				_, err := w.Write([]byte(body))
				if err != nil {
					t.Fatalf("Unexpected error: %s", err.Error())
				}
			}))
			defer s.Close()

			client := clientMaker(s.URL, http.DefaultClient)
			_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
			if err == nil {
				t.Fatal("Expected error, but found nil")
			}
			if twerr, ok := err.(twirp.Error); !ok {
				t.Fatalf("expected twirp.Error typed err, have=%T", err)
			} else {
				// error message should mention the code
				if !strings.Contains(twerr.Msg(), fmt.Sprintf("Error from intermediary with HTTP status code %d", code)) {
					t.Errorf("unexpected error message: %q", twerr.Msg())
				}
				// error meta should include http_error_from_intermediary
				if twerr.Meta("http_error_from_intermediary") != "true" {
					t.Errorf("expected error.Meta('http_error_from_intermediary') to be %q, but found %q", "true", twerr.Meta("http_error_from_intermediary"))
				}
				// error meta should include status
				if twerr.Meta("status_code") != strconv.Itoa(code) {
					t.Errorf("expected error.Meta('status_code') to be %q, but found %q", code, twerr.Meta("status_code"))
				}
				// error meta should include body
				if twerr.Meta("body") != body {
					t.Errorf("expected error.Meta('body') to be the response from intermediary, but found %q", twerr.Meta("body"))
				}
				// error code should be properly mapped from HTTP Code
				if twerr.Code() != expectedErrorCode {
					t.Errorf("expected to map HTTP status %q to twirp.ErrorCode %q, but found %q", code, expectedErrorCode, twerr.Code())
				}
			}
		}
	}

	// HTTP Status Code -> desired Twirp Error Code
	statusCodes := map[int]twirp.ErrorCode{
		// Map meaningful HTTP codes to semantic equivalent twirp.ErrorCodes
		400: twirp.Internal,
		401: twirp.Unauthenticated,
		403: twirp.PermissionDenied,
		404: twirp.BadRoute,
		429: twirp.Unavailable,
		502: twirp.Unavailable,
		503: twirp.Unavailable,
		504: twirp.Unavailable,

		// all other codes are unknown
		505: twirp.Unknown,
		410: twirp.Unknown,
		408: twirp.Unknown,
	}

	// label -> http response body
	bodies := map[string]string{
		"text":        "error from intermediary",
		"emptyjson":   "{}",
		"invalidjson": `{"message":"Signature expired: 19700101T000000Z is now earlier than 20190612T110154Z (20190612T110654Z - 5 min.)"}`,
	}

	clients := map[string]func(string, HTTPClient, ...twirp.ClientOption) Haberdasher{
		"json_client":  NewHaberdasherJSONClient,
		"proto_client": NewHaberdasherProtobufClient,
	}

	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			for name, body := range bodies {
				t.Run(name, func(t *testing.T) {
					for httpcode, twirpcode := range statusCodes {
						t.Run(fmt.Sprintf("%d_to_%s", httpcode, twirpcode),
							testcase(body, httpcode, twirpcode, client))
					}
				})
			}
		})
	}
}

func TestJSONClientAllowUnknownFields(t *testing.T) {
	// Make a server that always returns JSON with extra fields
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json := `{"size":1, "color":"black", "extra1":"foo", "EXTRAMORE":"bar"}`

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(json))
		if err != nil {
			t.Fatalf("Unexpected error: %s", err.Error())
		}
	}))
	defer s.Close()

	client := NewHaberdasherJSONClient(s.URL, http.DefaultClient)
	resp, err := client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	// resp should have the values from the response json
	if resp.Size != 1 {
		t.Errorf("expected resp.Size to be %d, found %d", 1, resp.Size)
	}
	if resp.Color != "black" {
		t.Errorf("expected resp.Color to be %q, found %q", "black", resp.Color)
	}
	if resp.Name != "" { // not included in the response, should default to zero-value
		t.Errorf("expected resp.Name to be empty (zero-value), found %q", resp.Name)
	}
}

func TestClientErrorsCanBeCaused(t *testing.T) {
	rootErr := fmt.Errorf("some root cause")
	httpClient := &http.Client{
		Transport: &failingTransport{rootErr},
	}

	client := NewHaberdasherJSONClient("", httpClient)
	_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
	if err == nil {
		t.Errorf("JSON MakeHat err is unexpectedly nil")
	}
	cause := errCause(err)
	if cause != rootErr {
		t.Errorf("JSON MakeHat err cause is %q, want %q", cause, rootErr)
	}

	client = NewHaberdasherProtobufClient("", httpClient)
	_, err = client.MakeHat(context.Background(), &Size{Inches: 1})
	if err == nil {
		t.Errorf("Protobuf MakeHat err is unexpectedly nil")
	}
	cause = errCause(err)
	if cause != rootErr {
		t.Errorf("Protobuf MakeHat err cause is %q, want %q", cause, rootErr)
	}
}

func TestCustomHTTPClientInterface(t *testing.T) {
	// Start up a server just so we can make a working client later.
	h := PickyHatmaker(1)
	s := httptest.NewServer(NewHaberdasherServer(h, nil))
	defer s.Close()

	// Create a custom wrapper to wrap our default client
	httpClient := &wrappedHTTPClient{
		client:    http.DefaultClient,
		wasCalled: false,
	}

	// Test the JSON client and the Protobuf client with a custom http.Client interface
	client := NewHaberdasherJSONClient(s.URL, httpClient)

	_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Errorf("MakeHat err=%s", err)
	}

	// Check if the Do function within the http.Client wrapper gets actually called
	if !httpClient.wasCalled {
		t.Errorf("HTTPClient.Do function was not called within the JSONClient")
	}

	// Reset bool for second test
	httpClient.wasCalled = false

	client = NewHaberdasherProtobufClient(s.URL, httpClient)

	_, err = client.MakeHat(context.Background(), &Size{Inches: 1})
	if err != nil {
		t.Errorf("MakeHat err=%s", err)
	}

	// Check if the Do function within the http.Client wrapper gets actually called
	if !httpClient.wasCalled {
		t.Errorf("HTTPClient.Do function was not called within the ProtobufClient")
	}
}

// failingTransport is a http.RoundTripper which always returns an error.
type failingTransport struct {
	err error // the error to return
}

func (t failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

func errCause(err error) error {
	cause := errors.Cause(err)
	if uerr, ok := cause.(*url.Error); ok {
		// in go1.8+, http.Client errors are wrapped in *url.Error
		cause = uerr.Err
	}
	return cause
}

// wrappedHTTPClient implements HTTPClient, but can be inspected during tests.
type wrappedHTTPClient struct {
	client    *http.Client
	wasCalled bool
}

func (c *wrappedHTTPClient) Do(req *http.Request) (resp *http.Response, err error) {
	c.wasCalled = true
	return c.client.Do(req)
}

func TestClientReturnsCloseErrors(t *testing.T) {
	h := PickyHatmaker(1)
	s := httptest.NewServer(NewHaberdasherServer(h, nil))
	defer s.Close()

	httpClient := &bodyCloseErrClient{base: http.DefaultClient}

	testcase := func(client Haberdasher) func(*testing.T) {
		return func(t *testing.T) {
			_, err := client.MakeHat(context.Background(), &Size{Inches: 1})
			if err == nil {
				t.Error("expected an error when body fails to close, have nil")
			} else {
				if errors.Cause(err) != bodyCloseErr {
					t.Errorf("got wrong root cause for error, have=%v, want=%v", err, bodyCloseErr)
				}
			}
		}
	}
	t.Run("json client", testcase(NewHaberdasherJSONClient(s.URL, httpClient)))
	t.Run("protobuf client", testcase(NewHaberdasherProtobufClient(s.URL, httpClient)))
}

// bodyCloseErrClient implements HTTPClient, but the response bodies it returns
// give an error when they are closed.
type bodyCloseErrClient struct {
	base HTTPClient
}

func (c *bodyCloseErrClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.base.Do(req)
	if resp == nil {
		return resp, err
	}
	resp.Body = &errBodyCloser{resp.Body}
	return resp, nil
}

var bodyCloseErr = errors.New("failed closing")

type errBodyCloser struct {
	base io.ReadCloser
}

func (ec *errBodyCloser) Read(p []byte) (int, error) {
	return ec.base.Read(p)
}

func (ec *errBodyCloser) Close() error {
	return bodyCloseErr
}
