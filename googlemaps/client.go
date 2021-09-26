package googlemaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/markdrayton/sls/geo"
	"golang.org/x/sync/errgroup"
)

const (
	geocodeUrl = "https://maps.googleapis.com/maps/api/geocode/json?latlng=%f,%f&key=%s"
	// The geocoding API has a 50 QPS limit in addition to quotas. The average
	// response time is 100ms so a parallelism of 3 should stay under the cap.
	numWorkers = 3
)

type Client struct {
	APIKey string
	hc     *http.Client
}

func NewClient(APIKey string) *Client {
	return &Client{APIKey, &http.Client{}}
}

type GoogleGeocodeResponse struct {
	Results []GoogleGeocodeResult `json:"results"`
	Status  string                `json:"status"`
}

type GoogleGeocodeResult struct {
	AddressComponents []GoogleAddressComponent `json:"address_components"`
}

type GoogleAddressComponent struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

type GeocodeResult struct {
	LatLng  geo.LatLng            `json:"latlng"`
	Results []GoogleGeocodeResult `json:"results"`
}

func (c *Client) GeocodePoints(points []geo.LatLng) ([]GeocodeResult, error) {
	results := make([]GeocodeResult, 0)
	if c.APIKey == "" { // not configured
		return results, nil
	}

	group, _ := errgroup.WithContext(context.Background())

	pointsCh := make(chan geo.LatLng)
	go func() {
		for _, point := range points {
			pointsCh <- point
		}
		close(pointsCh)
	}()

	n := numWorkers
	if len(points) < numWorkers {
		n = len(points)
	}

	resultsCh := make(chan GeocodeResult)
	for i := 0; i < n; i++ {
		group.Go(func() error {
			for point := range pointsCh {
				url := fmt.Sprintf(geocodeUrl, point.Lat(), point.Lng(), c.APIKey)
				log.Debug("fetching " + url)
				resp, err := c.hc.Get(url)
				if err != nil {
					return err
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					return err
				}

				var g GoogleGeocodeResponse
				err = json.Unmarshal(body, &g)
				if err != nil {
					return err
				}

				if g.Status != "OK" && g.Status != "ZERO_RESULTS" {
					return fmt.Errorf("got non-OK from Google Maps API: %s", g.Status)
				}
				// ZERO_RESULTS returns empty results array
				resultsCh <- GeocodeResult{point, g.Results}
			}
			return nil
		})
	}

	go func() {
		for result := range resultsCh {
			results = append(results, result)
		}
	}()

	err := group.Wait()
	close(resultsCh)
	return results, err
}
