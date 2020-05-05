package strava

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type credentials struct {
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
}

func unmarshalCredentials(data []byte) (c credentials, err error) {
	err = json.Unmarshal(data, &c)
	if err != nil {
		return c, fmt.Errorf("Couldn't unmarshal token data: %s", err)
	}
	return c, err
}

func (c credentials) isExpired() bool {
	return time.Now().Add(time.Minute*time.Duration(5)).Unix() > c.ExpiresAt
}

func readCredentials(credsFile string) (c credentials, err error) {
	data, err := ioutil.ReadFile(credsFile)
	if err != nil {
		return c, fmt.Errorf("Couldn't read bootstrap token data from %s", credsFile)
	}
	c, err = unmarshalCredentials(data)
	if err != nil {
		return c, fmt.Errorf("Couldn't unmarshal token data: %s", err)
	}
	return c, err
}

func updateCredentials(refreshToken, clientId, clientSecret, credsFile string) (c credentials, err error) {
	form := url.Values{
		"client_id":     []string{clientId},
		"client_secret": []string{clientSecret},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{refreshToken},
	}
	resp, err := http.PostForm("https://www.strava.com/api/v3/oauth/token", form)
	if err != nil {
		return c, fmt.Errorf("Couldn't refresh token: %s", err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return c, fmt.Errorf("Couldn't read token data stream: %s", err)
	}
	err = ioutil.WriteFile(credsFile, data, 0644)
	if err != nil {
		return c, fmt.Errorf("Couldn't write token data: %s", err)
	}
	c, err = unmarshalCredentials(data)
	if err != nil {
		return c, err
	}
	return c, err
}

type Fetcher struct {
	client       *http.Client
	credsFile    string
	clientId     string
	clientSecret string
	accessToken  string
	mu           *sync.Mutex
}

func NewFetcher(credsFile, clientId, clientSecret string) *Fetcher {
	return &Fetcher{
		client:       &http.Client{},
		credsFile:    credsFile,
		clientId:     clientId,
		clientSecret: clientSecret,
		mu:           &sync.Mutex{},
	}
}

func (s *Fetcher) getToken() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken == "" {
		c, err := readCredentials(s.credsFile)
		if err != nil {
			return err
		}
		if c.isExpired() {
			c, err = updateCredentials(c.RefreshToken, s.clientId, s.clientSecret, s.credsFile)
			if err != nil {
				return err
			}
		}
		s.accessToken = c.AccessToken
	}
	return nil
}

func (s *Fetcher) fetchUrl(u *url.URL) ([]byte, error) {
	err := s.getToken()
	if err != nil {
		return nil, err
	}
	req := &http.Request{
		Method: "GET",
		Header: map[string][]string{
			"Authorization": {"Bearer " + s.accessToken},
		},
		URL: u,
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func isFault(data []byte) bool {
	var f apiFault
	err := json.Unmarshal(data, &f)
	if err != nil {
		// Failed to unmarshal into a Strava error: assume this is
		// valid data. This test isn't conclusive as almost any hash
		// will unmarshal into {[]}.
		return false
	}
	// Messages present? Assume error.
	if len(f.Message) > 0 {
		return true
	}
	return false
}

func (s *Fetcher) Fetch(v interface{}, u *url.URL) error {
	data, err := s.fetchUrl(u)
	if err != nil {
		return err
	}
	if isFault(data) {
		return fmt.Errorf("Strava API error: %s", string(data))
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

type apiError struct {
	Code     string `json:"code"`
	Field    string `json:"field"`
	Resource string `json:"resource"`
}

type apiFault struct {
	Message string     `json:"message"`
	Errors  []apiError `json:"errors"`
}
