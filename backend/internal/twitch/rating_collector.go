package twitch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const eventSubWebSocketURL = "wss://eventsub.wss.twitch.tv/ws?keepalive_timeout_seconds=30"

type RatingStream struct {
	ID, Channel, Title, GameName string
	StartedAt                    time.Time
	EndedAt                      *time.Time
	ObservedLive                 bool
}

type RatingMessage struct {
	ID, Channel, UserID, Login, DisplayName, AvatarURL, StreamID, Role, TextHash string
	SentAt                                                                       time.Time
	TextLength                                                                   int
	IsCommand, IsDuplicate, IsEmoteOnly, IsReply                                 bool
}

type RatingCredentials struct {
	AccessToken, RefreshToken, UserID, Login string
	ExpiresAt                                *time.Time
}

type RatingCollectorStore interface {
	RatingCredentials(context.Context) (RatingCredentials, error)
	SaveRatingCredentials(context.Context, RatingCredentials) error
	UpsertRatingStream(context.Context, RatingStream) error
	CloseRatingStreams(context.Context, string, string, time.Time) error
	InsertRatingMessage(context.Context, RatingMessage) error
	SetRatingCollectorStatus(context.Context, []string, string, string, string, *time.Time) error
}

type RatingCollector struct {
	clientID, clientSecret string
	initial                RatingCredentials
	channels               []string
	store                  RatingCollectorStore
	logger                 *slog.Logger
	http                   *http.Client

	mu           sync.Mutex
	credentials  RatingCredentials
	channelIDs   map[string]string
	current      map[string]*Stream
	recentHashes map[string]time.Time
}

func NewRatingCollector(clientID, clientSecret, accessToken, refreshToken string, store RatingCollectorStore, logger *slog.Logger) *RatingCollector {
	return &RatingCollector{
		clientID: clientID, clientSecret: clientSecret,
		initial:  RatingCredentials{AccessToken: strings.TrimSpace(accessToken), RefreshToken: strings.TrimSpace(refreshToken)},
		channels: []string{"ravshann", "ravshanbtw"}, store: store, logger: logger,
		http: &http.Client{Timeout: 15 * time.Second}, channelIDs: map[string]string{},
		current: map[string]*Stream{}, recentHashes: map[string]time.Time{},
	}
}

func (c *RatingCollector) Configured() bool {
	return c.clientID != "" && c.clientSecret != ""
}

func (c *RatingCollector) Run(ctx context.Context) {
	if !c.Configured() {
		_ = c.store.SetRatingCollectorStatus(ctx, c.channels, "not_configured", "", "", nil)
		return
	}
	for ctx.Err() == nil {
		if err := c.runSession(ctx, eventSubWebSocketURL, true); err != nil && ctx.Err() == nil {
			c.logger.Error("rating collector disconnected", "error", err)
			status := "error"
			if c.credentials.AccessToken == "" {
				status = "not_configured"
			}
			_ = c.store.SetRatingCollectorStatus(ctx, c.channels, status, c.credentials.Login, err.Error(), nil)
			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Second):
			}
		}
	}
}

func (c *RatingCollector) prepare(ctx context.Context) error {
	credentials, err := c.store.RatingCredentials(ctx)
	if err != nil || credentials.RefreshToken == "" {
		credentials = c.initial
	}
	validation, err := c.validate(ctx, credentials.AccessToken)
	if err != nil || validation.ExpiresIn < 300 {
		if credentials.RefreshToken == "" {
			return errors.New("Twitch chat token is expired; reconnect the collector")
		}
		credentials, err = c.refresh(ctx, credentials.RefreshToken)
		if err != nil {
			return err
		}
		validation, err = c.validate(ctx, credentials.AccessToken)
		if err != nil {
			return err
		}
	}
	if !contains(validation.Scopes, "user:read:chat") {
		return errors.New("Twitch chat token does not include user:read:chat")
	}
	credentials.UserID, credentials.Login = validation.UserID, validation.Login
	expires := time.Now().Add(time.Duration(validation.ExpiresIn) * time.Second)
	credentials.ExpiresAt = &expires
	if err := c.store.SaveRatingCredentials(ctx, credentials); err != nil {
		return err
	}
	c.mu.Lock()
	c.credentials = credentials
	c.mu.Unlock()
	for _, login := range c.channels {
		user, err := c.userByLogin(ctx, login)
		if err != nil {
			return err
		}
		c.channelIDs[login] = user.ID
		_, stream, err := c.streamByID(ctx, user.ID)
		if err != nil {
			return err
		}
		if stream != nil {
			c.current[login] = stream
			_ = c.saveStream(ctx, login, stream)
		}
	}
	return nil
}

