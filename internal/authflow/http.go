package authflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// bodyLimit caps what facile will read from a server it does not control.
const bodyLimit = 1 << 20

var client = &http.Client{Timeout: 20 * time.Second}

type response struct {
	status int
	body   []byte
	header http.Header
}

func get(url string, headers map[string]string) (response, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return response{}, err
	}
	return send(request, headers)
}

func post(url string, payload any) (response, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return response{}, err
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return response{}, err
	}
	return send(request, map[string]string{"Content-Type": "application/json"})
}

func send(request *http.Request, headers map[string]string) (response, error) {
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	raw, err := client.Do(request)
	if err != nil {
		return response{}, fmt.Errorf("cannot reach %s — check the URL and your network", request.URL.Host)
	}
	defer raw.Body.Close()

	body, err := io.ReadAll(io.LimitReader(raw.Body, bodyLimit))
	if err != nil {
		return response{}, fmt.Errorf("cannot read the answer from %s — try again", request.URL.Host)
	}
	return response{status: raw.StatusCode, body: body, header: raw.Header}, nil
}

// cookie scrapes a Set-Cookie header, which is how antenne returns its session
// instead of a token in the body.
func (r response) cookie(name string) string {
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == name && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func (r response) ok() bool { return r.status >= 200 && r.status < 300 }

// decode pulls one string field out of a JSON object without binding facile to
// the whole shape, which differs per tool and changes without warning.
func (r response) decode() map[string]any {
	var doc map[string]any
	if err := json.Unmarshal(r.body, &doc); err != nil {
		return nil
	}
	return doc
}

func stringField(doc map[string]any, key string) string {
	if value, ok := doc[key].(string); ok {
		return value
	}
	return ""
}

func boolField(doc map[string]any, key string) bool {
	value, _ := doc[key].(bool)
	return value
}
