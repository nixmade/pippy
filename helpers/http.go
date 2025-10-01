package helpers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func HttpPost(url, post string) (string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(post)))
	if err != nil {
		return "", err
	}
	req.Close = true
	respU, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := respU.Body.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	if respU.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get returned %d, expected success with 200, error: %s", respU.StatusCode, respU.Status)
	}

	body, err := io.ReadAll(respU.Body)
	if err != nil {
		return "", err
	}
	return string(body), err
}

func HttpGet(url string, headers map[string]string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	req.Close = true
	respU, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := respU.Body.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	if respU.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get returned %d, expected success with 200, error: %s", respU.StatusCode, respU.Status)
	}

	body, err := io.ReadAll(respU.Body)
	if err != nil {
		return "", err
	}
	return string(body), err
}