func (c *RatingCollector) runSession(ctx context.Context, endpoint string, subscribe bool) error {
	if subscribe {
		if err := c.prepare(ctx); err != nil {
			return err
		}
	}
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	for {
		var envelope eventSubEnvelope
		if err := wsjson.Read(ctx, conn, &envelope); err != nil {
			return err
		}
		switch envelope.Metadata.MessageType {
		case "session_welcome":
			if subscribe {
				if err := c.subscribe(ctx, envelope.Payload.Session.ID); err != nil {
					return err
				}
			}
			_ = c.store.SetRatingCollectorStatus(ctx, c.channels, "connected", c.credentials.Login, "", timePointer(time.Now()))
		case "session_reconnect":
			return c.runSession(ctx, envelope.Payload.Session.ReconnectURL, false)
		case "notification":
			if err := c.handleEvent(ctx, envelope); err != nil {
				c.logger.Warn("rating event failed", "error", err)
			}
		case "revocation":
			return fmt.Errorf("Twitch revoked %s: %s", envelope.Metadata.SubscriptionType, envelope.Payload.Subscription.Status)
		}
	}
}

type tokenValidation struct {
	ClientID, Login, UserID string
	Scopes                  []string
	ExpiresIn               int
}

func (c *RatingCollector) validate(ctx context.Context, token string) (tokenValidation, error) {
	if token == "" {
		return tokenValidation{}, errors.New("empty Twitch chat token")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://id.twitch.tv/oauth2/validate", nil)
	req.Header.Set("Authorization", "OAuth "+token)
	response, err := c.http.Do(req)
	if err != nil {
		return tokenValidation{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return tokenValidation{}, fmt.Errorf("validate chat token: %s", response.Status)
	}
	var raw struct {
		ClientID  string   `json:"client_id"`
		Login     string   `json:"login"`
		UserID    string   `json:"user_id"`
		Scopes    []string `json:"scopes"`
		ExpiresIn int      `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return tokenValidation{}, err
	}
	return tokenValidation(raw), nil
}

func (c *RatingCollector) refresh(ctx context.Context, refreshToken string) (RatingCredentials, error) {
	values := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {c.clientID}, "client_secret": {c.clientSecret}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(req)
	if err != nil {
		return RatingCredentials{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return RatingCredentials{}, fmt.Errorf("refresh chat token: %s", response.Status)
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return RatingCredentials{}, err
	}
	expires := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return RatingCredentials{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, ExpiresAt: &expires}, nil
}

func (c *RatingCollector) helix(ctx context.Context, method, endpoint string, body any, target any) error {
	var reader *strings.Reader
	if body != nil {
		bytes, _ := json.Marshal(body)
		reader = strings.NewReader(string(bytes))
	} else {
		reader = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Client-Id", c.clientID)
	req.Header.Set("Authorization", "Bearer "+c.credentials.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Twitch Helix %s: %s", endpoint, response.Status)
	}
	if target != nil {
		return json.NewDecoder(response.Body).Decode(target)
	}
	return nil
}

func (c *RatingCollector) userByLogin(ctx context.Context, login string) (User, error) {
	var result struct {
		Data []User `json:"data"`
	}
	if err := c.helix(ctx, http.MethodGet, "https://api.twitch.tv/helix/users?login="+url.QueryEscape(login), nil, &result); err != nil {
		return User{}, err
	}
	if len(result.Data) != 1 {
		return User{}, fmt.Errorf("Twitch channel %s not found", login)
	}
	return result.Data[0], nil
}
func (c *RatingCollector) streamByID(ctx context.Context, id string) (User, *Stream, error) {
	var result struct {
		Data []Stream `json:"data"`
	}
	if err := c.helix(ctx, http.MethodGet, "https://api.twitch.tv/helix/streams?user_id="+url.QueryEscape(id), nil, &result); err != nil {
		return User{}, nil, err
	}
	if len(result.Data) == 0 {
		return User{}, nil, nil
	}
	return User{}, &result.Data[0], nil
}

func (c *RatingCollector) subscribe(ctx context.Context, sessionID string) error {
	transport := map[string]string{"method": "websocket", "session_id": sessionID}
	for _, login := range c.channels {
		id := c.channelIDs[login]
		subscriptions := []map[string]any{
			{"type": "channel.chat.message", "version": "1", "condition": map[string]string{"broadcaster_user_id": id, "user_id": c.credentials.UserID}, "transport": transport},
			{"type": "stream.online", "version": "1", "condition": map[string]string{"broadcaster_user_id": id}, "transport": transport},
			{"type": "stream.offline", "version": "1", "condition": map[string]string{"broadcaster_user_id": id}, "transport": transport},
		}
		for _, item := range subscriptions {
			if err := c.helix(ctx, http.MethodPost, "https://api.twitch.tv/helix/eventsub/subscriptions", item, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

type eventSubEnvelope struct {
	Metadata struct {
		MessageType      string    `json:"message_type"`
		SubscriptionType string    `json:"subscription_type"`
		MessageTimestamp time.Time `json:"message_timestamp"`
	} `json:"metadata"`
	Payload struct {
		Session struct {
			ID           string `json:"id"`
			ReconnectURL string `json:"reconnect_url"`
		} `json:"session"`
		Subscription struct {
			Status string `json:"status"`
		} `json:"subscription"`
		Event json.RawMessage `json:"event"`
	} `json:"payload"`
}

func (c *RatingCollector) handleEvent(ctx context.Context, envelope eventSubEnvelope) error {
	switch envelope.Metadata.SubscriptionType {
	case "stream.online":
		var event struct {
			ID            string    `json:"id"`
			BroadcasterID string    `json:"broadcaster_user_id"`
			StartedAt     time.Time `json:"started_at"`
		}
		if err := json.Unmarshal(envelope.Payload.Event, &event); err != nil {
			return err
		}
		login := c.loginByID(event.BroadcasterID)
		_, stream, err := c.streamByID(ctx, event.BroadcasterID)
		if err != nil {
			return err
		}
		if stream == nil {
			stream = &Stream{ID: event.ID, UserLogin: login, StartedAt: event.StartedAt}
		}
		c.current[login] = stream
		return c.saveStream(ctx, login, stream)
	case "stream.offline":
		var event struct {
			BroadcasterID string `json:"broadcaster_user_id"`
		}
		if err := json.Unmarshal(envelope.Payload.Event, &event); err != nil {
			return err
		}
		login := c.loginByID(event.BroadcasterID)
		streamID := ""
		if stream := c.current[login]; stream != nil {
			streamID = stream.ID
		}
		delete(c.current, login)
		return c.store.CloseRatingStreams(ctx, login, streamID, envelope.Metadata.MessageTimestamp)
	case "channel.chat.message":
		return c.saveMessage(ctx, envelope.Payload.Event, envelope.Metadata.MessageTimestamp)
	}
	return nil
}

func (c *RatingCollector) saveStream(ctx context.Context, login string, stream *Stream) error {
	return c.store.UpsertRatingStream(ctx, RatingStream{ID: stream.ID, Channel: login, Title: stream.Title, GameName: stream.GameName, StartedAt: stream.StartedAt, ObservedLive: true})
}
func (c *RatingCollector) loginByID(id string) string {
	for login, candidate := range c.channelIDs {
		if candidate == id {
			return login
		}
	}
	return ""
}

func (c *RatingCollector) saveMessage(ctx context.Context, raw json.RawMessage, sentAt time.Time) error {
	var wire struct {
		BroadcasterID       string `json:"broadcaster_user_id"`
		SourceBroadcasterID string `json:"source_broadcaster_user_id"`
		MessageID           string `json:"message_id"`
		ChatterID           string `json:"chatter_user_id"`
		ChatterLogin        string `json:"chatter_user_login"`
		ChatterName         string `json:"chatter_user_name"`
		MessageType         string `json:"message_type"`
		Message             struct {
			Text      string `json:"text"`
			Fragments []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"fragments"`
		} `json:"message"`
		Badges []struct {
			SetID string `json:"set_id"`
		} `json:"badges"`
		Reply json.RawMessage `json:"reply"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}
	login := c.loginByID(wire.BroadcasterID)
	if login == "" {
		return nil
	}
	if wire.SourceBroadcasterID != "" && wire.SourceBroadcasterID != wire.BroadcasterID {
		return nil
	}
	stream := c.current[login]
	if stream == nil {
		return nil
	}
	text := strings.TrimSpace(wire.Message.Text)
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	hash := sha256.Sum256([]byte(normalized))
	hashString := hex.EncodeToString(hash[:])
	key := wire.ChatterID + ":" + stream.ID + ":" + hashString
	previous := c.recentHashes[key]
	duplicate := !previous.IsZero() && sentAt.Sub(previous) < 10*time.Minute
	c.recentHashes[key] = sentAt
	if len(c.recentHashes) > 10000 {
		for key, at := range c.recentHashes {
			if sentAt.Sub(at) > time.Hour {
				delete(c.recentHashes, key)
			}
		}
	}
	emoteOnly := len(wire.Message.Fragments) > 0
	for _, fragment := range wire.Message.Fragments {
		if strings.TrimSpace(fragment.Text) != "" && fragment.Type != "emote" {
			emoteOnly = false
			break
		}
	}
	role := "viewer"
	for _, badge := range wire.Badges {
		switch badge.SetID {
		case "broadcaster":
			role = "broadcaster"
		case "moderator":
			if role != "broadcaster" {
				role = "moderator"
			}
		case "vip":
			if role == "viewer" {
				role = "vip"
			}
		}
	}
	message := RatingMessage{ID: wire.MessageID, Channel: login, UserID: wire.ChatterID, Login: strings.ToLower(wire.ChatterLogin), DisplayName: wire.ChatterName, StreamID: stream.ID, Role: role, TextHash: hashString, SentAt: sentAt, TextLength: len([]rune(text)), IsCommand: strings.HasPrefix(text, "!") || strings.HasPrefix(text, "/"), IsDuplicate: duplicate, IsEmoteOnly: emoteOnly, IsReply: len(wire.Reply) > 0 && string(wire.Reply) != "null"}
	if err := c.store.InsertRatingMessage(ctx, message); err != nil {
		return err
	}
	return c.store.SetRatingCollectorStatus(ctx, []string{login}, "connected", c.credentials.Login, "", &sentAt)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func timePointer(value time.Time) *time.Time { return &value }
