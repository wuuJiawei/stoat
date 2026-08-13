package updatecheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultURL       = "https://github.com/wuuJiawei/stoat/releases/latest/download/latest.txt"
	maxResponseBytes = 128
)

type Checker struct {
	URL    string
	Client *http.Client
}

func New(timeout time.Duration) Checker {
	return Checker{
		URL: DefaultURL,
		Client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(request *http.Request, previous []*http.Request) error {
				if request.URL.Scheme != "https" {
					return errors.New("update redirect must use HTTPS")
				}
				if len(previous) >= 5 {
					return errors.New("too many update redirects")
				}
				return nil
			},
		},
	}
}

func (c Checker) Latest(ctx context.Context, current string) (string, error) {
	currentVersion, err := parseStableVersion(current)
	if err != nil {
		return "", fmt.Errorf("parse current version: %w", err)
	}
	if c.URL == "" || c.Client == nil {
		return "", errors.New("update checker is not configured")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create update request: %w", err)
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", "stoat/"+current)

	response, err := c.Client.Do(request)
	if err != nil {
		return "", fmt.Errorf("check latest version: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("check latest version: HTTP %d", response.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("read latest version: %w", err)
	}
	if len(data) > maxResponseBytes {
		return "", errors.New("latest version response exceeds limit")
	}
	latest := strings.TrimSpace(string(data))
	latestVersion, err := parseStableVersion(latest)
	if err != nil {
		return "", fmt.Errorf("parse latest version: %w", err)
	}
	if compare(latestVersion, currentVersion) <= 0 {
		return "", nil
	}
	return canonical(latestVersion), nil
}

type stableVersion [3]int

func parseStableVersion(value string) (stableVersion, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	if value == "" || strings.ContainsAny(value, "-+") {
		return stableVersion{}, errors.New("expected a stable semantic version")
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return stableVersion{}, errors.New("expected major.minor.patch")
	}
	var parsed stableVersion
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return stableVersion{}, errors.New("invalid numeric version component")
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return stableVersion{}, errors.New("invalid numeric version component")
		}
		parsed[index] = number
	}
	return parsed, nil
}

func compare(left, right stableVersion) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func canonical(version stableVersion) string {
	return fmt.Sprintf("v%d.%d.%d", version[0], version[1], version[2])
}
