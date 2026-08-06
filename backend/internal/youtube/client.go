package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Metadata struct {
	ID               string
	Title            string
	ChannelTitle     string
	ThumbnailURL     string
	DurationSeconds  int
	ViewCount        int64
	YouTubeLikeCount int64
}

type Client interface {
	Fetch(context.Context, string) (Metadata, error)
}

type GoogleClient struct {
	apiKey string
	http   *http.Client
}

func NewGoogleClient(apiKey string) *GoogleClient {
	return &GoogleClient{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *GoogleClient) Fetch(ctx context.Context, id string) (Metadata, error) {
	if c.apiKey == "" {
		fallback := Metadata{
			ID:           id,
			Title:        "YouTube video " + id,
			ChannelTitle: "YouTube",
			ThumbnailURL: "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg",
		}
		endpoint := "https://www.youtube.com/oembed?format=json&url=" +
			url.QueryEscape("https://www.youtube.com/watch?v="+id)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fallback, nil
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return fallback, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fallback, nil
		}
		var payload struct {
			Title        string `json:"title"`
			AuthorName   string `json:"author_name"`
			ThumbnailURL string `json:"thumbnail_url"`
		}
		if json.NewDecoder(resp.Body).Decode(&payload) == nil {
			if payload.Title != "" {
				fallback.Title = payload.Title
			}
			if payload.AuthorName != "" {
				fallback.ChannelTitle = payload.AuthorName
			}
			if payload.ThumbnailURL != "" {
				fallback.ThumbnailURL = payload.ThumbnailURL
			}
		}
		return fallback, nil
	}

	endpoint, _ := url.Parse("https://www.googleapis.com/youtube/v3/videos")
	query := endpoint.Query()
	query.Set("part", "snippet,contentDetails,statistics,status")
	query.Set("id", id)
	query.Set("key", c.apiKey)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Metadata{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("YouTube API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("YouTube API returned %s", resp.Status)
	}

	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
			Statistics struct {
				ViewCount string `json:"viewCount"`
				LikeCount string `json:"likeCount"`
			} `json:"statistics"`
			Status struct {
				Embeddable bool   `json:"embeddable"`
				Privacy    string `json:"privacyStatus"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Metadata{}, err
	}
	if len(payload.Items) != 1 {
		return Metadata{}, errors.New("video not found")
	}
	item := payload.Items[0]
	if item.Status.Privacy != "public" {
		return Metadata{}, errors.New("video is not public")
	}

	thumbnail := item.Snippet.Thumbnails["high"].URL
	if thumbnail == "" {
		thumbnail = item.Snippet.Thumbnails["medium"].URL
	}
	viewCount, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
	likeCount, _ := strconv.ParseInt(item.Statistics.LikeCount, 10, 64)
	return Metadata{
		ID:               item.ID,
		Title:            item.Snippet.Title,
		ChannelTitle:     item.Snippet.ChannelTitle,
		ThumbnailURL:     thumbnail,
		DurationSeconds:  parseISODuration(item.ContentDetails.Duration),
		ViewCount:        viewCount,
		YouTubeLikeCount: likeCount,
	}, nil
}

func parseISODuration(value string) int {
	value = strings.TrimPrefix(value, "PT")
	total := 0
	number := ""
	for _, r := range value {
		if r >= '0' && r <= '9' {
			number += string(r)
			continue
		}
		n, _ := strconv.Atoi(number)
		number = ""
		switch r {
		case 'H':
			total += n * 3600
		case 'M':
			total += n * 60
		case 'S':
			total += n
		}
	}
	return total
}
