package strava

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const urlToken = "https://www.strava.com/api/v3/oauth/token"

type Credentials struct {
	clientId     int
	clientSecret string
	tokenPath    string
	hc           *http.Client
	mutex        *sync.Mutex
	token        token
}

func NewCredentials(clientId int, clientSecret, tokenPath string, hc *http.Client) *Credentials {
	return &Credentials{
		clientId:     clientId,
		clientSecret: clientSecret,
		tokenPath:    tokenPath,
		hc:           hc,
		mutex:        &sync.Mutex{},
	}
}

func (c *Credentials) MustGetAccessToken() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if len(c.token.AccessToken) != 0 {
		return c.token.AccessToken
	}
	err := c.getToken()
	if err != nil {
		log.Fatal(err)
	}
	return c.token.AccessToken
}

func (c *Credentials) getToken() error {
	t, err := c.readToken()
	if err != nil {
		return err
	}
	if t.isExpired() {
		data, err := c.postRefresh(t.RefreshToken)
		if err != nil {
			return err
		}
		if isFault(data) {
			return fmt.Errorf("got an error response when updating token: %s", string(data))
		}
		t, err = unmarshalToken(data)
		if err != nil {
			return err
		}
		err = c.writeToken(data)
		if err != nil {
			return fmt.Errorf("couldn't write token data: %s", err)
		}
	}
	c.token = t
	return nil
}

func (c *Credentials) readToken() (t token, err error) {
	data, err := ioutil.ReadFile(c.tokenPath)
	if err != nil {
		return t, fmt.Errorf("couldn't read token data from %s", c.tokenPath)
	}
	t, err = unmarshalToken(data)
	if err != nil {
		return t, err
	}
	return t, err
}

func (c *Credentials) writeToken(data []byte) error {
	return ioutil.WriteFile(c.tokenPath, data, 0644)
}

func (c *Credentials) postRefresh(refreshToken string) ([]byte, error) {
	form := url.Values{
		"client_id":     []string{strconv.Itoa(c.clientId)},
		"client_secret": []string{c.clientSecret},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{refreshToken},
	}
	resp, err := c.hc.PostForm(urlToken, form)
	if err != nil {
		return nil, fmt.Errorf("couldn't refresh token: %s", err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read token data stream: %s", err)
	}
	return data, nil
}

type token struct {
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
}

func (t token) isExpired() bool {
	return time.Now().Add(time.Minute*time.Duration(5)).Unix() > t.ExpiresAt
}

func unmarshalToken(data []byte) (t token, err error) {
	err = json.Unmarshal(data, &t)
	if err != nil {
		return t, fmt.Errorf("couldn't unmarshal token data: %s", err)
	}
	return t, err
}
