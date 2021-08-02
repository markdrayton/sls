package strava

import (
	"time"
)

// DetailedGear (https://bit.ly/2zD10Wv)
type Gear struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

// SummaryActivity (https://bit.ly/3bzRVuE)
type Activity struct {
	Id                 int64     `json:"id"`
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	Distance           float64   `json:"distance"`
	TotalElevationGain float64   `json:"total_elevation_gain"`
	StartDateLocal     string    `json:"start_date_local"`
	StartDate          time.Time `json:"start_date"`
	MovingTime         int       `json:"moving_time"`
	GearId             string    `json:"gear_id"`
	Kilojoules         float64   `json:"kilojoules"`
	AverageWatts       float64   `json:"average_watts"`
	DeviceWatts        bool      `json:"device_watts"`
	ExternalId         string    `json:"external_id"`
}

type Activities []Activity

func (a Activities) GearIds() []string {
	gearMap := make(map[string]bool)
	for _, activity := range a {
		if len(activity.GearId) > 0 {
			gearMap[activity.GearId] = true
		}
	}
	gearIds := make([]string, 0, len(gearMap))
	for id := range gearMap {
		gearIds = append(gearIds, id)
	}
	return gearIds
}

func (a Activities) Len() int           { return len(a) }
func (a Activities) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Activities) Less(i, j int) bool { return a[i].StartDate.Before(a[j].StartDate) }
