package users

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "context value " + k.name
}

var (
	NameCtx  = &contextKey{"UserName"}
	EmailCtx = &contextKey{"UserEmail"}
)

type githubUser struct {
	Login     string `json:"login"`
	Id        uint64 `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatar_url"`
}

type githubEmail struct {
	Email   string `json:"email"`
	Primary bool   `json:"primary"`
}

type HttpError struct {
	Message string `json:"message"`
}

// tokenFromHeader tries to retreive the token string from the
// "Authorization" request header: "Authorization: Token T".
// func tokenFromHeader(r *http.Request) string {
// 	// Get token from authorization header.
// 	bearer := r.Header.Get("Authorization")
// 	if len(bearer) > 6 && strings.ToUpper(bearer[0:5]) == "TOKEN" {
// 		return bearer[6:]
// 	}
// 	return ""
// }

func defaultTransport() http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
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

func GithubUser(accessToken string) (*githubUser, error) {
	var user githubUser
	err := GetJSON("https://api.github.com/user", fmt.Sprintf("token %s", accessToken), &user)
	if err != nil {
		return nil, err
	}

	if user.Email == "" {
		email, err := GithubPrimaryEmail(accessToken)
		if err != nil {
			return nil, err
		}
		user.Email = email
	}

	return &user, nil
}

func GithubPrimaryEmail(accessToken string) (string, error) {
	var emails []*githubEmail
	err := GetJSON("https://api.github.com/user/emails", fmt.Sprintf("token %s", accessToken), &emails)
	if err != nil {
		return "", err
	}

	for _, res := range emails {
		if res.Primary {
			return res.Email, nil
		}
	}

	return "", nil
}
