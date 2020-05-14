package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"sort"

	"golang.org/x/sync/errgroup"

	"github.com/markdrayton/sls/strava"
	"github.com/spf13/viper"
)

const urlActivities = "https://www.strava.com/api/v3/athletes/%d/activities?page=%d&per_page=%d"
const urlGear = "https://www.strava.com/api/v3/gear/%s"

type Gear map[string]strava.SummaryGear

type sls struct {
	athleteId int64
	perPage   int
	pageHint  int
	sf        *strava.Fetcher
}

func (s *sls) fetchPage(page int) (strava.Activities, error) {
	activities := make(strava.Activities, 0, s.perPage)
	u, _ := url.Parse(fmt.Sprintf(urlActivities, s.athleteId, page, s.perPage))
	err := s.sf.Fetch(&activities, u)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page %d: %s", page, err)
	}
	return activities, nil
}

func (s *sls) fetchActivities() (strava.Activities, error) {
	pages := make([]strava.Activities, s.pageHint)
	complete := false
	var g errgroup.Group
	for i := 0; i < s.pageHint; i++ {
		i := i // https://git.io/JfGiM
		g.Go(func() error {
			// Pages are 1-indexed
			page, err := s.fetchPage(i + 1)
			if err == nil {
				pages[i] = page
				// A short page indicates the last page in the set
				if len(page) < s.perPage {
					complete = true
				}
			}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Fetch remaining pages serially
	for complete == false {
		// Pages are 1-indexed
		page, err := s.fetchPage(len(pages) + 1)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
		if len(page) < s.perPage {
			complete = true
		}
	}

	activities := make(strava.Activities, 0, len(pages)*s.pageHint)
	for _, page := range pages {
		activities = append(activities, page...)
	}

	sort.Sort(activities)
	return activities, nil
}

func (s *sls) fetchGear(id string) (strava.SummaryGear, error) {
	u, _ := url.Parse(fmt.Sprintf(urlGear, id))

	var g strava.SummaryGear
	err := s.sf.Fetch(&g, u)
	if err != nil {
		return g, err
	}
	return g, nil
}

func (s *sls) fetchGears(activities strava.Activities) (Gear, error) {
	gearIds := activities.GearIds()
	gear := make([]strava.SummaryGear, len(gearIds))
	var g errgroup.Group
	for i, gearId := range gearIds {
		i, gearId := i, gearId // https://git.io/JfGiM
		g.Go(func() error {
			g, err := s.fetchGear(gearId)
			if err == nil {
				gear[i] = g
			}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	gears := make(Gear)
	for _, g := range gear {
		gears[g.Id] = g
	}

	return gears, nil
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to determine home directory")
	}
	confDir := path.Join(homeDir, ".sls")
	viper.SetConfigFile(path.Join(confDir, "config.toml"))
	viper.SetDefault("per_page", 100)
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config: %s", err)
	}

	s := sls{
		viper.GetInt64("athlete_id"),
		viper.GetInt("per_page"),
		viper.GetInt("page_hint"),
		strava.NewFetcher(
			path.Join(confDir, "token"),
			viper.GetString("client_id"),
			viper.GetString("client_secret"),
		),
	}

	activities, err := s.fetchActivities()
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}
	gears, err := s.fetchGears(activities)
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	fmt.Printf(
		"%10s  %10s  %-13s  %5s  %5s  %-26s  %-s\n",
		"#     Date",
		"ID",
		"Type",
		"Dist",
		"Elev",
		"Gear",
		"Name",
	)

	for _, a := range activities {
		gearName := ""
		if val, ok := gears[a.GearId]; ok {
			gearName = val.Name
		}
		fmt.Printf(
			"%10s  %10d  %-13s  %5.1f  %5.0f  %-26s  %-s\n",
			a.StartDateLocal[:10],
			a.Id,
			a.Type,
			a.Distance/1000,
			a.TotalElevationGain,
			gearName,
			a.Name,
		)
	}
}
