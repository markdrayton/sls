package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"sort"

	"golang.org/x/sync/errgroup"

	"github.com/markdrayton/sls/strava"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const perPage = 100

type GearMap map[string]strava.Gear

type sls struct {
	athleteId    int64
	activityHint int
	c            *strava.Client
}

func (s *sls) fetchActivities() (strava.Activities, error) {
	pages := make([]strava.Activities, (s.activityHint/perPage)+1)
	complete := false
	var g errgroup.Group
	for i := 0; i < len(pages); i++ {
		i := i // https://git.io/JfGiM
		g.Go(func() error {
			// Pages are 1-indexed
			page, err := s.c.ActivityPage(s.athleteId, i+1, perPage)
			if err == nil {
				pages[i] = page
				// A short page indicates the last page in the set
				if len(page) < perPage {
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
		page, err := s.c.ActivityPage(s.athleteId, len(pages)+1, perPage)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
		if len(page) < perPage {
			complete = true
		}
	}

	activities := make(strava.Activities, 0, len(pages)*perPage)
	for _, page := range pages {
		activities = append(activities, page...)
	}

	sort.Sort(activities)
	return activities, nil
}

func (s *sls) fetchGears(activities strava.Activities) (GearMap, error) {
	gearIds := activities.GearIds()
	gear := make([]strava.Gear, len(gearIds))
	var g errgroup.Group
	for i, gearId := range gearIds {
		i, gearId := i, gearId // https://git.io/JfGiM
		g.Go(func() error {
			g, err := s.c.Gear(gearId)
			if err == nil {
				gear[i] = g
			}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	gears := make(GearMap)
	for _, g := range gear {
		gears[g.Id] = g
	}

	return gears, nil
}

func main() {
	var power *bool = flag.BoolP("power", "p", false, "power-related columns")
	flag.Parse()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to determine home directory")
	}
	confDir := path.Join(homeDir, ".sls")
	viper.SetConfigFile(path.Join(confDir, "config.toml"))
	viper.SetDefault("activity_hint", 100)
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config: %s", err)
	}

	s := sls{
		viper.GetInt64("athlete_id"),
		viper.GetInt("activity_hint"),
		strava.NewClient(
			viper.GetInt("client_id"),
			viper.GetString("client_secret"),
			path.Join(confDir, "token"),
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

	if *power {
		fmt.Printf(
			"%10s  %10s  %-13s  %5s  %5s  %4s  %3s  %-26s  %-s\n",
			"#     Date",
			"ID",
			"Type",
			"Dist",
			"Elev",
			"Work",
			"Pwr",
			"Gear",
			"Name",
		)
	} else {
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
	}

	for _, a := range activities {
		gearName := "-"
		if val, ok := gears[a.GearId]; ok {
			gearName = val.Name
		}
		if *power {
			fmt.Printf(
				"%10s  %10d  %-13s  %5.1f  %5.0f  %s  %s  %-26s  %-s\n",
				a.StartDateLocal[:10],
				a.Id,
				a.Type,
				a.Distance/1000,
				a.TotalElevationGain,
				a.FmtWork(4),
				a.FmtAveragePower(3),
				gearName,
				a.Name,
			)
		} else {
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
}
