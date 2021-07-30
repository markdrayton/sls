package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/markdrayton/sls/strava"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const numWorkers = 20

type GearMap map[string]strava.Gear

type sls struct {
	athleteId     int64
	activityCache string
	gearCache     string
	refreshCache  bool
	c             *strava.Client
}

func (s *sls) fetchActivities(epoch time.Time) (strava.Activities, error) {
	g, ctx := errgroup.WithContext(context.Background())
	const perPage = 100

	pageNums := make(chan int)
	pages := make(chan strava.Activities)

	// Producer
	done := make(chan bool)
	g.Go(func() error {
		pageNum := 1
		for {
			select {
			case pageNums <- pageNum:
				pageNum += 1
			case <-done:
				close(pageNums)
				return nil
			}
		}
	})

	// Workers
	workers := int32(numWorkers)
	if epoch.Unix() > 0 {
		workers = 1
	}

	for i := 0; i < int(workers); i++ {
		g.Go(func() error {
			defer func() {
				if atomic.AddInt32(&workers, -1) == 0 {
					close(pages)
					done <- true
				}
			}()

			for pageNum := range pageNums {
				page, err := s.c.ActivityPage(s.athleteId, epoch, pageNum, perPage)
				if err != nil {
					return err
				}
				if len(page) == 0 {
					break // Nothing more to read, worker can exit
				}
				select {
				case pages <- page:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	// Reducer
	activities := make(strava.Activities, 0, perPage*workers)
	g.Go(func() error {
		for page := range pages {
			activities = append(activities, page...)
		}
		sort.Sort(activities)
		return nil
	})

	return activities, g.Wait()
}

func (s *sls) activities() (strava.Activities, error) {
	cached := s.readActivityCache()
	epoch := time.Unix(0, 0)
	if !s.refreshCache && len(cached) > 0 {
		epoch = cached[len(cached)-1].StartDate
	}

	new, err := s.fetchActivities(epoch)
	if err != nil {
		return nil, err
	}

	return append(cached, new...), nil
}

func (s *sls) gears(gearIds []string) (GearMap, error) {
	cached := s.readGearCache()
	gm := make(GearMap)

	ch := make(chan strava.Gear)
	go func() {
		for gear := range ch {
			gm[gear.Id] = gear
		}
	}()

	var g errgroup.Group
	for _, gearId := range gearIds {
		gearId := gearId // https://git.io/JfGiM
		gear, ok := cached[gearId]
		if !s.refreshCache && ok {
			ch <- gear
		} else {
			g.Go(func() error {
				gear, err := s.c.Gear(gearId)
				if err != nil {
					return err
				}
				ch <- gear
				return nil
			})
		}
	}

	err := g.Wait()
	close(ch)

	return gm, err
}

func (s *sls) readActivityCache() strava.Activities {
	var activities strava.Activities
	readCache(s.activityCache, &activities)
	return activities
}

func (s *sls) writeActivityCache(activities strava.Activities) {
	if s.activityCache == "" {
		return
	}
	writeCache(s.activityCache, activities)
}

func (s *sls) readGearCache() GearMap {
	var gm GearMap
	readCache(s.gearCache, &gm)
	return gm
}

func (s *sls) writeGearCache(gm GearMap) {
	if s.gearCache == "" {
		return
	}
	writeCache(s.gearCache, gm)
}

func main() {
	var power *bool = flag.BoolP("power", "p", false, "power-related columns")
	var ignoreCache *bool = flag.BoolP("ignore-cache", "i", false, "ignore cache contents")
	flag.Parse()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to determine home directory")
	}
	slsDir := path.Join(homeDir, ".sls")
	viper.SetConfigFile(path.Join(slsDir, "config.toml"))
	viper.SetDefault("activity_cache", path.Join(slsDir, "activities.json"))
	viper.SetDefault("gear_cache", path.Join(slsDir, "gear.json"))
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config: %s", err)
	}

	s := sls{
		viper.GetInt64("athlete_id"),
		viper.GetString("activity_cache"),
		viper.GetString("gear_cache"),
		*ignoreCache,
		strava.NewClient(
			viper.GetInt("client_id"),
			viper.GetString("client_secret"),
			path.Join(slsDir, "token"),
		),
	}

	activities, err := s.activities()
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	gears, err := s.gears(activities.GearIds())
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

	s.writeActivityCache(activities)
	s.writeGearCache(gears)
}
