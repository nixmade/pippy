package users

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/nixmade/pippy/helpers"

	"github.com/nixmade/pippy/store"

	"github.com/urfave/cli/v3"
)

var ClientID string = "46ca5443da5014f4f00f"

const (
	Settings  = "settings:pippy"
	Scope     = "repo workflow user:email read:org read:user"
	GrantType = "urn:ietf:params:oauth:grant-type:device_code"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "user management",
		Commands: []*cli.Command{
			{
				Name:  "login",
				Usage: "login user",
				Action: func(ctx context.Context, c *cli.Command) error {
					if _, err := LoginUser(); err != nil {
						fmt.Printf("%v\n", err)
						return err
					}
					return nil
				},
			},
		},
	}
}

// userStore caches accesstoken and apikey in ~/.pippy/db
type UserStore struct {
	AccessToken           string     `json:"access_token"`
	ExpiresIn             int64      `json:"expires_in"`
	RefreshToken          string     `json:"refresh_token"`
	RefreshTokenExpiresIn int64      `json:"refresh_token_expires_in"`
	RefreshTime           int64      `json:"refresh_time"`
	GithubUser            githubUser `json:"user"`
}

func CacheTokens(cacheStore *UserStore) error {
	dbStore, err := store.Get(context.Background())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	if cacheStore.GithubUser.Login == "" {
		user, err := GithubUser(cacheStore.AccessToken)
		if err != nil {
			return err
		}
		cacheStore.GithubUser = *user
	}

	return dbStore.SaveJSON(Settings, cacheStore)
}

func GetCachedTokens() (*UserStore, error) {
	dbStore, err := store.Get(context.Background())
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := store.Close(dbStore); closeErr != nil {
			err = closeErr
		}
	}()

	cacheStore := &UserStore{}
	if err := dbStore.LoadJSON(Settings, cacheStore); err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return cacheStore, nil
}

func GetCachedAccessToken() (string, error) {
	cachedStore, err := LoginUser()
	if err != nil {
		return "", err
	}

	return cachedStore.AccessToken, nil
}

// LoginUser logs user into pippy and generates api key for usage
// Apikey is typically valid for predetermined time, key is automatically refreshed by client
// if signup is true, signs up user automatically
func LoginUser() (*UserStore, error) {
	cachedStore, err := GetCachedTokens()
	if err != nil {
		return nil, err
	}
	if cachedStore != nil && cachedStore.AccessToken != "" {
		if cachedStore.ExpiresIn > 0 {
			currentTime := time.Now().UTC().Unix()
			if currentTime > (cachedStore.RefreshTime + cachedStore.ExpiresIn) {
				userStore, err := RefreshAccessToken(ClientID, "", cachedStore.RefreshToken)
				if err != nil {
					return nil, err
				}
				return userStore, CacheTokens(userStore)
			}
		}
		return cachedStore, nil
	}

	resp, err := helpers.HttpPost("https://github.com/login/device/code", fmt.Sprintf("client_id=%s&scope=%s", ClientID, Scope))
	if err != nil {
		return nil, err
	}

	user_store, err := getAccessToken(string(resp))
	if err != nil {
		return nil, err
	}

	if err := CacheTokens(user_store); err != nil {
		return nil, err
	}

	return user_store, nil
}

func getAccessToken(resp string) (*UserStore, error) {
	values, err := url.ParseQuery(resp)
	if err != nil {
		return nil, err
	}

	verification_url, err := url.QueryUnescape(values.Get("verification_uri"))
	if err != nil {
		return nil, err
	}

	expires, err := strconv.ParseInt(values.Get("expires_in"), 10, 32)
	if err != nil {
		return nil, err
	}

	interval, err := strconv.ParseInt(values.Get("interval"), 10, 32)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Please enter user verification code %s at %s\n", values.Get("user_code"), verification_url)

	openbrowser(verification_url)

	params := url.Values{}
	params.Set("client_id", ClientID)
	params.Set("device_code", values.Get("device_code"))
	params.Set("grant_type", GrantType)

	start := time.Now().UTC()
	for int64(time.Since(start).Seconds()) < expires {
		resp, err := helpers.HttpPost("https://github.com/login/oauth/access_token", params.Encode())
		if err != nil {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		token, err := url.ParseQuery(resp)
		if err != nil {
			return nil, err
		}

		if token.Get("access_token") == "" {
			error_code := token.Get("error")
			if strings.EqualFold(error_code, "slow_down") {
				interval, err = strconv.ParseInt(token.Get("interval"), 10, 32)
				if err != nil {
					return nil, err
				}
			} else if !strings.EqualFold(error_code, "authorization_pending") {
				fmt.Printf("terminal error: %s\n", token.Get("error_description"))
				break
			}

			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		access_token := token.Get("access_token")
		refresh_token := token.Get("refresh_token")
		expires_in_token := token.Get("expires_in")
		var expires_in int64 = 0

		if expires_in_token != "" {
			expires_in, err = strconv.ParseInt(expires_in_token, 10, 64)
			if err != nil {
				return nil, err
			}
		}

		refresh_token_expires_in_token := token.Get("refresh_token_expires_in")
		var refresh_token_expires_in int64 = 0

		if refresh_token_expires_in_token != "" {
			refresh_token_expires_in, err = strconv.ParseInt(refresh_token_expires_in_token, 10, 64)
			if err != nil {
				return nil, err
			}
		}

		return &UserStore{
			AccessToken:           access_token,
			RefreshToken:          refresh_token,
			ExpiresIn:             expires_in,
			RefreshTokenExpiresIn: refresh_token_expires_in,
			RefreshTime:           time.Now().UTC().Unix(),
		}, nil
	}

	return nil, fmt.Errorf("failed to get access token, user did not authorize within %d seconds", expires)
}

func RefreshAccessToken(clientID, clientSecret, refreshToken string) (*UserStore, error) {
	params := url.Values{}
	params.Set("client_id", clientID)
	if clientSecret != "" {
		params.Set("client_secret", clientSecret)
	}
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")

	resp, err := helpers.HttpPost("https://github.com/login/oauth/access_token", params.Encode())
	if err != nil {
		return nil, err
	}
	token, err := url.ParseQuery(resp)
	if err != nil {
		return nil, err
	}

	refreshError := token.Get("error")
	if refreshError != "" {
		refreshErrorDescription := token.Get("error_description")
		return nil, fmt.Errorf("failed to refresh token %s: %s", refreshError, refreshErrorDescription)
	}

	access_token := token.Get("access_token")
	refresh_token := token.Get("refresh_token")
	expires_in, err := strconv.ParseInt(token.Get("expires_in"), 10, 64)
	if err != nil {
		return nil, err
	}

	refresh_token_expires_in, err := strconv.ParseInt(token.Get("refresh_token_expires_in"), 10, 64)
	if err != nil {
		return nil, err
	}

	userStore := &UserStore{
		AccessToken:           access_token,
		RefreshToken:          refresh_token,
		ExpiresIn:             expires_in,
		RefreshTokenExpiresIn: refresh_token_expires_in,
		RefreshTime:           time.Now().UTC().Unix(),
	}
	return userStore, nil
}

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}
