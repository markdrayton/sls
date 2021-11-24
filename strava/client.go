package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"

	"golang.org/x/sync/errgroup"
)

const urlActivities = "https://www.strava.com/api/v3/athletes/%d/activities?after=%d&page=%d&per_page=%d"
const urlGear = "https://www.strava.com/api/v3/gear/%s"

type Client struct {
	creds *Credentials
	hc    *http.Client
}

const (
	perPage    = 100
	numWorkers = 10
)

func NewClient(clientId int, clientSecret, tokenPath string) *Client {
	hc := &http.Client{}
	return &Client{
		NewCredentials(clientId, clientSecret, tokenPath, hc),
		hc,
	}
}

func (c *Client) Activities(athleteId int64, epoch time.Time) (Activities, error) {
	group, ctx := errgroup.WithContext(context.Background())
	activities := make(Activities, 0)

	urls := make(chan string)
	done := make(chan struct{}) // stop yielding page URLs
	group.Go(func() error {
		page := 1
		stop := false
		for !stop {
			url := fmt.Sprintf(urlActivities, athleteId, epoch.Unix(), page, perPage)
			select {
			case urls <- url:
			case <-done:
				stop = true
			}
			page++
		}
		close(urls)
		return nil
	})

	responses := make(chan []byte)
	go func() {
		for response := range responses {
			var page Activities
			err := unmarshal(response, &page)
			if err != nil {
				log.Fatalf("couldn't unmarshal gear: %s", err) // TODO avoid fatal?
			}
			if len(page) < perPage {
				// Reading a short page signifies the end of the activity set has
				// been reached, so tell the producer to stop yielding URLs. Use a
				// nonblocking send because the producer goroutine exits once told
				// to stop.
				select {
				case done <- struct{}{}:
				default:
				}
			}
			activities = append(activities, page...)
		}
	}()

	n := numWorkers
	if epoch.Unix() > 0 {
		// A non-zero epoch implies some cached data was found. Fetch remaining
		// pages serially.
		n = 1
	}

	for i := 0; i < n; i++ {
		group.Go(c.worker(ctx, urls, responses))
	}

	err := group.Wait()
	close(responses)
	return activities, err
}

func (c *Client) Gears(gearIds []string) ([]Gear, error) {
	group, ctx := errgroup.WithContext(context.Background())
	// TODO use ctx
	gears := make([]Gear, len(gearIds))

	urls := make(chan string)
	group.Go(func() error {
		for _, gearId := range gearIds {
			urls <- fmt.Sprintf(urlGear, gearId)
		}
		close(urls)
		return nil
	})

	responses := make(chan []byte)
	go func() {
		for response := range responses {
			var gear Gear
			err := unmarshal(response, &gear)
			if err != nil {
				log.Fatalf("couldn't unmarshal gear: %s", err)
			}
			gears = append(gears, gear)
		}
	}()

	n := numWorkers
	if len(gearIds) < numWorkers {
		n = len(gearIds)
	}

	for i := 0; i < n; i++ {
		group.Go(c.worker(ctx, urls, responses))
	}

	err := group.Wait()
	close(responses)
	return gears, err
}

func (c *Client) worker(ctx context.Context, urls chan string, responses chan []byte) func() error {
	return func() error {
		for rawurl := range urls {
			log.Debug("fetching " + rawurl)
			u, err := url.Parse(rawurl)
			if err != nil {
				return err
			}

			data, err := c.fetchUrl(u)
			if err != nil {
				return err
			}

			select {
			case responses <- data:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
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

func unmarshal(data []byte, v interface{}) error {
	if isFault(data) {
		return fmt.Errorf("client API error: %s", string(data))
	}
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}
