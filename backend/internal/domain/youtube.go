package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func ParseYouTubeID(input string) (string, error) {
	value := strings.TrimSpace(input)
	if youtubeIDPattern.MatchString(value) {
		return value, nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid YouTube URL")
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))

	var candidate string
	switch host {
	case "youtu.be":
		candidate = firstPathPart(parsed.Path)
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		parts := pathParts(parsed.Path)
		if parsed.Path == "/watch" {
			candidate = parsed.Query().Get("v")
		} else if len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
			candidate = parts[1]
		}
	default:
		return "", errors.New("unsupported YouTube host")
	}
	if !youtubeIDPattern.MatchString(candidate) {
		return "", errors.New("invalid YouTube video ID")
	}
	return candidate, nil
}

func CanonicalYouTubeURL(id string) string {
	return "https://www.youtube.com/watch?v=" + id
}

func firstPathPart(path string) string {
	parts := pathParts(path)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func pathParts(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		if item != "" {
			parts = append(parts, item)
		}
	}
	return parts
}
