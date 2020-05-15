package strava

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const urlActivities = "https://www.strava.com/api/v3/athletes/%d/activities?page=%d&per_page=%d"
const urlGear = "https://www.strava.com/api/v3/gear/%s"

type Client struct {
	creds *Credentials
	hc    *http.Client
}

func NewClient(clientId int, clientSecret, tokenPath string) *Client {
	hc := &http.Client{}
	return &Client{
		NewCredentials(clientId, clientSecret, tokenPath, hc),
		hc,
	}
}

func (c *Client) ActivityPage(athleteId int64, page, perPage int) (Activities, error) {
	activities := make(Activities, 0, perPage)
	u, _ := url.Parse(fmt.Sprintf(urlActivities, athleteId, page, perPage))
	err := c.fetch(&activities, u)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page %d: %s", page, err)
	}
	return activities, nil
}

func (c *Client) Gear(id string) (Gear, error) {
	var gear Gear
	u, _ := url.Parse(fmt.Sprintf(urlGear, id))
	err := c.fetch(&gear, u)
	if err != nil {
		return gear, err
	}
	return gear, nil
}

func (c *Client) fetchUrl(u *url.URL) ([]byte, error) {
	req := &http.Request{
		Method: "GET",
		Header: map[string][]string{
			"Authorization": {"Bearer " + c.creds.MustGetAccessToken()},
		},
		URL: u,
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (c *Client) fetch(v interface{}, u *url.URL) error {
	data, err := c.fetchUrl(u)
	if err != nil {
		return err
	}
	if isFault(data) {
		return fmt.Errorf("Client API error: %s", string(data))
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}
