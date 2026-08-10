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

type Clip struct {
	ID              string    `json:"id"`
	URL             string    `json:"url"`
	EmbedURL        string    `json:"embed_url"`
	BroadcasterName string    `json:"broadcaster_name"`
	CreatorName     string    `json:"creator_name"`
	VideoID         string    `json:"video_id"`
	Title           string    `json:"title"`
	ViewCount       int       `json:"view_count"`
	CreatedAt       time.Time `json:"created_at"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	Duration        float64   `json:"duration"`
	VODOffset       *int      `json:"vod_offset"`
}

type Video struct {
	ID           string    `json:"id"`
	StreamID     string    `json:"stream_id"`
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
	PublishedAt  time.Time `json:"published_at"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	ViewCount    int       `json:"view_count"`
	Language     string    `json:"language"`
	Type         string    `json:"type"`
	Duration     string    `json:"duration"`
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
	return c.AuthorizationURLWithScopes(state, nil)
}

func (c *Client) AuthorizationURLWithScopes(state string, scopes []string) string {
	values := url.Values{
		"response_type": {"code"},
		"client_id":     {c.clientID},
		"redirect_uri":  {c.redirectURL},
		"scope":         {strings.Join(scopes, " ")},
		"state":         {state},
	}
	return "https://id.twitch.tv/oauth2/authorize?" + values.Encode()
}

type UserToken struct {
	User         User
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func (c *Client) Exchange(ctx context.Context, code string) (User, error) {
	result, err := c.ExchangeUserToken(ctx, code)
	return result.User, err
}

func (c *Client) ExchangeUserToken(ctx context.Context, code string) (UserToken, error) {
	values := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.redirectURL},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return UserToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return UserToken{}, fmt.Errorf("twitch token exchange: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return UserToken{}, fmt.Errorf("twitch token exchange returned %s", response.Status)
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return UserToken{}, err
	}
	if token.AccessToken == "" {
		return UserToken{}, errors.New("twitch returned an empty access token")
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return UserToken{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Client-Id", c.clientID)
	response, err = c.http.Do(req)
	if err != nil {
		return UserToken{}, fmt.Errorf("twitch users request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return UserToken{}, fmt.Errorf("twitch users returned %s", response.Status)
	}
	var users struct {
		Data []User `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&users); err != nil {
		return UserToken{}, err
	}
	if len(users.Data) != 1 {
		return UserToken{}, errors.New("twitch user not found")
	}
	return UserToken{User: users.Data[0], AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, ExpiresIn: token.ExpiresIn}, nil
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

func (c *Client) ClipsByLogin(ctx context.Context, login string, startedAt, endedAt *time.Time) ([]Clip, error) {
	if !c.Configured() {
		return nil, errors.New("twitch is not configured")
	}
	values := url.Values{"client_id": {c.clientID}, "client_secret": {c.clientSecret}, "grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitch app token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch app token returned %s", response.Status)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return nil, err
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
		return nil, err
	}
	if len(users.Data) != 1 {
		return nil, errors.New("twitch streamer not found")
	}
	type clipWindow struct{ start, end *time.Time }
	windows := []clipWindow{{start: startedAt, end: endedAt}}
	if startedAt == nil && endedAt == nil {
		start := time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Now().UTC()
		startedAt, endedAt = &start, &end
	}
	if startedAt != nil && endedAt != nil && endedAt.Sub(*startedAt) > 370*24*time.Hour {
		windows = nil
		for cursor := startedAt.UTC(); cursor.Before(*endedAt); {
			next := cursor.AddDate(1, 0, 0)
			if next.After(*endedAt) {
				next = endedAt.UTC()
			}
			windowStart, windowEnd := cursor, next
			windows = append(windows, clipWindow{start: &windowStart, end: &windowEnd})
			cursor = next
		}
	}

	result := make([]Clip, 0, 500)
	seen := make(map[string]struct{})
	for _, window := range windows {
		query := url.Values{"broadcaster_id": {users.Data[0].ID}, "first": {"100"}}
		if window.start != nil {
			query.Set("started_at", window.start.UTC().Format(time.RFC3339))
		}
		if window.end != nil {
			query.Set("ended_at", window.end.UTC().Format(time.RFC3339))
		}
		for page := 0; page < 10; page++ {
			var clips struct {
				Data       []Clip `json:"data"`
				Pagination struct {
					Cursor string `json:"cursor"`
				} `json:"pagination"`
			}
			if err := get("https://api.twitch.tv/helix/clips?"+query.Encode(), &clips); err != nil {
				return nil, err
			}
			for _, clip := range clips.Data {
				if _, exists := seen[clip.ID]; exists {
					continue
				}
				seen[clip.ID] = struct{}{}
				result = append(result, clip)
			}
			if clips.Pagination.Cursor == "" || len(clips.Data) == 0 {
				break
			}
			query.Set("after", clips.Pagination.Cursor)
		}
	}
	return result, nil
}

func (c *Client) VideosByLogin(ctx context.Context, login string) ([]Video, error) {
	get, err := c.appHelix(ctx)
	if err != nil {
		return nil, err
	}
	var users struct {
		Data []User `json:"data"`
	}
	if err := get("https://api.twitch.tv/helix/users?login="+url.QueryEscape(login), &users); err != nil {
		return nil, err
	}
	if len(users.Data) != 1 {
		return nil, errors.New("twitch streamer not found")
	}
	query := url.Values{"user_id": {users.Data[0].ID}, "first": {"100"}, "type": {"archive"}}
	result := make([]Video, 0, 100)
	for page := 0; page < 5; page++ {
		var response struct {
			Data       []Video `json:"data"`
			Pagination struct {
				Cursor string `json:"cursor"`
			} `json:"pagination"`
		}
		if err := get("https://api.twitch.tv/helix/videos?"+query.Encode(), &response); err != nil {
			return nil, err
		}
		result = append(result, response.Data...)
		if response.Pagination.Cursor == "" || len(response.Data) == 0 {
			break
		}
		query.Set("after", response.Pagination.Cursor)
	}
	return result, nil
}

func (c *Client) VideoByID(ctx context.Context, id string) (Video, error) {
	get, err := c.appHelix(ctx)
	if err != nil {
		return Video{}, err
	}
	var response struct {
		Data []Video `json:"data"`
	}
	if err := get("https://api.twitch.tv/helix/videos?id="+url.QueryEscape(id), &response); err != nil {
		return Video{}, err
	}
	if len(response.Data) != 1 {
		return Video{}, errors.New("twitch video not found")
	}
	return response.Data[0], nil
}

func (c *Client) appHelix(ctx context.Context) (func(string, any) error, error) {
	if !c.Configured() {
		return nil, errors.New("twitch is not configured")
	}
	values := url.Values{"client_id": {c.clientID}, "client_secret": {c.clientSecret}, "grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitch app token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch app token returned %s", response.Status)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return nil, err
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
	return get, nil
}
