package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type User struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"profile_image_url"`
}

type Stream struct {
	ID           string    `json:"id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	GameName     string    `json:"game_name"`
	Title        string    `json:"title"`
	ViewerCount  int       `json:"viewer_count"`
	StartedAt    time.Time `json:"started_at"`
	ThumbnailURL string    `json:"thumbnail_url"`
}

type Client struct {
	clientID     string
	clientSecret string
	redirectURL  string
	http         *http.Client
}

func New(clientID, clientSecret, redirectURL string) *Client {
	return &Client{
		clientID: clientID, clientSecret: clientSecret, redirectURL: redirectURL,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c.clientID != "" && c.clientSecret != "" && c.redirectURL != ""
}

func (c *Client) AuthorizationURL(state string) string {
	values := url.Values{
		"response_type": {"code"},
		"client_id":     {c.clientID},
		"redirect_uri":  {c.redirectURL},
		"scope":         {""},
		"state":         {state},
	}
	return "https://id.twitch.tv/oauth2/authorize?" + values.Encode()
}

func (c *Client) Exchange(ctx context.Context, code string) (User, error) {
	values := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.redirectURL},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("twitch token exchange: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("twitch token exchange returned %s", response.Status)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return User{}, err
	}
	if token.AccessToken == "" {
		return User{}, errors.New("twitch returned an empty access token")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Client-Id", c.clientID)
	response, err = c.http.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("twitch users request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("twitch users returned %s", response.Status)
	}
	var users struct {
		Data []User `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&users); err != nil {
		return User{}, err
	}
	if len(users.Data) != 1 {
		return User{}, errors.New("twitch user not found")
	}
	return users.Data[0], nil
}

func (c *Client) StreamByLogin(ctx context.Context, login string) (User, *Stream, error) {
	if !c.Configured() {
		return User{}, nil, errors.New("twitch is not configured")
	}
	values := url.Values{"client_id": {c.clientID}, "client_secret": {c.clientSecret}, "grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return User{}, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return User{}, nil, fmt.Errorf("twitch app token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return User{}, nil, fmt.Errorf("twitch app token returned %s", response.Status)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return User{}, nil, err
	}

	get := func(endpoint string, target any) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		req.Header.Set("Client-Id", c.clientID)
		response, err := c.http.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("twitch helix returned %s", response.Status)
		}
		return json.NewDecoder(response.Body).Decode(target)
	}
	var users struct {
		Data []User `json:"data"`
	}
	if err := get("https://api.twitch.tv/helix/users?login="+url.QueryEscape(login), &users); err != nil {
		return User{}, nil, err
	}
	if len(users.Data) != 1 {
		return User{}, nil, errors.New("twitch streamer not found")
	}
	var streams struct {
		Data []Stream `json:"data"`
	}
	if err := get("https://api.twitch.tv/helix/streams?user_login="+url.QueryEscape(login), &streams); err != nil {
		return User{}, nil, err
	}
	if len(streams.Data) == 0 {
		return users.Data[0], nil, nil
	}
	return users.Data[0], &streams.Data[0], nil
}
