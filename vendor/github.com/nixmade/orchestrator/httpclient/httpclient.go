package httpclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type HttpError struct {
	Message string `json:"message"`
}

func defaultTransport() http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func GetJSON(url, token string, value interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	req.Close = true
	http.DefaultClient.Transport = defaultTransport()
	defer http.DefaultClient.CloseIdleConnections()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorMessage(url, resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(value); err != nil {
		return err
	}

	return err
}

func errorMessage(url string, resp *http.Response) error {
	var httpError HttpError
	if resp.Body != nil {
		if err := json.NewDecoder(resp.Body).Decode(&httpError); err != nil {
			return fmt.Errorf("%s returned %d, error: %s", url, resp.StatusCode, resp.Status)
		}
	}
	return fmt.Errorf("%s returned %d, details: %s", url, resp.StatusCode, httpError.Message)
}

func newRequest(verb, url string, in interface{}) (*http.Request, error) {
	if in == nil {
		return http.NewRequest(verb, url, nil)
	}
	post, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return http.NewRequest(verb, url, bytes.NewBuffer(post))

}

func PostJSON(url, token string, in interface{}, out interface{}) error {
	req, err := newRequest("POST", url, in)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	req.Close = true
	http.DefaultClient.Transport = defaultTransport()
	defer http.DefaultClient.CloseIdleConnections()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorMessage(url, resp)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}

	return err
}

func Delete(url, token string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	req.Close = true
	http.DefaultClient.Transport = defaultTransport()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer http.DefaultClient.CloseIdleConnections()

	if resp.StatusCode != http.StatusOK {
		return errorMessage(url, resp)
	}

	return nil
}
