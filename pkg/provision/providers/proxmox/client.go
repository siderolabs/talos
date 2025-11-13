// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proxmox

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Proxmox API client.
type Client struct {
	baseURL       string
	httpClient    *http.Client
	ticket        string
	csrfToken     string
	authenticated bool
}

// ProxmoxResponse is the standard Proxmox API response format.
type ProxmoxResponse struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error,omitempty"`
}

// VersionInfo contains Proxmox version information.
type VersionInfo struct {
	Release string `json:"release"`
	Version string `json:"version"`
	RepoID  string `json:"repoid"`
}

// NodeStatus contains Proxmox node status information.
type NodeStatus struct {
	Node   string  `json:"node"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    uint64  `json:"mem"`
	MaxMem uint64  `json:"maxmem"`
	Uptime uint64  `json:"uptime"`
}

// VMInfo contains VM information.
type VMInfo struct {
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    uint64  `json:"mem"`
	MaxMem uint64  `json:"maxmem"`
	Uptime uint64  `json:"uptime"`
}

// VMConfig is a map of VM configuration parameters.
type VMConfig map[string]interface{}

// VMStatus contains VM status information.
type VMStatus struct {
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    uint64  `json:"mem"`
	MaxMem uint64  `json:"maxmem"`
	Uptime uint64  `json:"uptime"`
}

// StorageInfo contains storage pool information.
type StorageInfo struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Used    uint64 `json:"used"`
	Total   uint64 `json:"total"`
}

// StorageContent contains storage content information.
type StorageContent struct {
	VolID  string `json:"volid"`
	Size   uint64 `json:"size"`
	Format string `json:"format"`
}

// TaskStatus contains task status information.
type TaskStatus struct {
	Status     string `json:"status"`
	Type       string `json:"type"`
	ExitStatus string `json:"exitstatus"`
}

// authTransport is a custom HTTP transport that adds authentication headers.
type authTransport struct {
	base   *http.Transport
	header string
	value  string
	csrf   string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.header == "Authorization" {
		req.Header.Set("Authorization", t.value)
	} else {
		req.Header.Set("Cookie", t.value)
		if t.csrf != "" {
			req.Header.Set("CSRFPreventionToken", t.csrf)
		}
	}
	return t.base.RoundTrip(req)
}

// NewClient creates a new Proxmox API client.
func NewClient(baseURL string, insecure bool) (*Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	baseURL = strings.TrimSuffix(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/api2/json") {
		if strings.HasSuffix(baseURL, "/") {
			baseURL += "api2/json"
		} else {
			baseURL += "/api2/json"
		}
	}

	return &Client{
		baseURL:       baseURL,
		httpClient:    client,
		authenticated: false,
	}, nil
}

// LoginWithToken authenticates using an API token.
func (c *Client) LoginWithToken(ctx context.Context, tokenID, secret string) error {
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", tokenID, secret)
	c.authenticated = true

	// Create a custom transport that adds the auth header
	tr := c.httpClient.Transport.(*http.Transport)
	c.httpClient.Transport = &authTransport{
		base:   tr,
		header: "Authorization",
		value:  authHeader,
	}

	return nil
}

// LoginWithUsernamePassword authenticates using username and password.
func (c *Client) LoginWithUsernamePassword(ctx context.Context, username, password string) error {
	loginURL := fmt.Sprintf("%s/access/ticket", c.baseURL)

	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var proxmoxResp ProxmoxResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxmoxResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if proxmoxResp.Error != "" {
		return fmt.Errorf("proxmox error: %s", proxmoxResp.Error)
	}

	var loginData struct {
		Ticket              string `json:"ticket"`
		CSRFPreventionToken string `json:"CSRFPreventionToken"`
	}

	if err := json.Unmarshal(proxmoxResp.Data, &loginData); err != nil {
		return fmt.Errorf("failed to unmarshal login data: %w", err)
	}

	c.ticket = loginData.Ticket
	c.csrfToken = loginData.CSRFPreventionToken
	c.authenticated = true

	// Create a custom transport that adds the auth headers
	tr := c.httpClient.Transport.(*http.Transport)
	c.httpClient.Transport = &authTransport{
		base:   tr,
		header: "Cookie",
		value:  fmt.Sprintf("PVEAuthCookie=%s", c.ticket),
		csrf:   c.csrfToken,
	}

	return nil
}

// doRequest performs an HTTP request to the Proxmox API.
func (c *Client) doRequest(ctx context.Context, method, path string, params url.Values, body io.Reader) (*ProxmoxResponse, error) {
	if !c.authenticated {
		return nil, fmt.Errorf("not authenticated")
	}

	reqURL := fmt.Sprintf("%s%s", c.baseURL, path)
	if params != nil && len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil && method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed (URL: %s, method: %s): %w\nPossible causes:\n• Network connectivity issues\n• Invalid PROXMOX_URL\n• TLS certificate problems (try PROXMOX_INSECURE=true)\n• Firewall blocking the connection", reqURL, method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		// Provide specific guidance based on status codes
		var guidance string
		switch resp.StatusCode {
		case 401:
			guidance = "Authentication failed - check PROXMOX_USERNAME and PROXMOX_PASSWORD"
		case 403:
			guidance = "Access forbidden - check user permissions in Proxmox"
		case 404:
			guidance = "API endpoint not found - check PROXMOX_URL and node name"
		case 500:
			guidance = "Proxmox server error - check Proxmox logs"
		case 596:
			guidance = "Connection failed - likely TLS or network issue (try PROXMOX_INSECURE=true)"
		default:
			guidance = "Check Proxmox API documentation for this status code"
		}

		return nil, fmt.Errorf("Proxmox API request failed (URL: %s, method: %s, status: %d): %s\nGuidance: %s",
			reqURL, method, resp.StatusCode, bodyStr, guidance)
	}

	var proxmoxResp ProxmoxResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxmoxResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if proxmoxResp.Error != "" {
		return nil, fmt.Errorf("proxmox error: %s", proxmoxResp.Error)
	}

	return &proxmoxResp, nil
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.doRequest(ctx, "GET", path, nil, nil)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(resp.Data, result); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return nil
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, params url.Values, result interface{}) error {
	var body io.Reader
	if params != nil {
		body = strings.NewReader(params.Encode())
	}

	resp, err := c.doRequest(ctx, "POST", path, nil, body)
	if err != nil {
		return err
	}

	if result != nil {
		if err := json.Unmarshal(resp.Data, result); err != nil {
			return fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	return nil
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, params url.Values, result interface{}) error {
	var body io.Reader
	if params != nil {
		body = strings.NewReader(params.Encode())
	}

	resp, err := c.doRequest(ctx, "PUT", path, nil, body)
	if err != nil {
		return err
	}

	if result != nil {
		if err := json.Unmarshal(resp.Data, result); err != nil {
			return fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	return nil
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, params url.Values) error {
	resp, err := c.doRequest(ctx, "DELETE", path, params, nil)
	if err != nil {
		return err
	}
	// Check if response indicates success (some DELETE operations return data)
	if resp != nil && len(resp.Data) > 0 {
		// Some DELETE operations return task IDs or other data
		// This is fine, just return nil
	}
	return nil
}

// WaitForTask waits for a Proxmox task to complete.
// Task ID can be in format "UPID:node:..." or just the task ID.
// Returns true if task completed successfully (OK or WARNINGS), false otherwise.
func (c *Client) WaitForTask(ctx context.Context, node, taskID string, timeout time.Duration) bool {
	// Extract task ID from UPID format if needed
	// UPID format: UPID:node:timestamp:pid:type:user:description
	actualTaskID := taskID
	if strings.HasPrefix(taskID, "UPID:") {
		// For UPID format, we need to use the full UPID for status check
		actualTaskID = taskID
	}

	startTime := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Track consecutive errors to avoid infinite loops on persistent errors
	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if time.Since(startTime) >= timeout {
				return false
			}

			var task TaskStatus
			statusPath := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, actualTaskID)
			if err := c.Get(ctx, statusPath, &task); err != nil {
				// Task might not exist yet, continue waiting
				// But track consecutive errors to avoid infinite loops
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					// Too many consecutive errors, likely a persistent issue
					return false
				}
				continue
			}

			// Reset error counter on successful status check
			consecutiveErrors = 0

			// Task is complete
			if task.Status == "stopped" {
				// WARNINGS are acceptable (e.g., VM start warnings about missing disk image)
				return task.ExitStatus == "OK" || task.ExitStatus == "WARNINGS"
			}

			// Task is running, continue waiting
			if task.Status == "running" {
				continue
			}

			// Unknown status, continue waiting
		}
	}
}

// WaitForTaskWithError waits for a Proxmox task to complete and returns a detailed error if it fails.
// This is a convenience wrapper around WaitForTask that provides better error messages.
func (c *Client) WaitForTaskWithError(ctx context.Context, node, taskID string, timeout time.Duration) error {
	if !c.WaitForTask(ctx, node, taskID, timeout) {
		var task TaskStatus
		if err := c.Get(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", node, taskID), &task); err == nil {
			return fmt.Errorf("task failed: status=%s, exitstatus=%s", task.Status, task.ExitStatus)
		}
		return fmt.Errorf("task failed or timed out")
	}
	return nil
}

// UploadFile uploads a file to Proxmox storage.
func (c *Client) UploadFile(ctx context.Context, node, storage, filename string, data io.Reader) (string, error) {
	if !c.authenticated {
		return "", fmt.Errorf("not authenticated")
	}

	uploadURL := fmt.Sprintf("%s/nodes/%s/storage/%s/upload", c.baseURL, node, storage)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add content parameter
	if err := writer.WriteField("content", "iso"); err != nil {
		return "", fmt.Errorf("failed to write field: %w", err)
	}

	// Add file
	part, err := writer.CreateFormFile("filename", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, data); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Add auth headers
	if c.ticket != "" {
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.ticket))
		req.Header.Set("CSRFPreventionToken", c.csrfToken)
	} else {
		// API token auth
		if tr, ok := c.httpClient.Transport.(*authTransport); ok {
			req.Header.Set("Authorization", tr.value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var proxmoxResp ProxmoxResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxmoxResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	var taskID string
	if err := json.Unmarshal(proxmoxResp.Data, &taskID); err != nil {
		return "", fmt.Errorf("failed to unmarshal task ID: %w", err)
	}

	return taskID, nil
}

// CheckISOExists checks if an ISO file already exists in storage.
func (c *Client) CheckISOExists(ctx context.Context, node, storage, filename string) (bool, error) {
	if !c.authenticated {
		return false, fmt.Errorf("not authenticated")
	}

	var contents []StorageContent
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	if err := c.Get(ctx, path, &contents); err != nil {
		// If storage doesn't support content listing, assume it doesn't exist
		return false, nil
	}

	expectedVolID := fmt.Sprintf("%s:iso/%s", storage, filename)
	for _, content := range contents {
		if content.VolID == expectedVolID {
			return true, nil
		}
	}

	return false, nil
}

// GetISOSize gets the size of an ISO file in storage.
// Returns the size in bytes and an error if the file doesn't exist or can't be accessed.
func (c *Client) GetISOSize(ctx context.Context, node, storage, filename string) (uint64, error) {
	if !c.authenticated {
		return 0, fmt.Errorf("not authenticated")
	}

	var contents []StorageContent
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	if err := c.Get(ctx, path, &contents); err != nil {
		return 0, fmt.Errorf("failed to list storage content: %w", err)
	}

	expectedVolID := fmt.Sprintf("%s:iso/%s", storage, filename)
	for _, content := range contents {
		if content.VolID == expectedVolID {
			return content.Size, nil
		}
	}

	return 0, fmt.Errorf("ISO file %s not found in storage %s", filename, storage)
}

