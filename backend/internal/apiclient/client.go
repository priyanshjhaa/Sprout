// Package apiclient is a typed client for the Sprout API, used by the CLI and,
// later, the MCP deploy tool. It speaks the same public contract as the
// dashboard (backend/openapi/openapi.yaml) and authenticates with a personal
// access token. It never follows redirects and refuses plain HTTP to anything
// but this machine, so a token is only ever sent to the server it was meant for.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// maxJSONBytes bounds a JSON response; every response the client reads is small.
const maxJSONBytes = 1 << 20

var ErrInvalidBaseURL = errors.New("the API address must be http(s)://host[:port], and plain http is only allowed for this machine")

// NormalizeBaseURL checks an API address and returns it as scheme://host[:port].
// Plain http is accepted only for loopback hosts: anywhere else a token would
// cross the network readable by anyone on the path.
func NormalizeBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return "", ErrInvalidBaseURL
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopback(parsed.Hostname()) {
			return "", ErrInvalidBaseURL
		}
	default:
		return "", ErrInvalidBaseURL
	}
	return parsed.Scheme + "://" + strings.ToLower(parsed.Host), nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Error is a failure reported by the API in its error envelope, or a response
// the client could not understand (code unexpected_response).
type Error struct {
	Status    int
	Code      string
	Message   string
	RequestID string
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s (%s, request %s)", e.Message, e.Code, e.RequestID)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// HasCode reports whether err is an API error with the given code.
func HasCode(err error, code string) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Code == code
}

type Client struct {
	baseURL   string
	token     string
	userAgent string
	http      *http.Client
}

// New returns a client for an API address accepted by NormalizeBaseURL.
func New(baseURL, token, userAgent string) (*Client, error) {
	normalized, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:   normalized,
		token:     token,
		userAgent: userAgent,
		http: &http.Client{
			// The API never redirects; a redirect is a misconfiguration or an
			// attempt to move the token somewhere else.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

type Workspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type Me struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Workspace   Workspace `json:"workspace"`
}

// Me resolves the token's owner and home workspace.
func (c *Client) Me(ctx context.Context) (Me, error) {
	var me Me
	err := c.do(ctx, http.MethodGet, "/api/v1/me", nil, "", http.StatusOK, &me)
	return me, err
}

// Capabilities reports the optional features this API process offers.
func (c *Client) Capabilities(ctx context.Context) (Capabilities, error) {
	var capabilities Capabilities
	err := c.do(ctx, http.MethodGet, "/api/v1/capabilities", nil, "", http.StatusOK, &capabilities)
	return capabilities, err
}

type Capabilities struct {
	LocalBuilds bool `json:"localBuilds"`
}

type Application struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	Lifecycle string  `json:"lifecycle"`
	URL       *string `json:"url"`
}

func applicationsPath(workspace string) string {
	return "/api/v1/workspaces/" + url.PathEscape(workspace) + "/applications"
}

func applicationPath(workspace, applicationID string) string {
	return applicationsPath(workspace) + "/" + url.PathEscape(applicationID)
}

func (c *Client) ListApplications(ctx context.Context, workspace string) ([]Application, error) {
	var list struct {
		Applications []Application `json:"applications"`
	}
	err := c.do(ctx, http.MethodGet, applicationsPath(workspace), nil, "", http.StatusOK, &list)
	return list.Applications, err
}

func (c *Client) GetApplication(ctx context.Context, workspace, applicationID string) (Application, error) {
	var application Application
	err := c.do(ctx, http.MethodGet, applicationPath(workspace, applicationID), nil, "", http.StatusOK, &application)
	return application, err
}

func (c *Client) CreateApplication(ctx context.Context, workspace, name, slug string) (Application, error) {
	body, err := json.Marshal(map[string]string{"name": name, "slug": slug})
	if err != nil {
		return Application{}, err
	}
	var application Application
	err = c.do(ctx, http.MethodPost, applicationsPath(workspace), bytes.NewReader(body), "application/json", http.StatusCreated, &application)
	return application, err
}

// Deployment is a real build; see the Deployment schema in the OpenAPI spec.
type Deployment struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	FailureCode *string `json:"failureCode"`
	Stages      []Stage `json:"stages"`
}

type Stage struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	FailureCode *string `json:"failureCode"`
}

// Done reports whether the deployment has reached a final status.
func (d Deployment) Done() bool {
	switch d.Status {
	case "succeeded", "live", "failed", "cancelled":
		return true
	}
	return false
}

// CreateDeployment uploads a source archive (a plain tar) and queues a build.
func (c *Client) CreateDeployment(ctx context.Context, workspace, applicationID string, archive []byte) (Deployment, error) {
	var deployment Deployment
	err := c.do(ctx, http.MethodPost, applicationPath(workspace, applicationID)+"/deployments", bytes.NewReader(archive), "application/x-tar", http.StatusCreated, &deployment)
	return deployment, err
}

func (c *Client) GetDeployment(ctx context.Context, workspace, applicationID, deploymentID string) (Deployment, error) {
	var deployment Deployment
	err := c.do(ctx, http.MethodGet, applicationPath(workspace, applicationID)+"/deployments/"+url.PathEscape(deploymentID), nil, "", http.StatusOK, &deployment)
	return deployment, err
}

// do sends one request and decodes a JSON response with the expected status.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, want int, into any) error {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxJSONBytes)
	if response.StatusCode != want {
		return decodeError(response.StatusCode, limited)
	}
	if into == nil {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(into); err != nil {
		return &Error{Status: response.StatusCode, Code: "unexpected_response", Message: "The API sent a response this client could not read."}
	}
	return nil
}

func decodeError(status int, body io.Reader) error {
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if json.NewDecoder(body).Decode(&envelope) != nil || envelope.Error.Code == "" {
		return &Error{Status: status, Code: "unexpected_response", Message: fmt.Sprintf("The API answered with HTTP %d.", status)}
	}
	return &Error{Status: status, Code: envelope.Error.Code, Message: envelope.Error.Message, RequestID: envelope.Error.RequestID}
}
