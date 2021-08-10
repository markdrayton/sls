package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/markdrayton/sls/strava"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const numWorkers = 20

type GearMap map[string]strava.Gear

type DetailedActivity struct {
	A strava.Activity `json:"activity"`
	G strava.Gear     `json:"gear"`
}

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

	workers := int32(numWorkers)
	if epoch.Unix() > 0 {
		workers = 1
	}

	pageNums := make(chan int)
	pages := make(chan strava.Activities)

	// Producer
	done := make(chan struct{}, workers) // Buffer a done signal from each worker.
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
	for i := 0; i < int(workers); i++ {
		g.Go(func() error {
			defer func() {
				if atomic.AddInt32(&workers, -1) == 0 {
					close(pages)
				}
			}()

			for pageNum := range pageNums {
				page, err := s.c.ActivityPage(s.athleteId, epoch, pageNum, perPage)
				if err != nil {
					return err
				}
				select {
				case pages <- page:
				case <-ctx.Done():
					return ctx.Err()
				}
				if len(page) < perPage {
					done <- struct{}{}
					return nil // No more work to do.
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
		done <- struct{}{}
		sort.Sort(activities)
		return nil
	})

	return activities, g.Wait()
}

func (s *sls) activities() (strava.Activities, error) {
	var cached strava.Activities
	if !s.refreshCache {
		cached = s.readActivityCache()
	}
	epoch := time.Unix(0, 0)
	if len(cached) > 0 {
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
		// Rather than copying the cached values into `gm` unconditonally check to see
		// if any activities still refer to a cached value, thus automatically pruning
		// the cache of dangling entries.
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

func init() {
	pflag.BoolP("all", "a", false, "show all columns")
	pflag.BoolP("power", "p", false, "show power-related columns")
	pflag.BoolP("refresh", "r", false, "refresh cache")
	pflag.BoolP("json", "j", false, "JSON output")
	pflag.Parse()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to determine home directory")
	}
	slsDir := path.Join(homeDir, ".sls")
	viper.SetConfigFile(path.Join(slsDir, "config.toml"))
	viper.SetDefault("activity_cache", path.Join(slsDir, "activities.json"))
	viper.SetDefault("gear_cache", path.Join(slsDir, "gear.json"))
	viper.SetDefault("token_path", path.Join(slsDir, "token"))

	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config: %s", err)
	}

	viper.BindPFlags(pflag.CommandLine)
}

func main() {
	s := sls{
		viper.GetInt64("athlete_id"),
		viper.GetString("activity_cache"),
		viper.GetString("gear_cache"),
		viper.GetBool("refresh"),
		strava.NewClient(
			viper.GetInt("client_id"),
			viper.GetString("client_secret"),
			viper.GetString("token_path"),
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

	detailedActivities := make([]DetailedActivity, 0, len(activities))
	for _, a := range activities {
		var gear strava.Gear
		if _, ok := gears[a.GearId]; ok {
			gear = gears[a.GearId]
		} else {
			gear = strava.Gear{Name: "-"}
		}
		detailedActivities = append(detailedActivities, DetailedActivity{a, gear})
	}

	if viper.GetBool("json") {
		j, err := json.Marshal(detailedActivities)
		if err != nil {
			log.Panicf("Coudln't marshal to JSON: %s", err)
		}
		fmt.Print(string(j))
	} else {
		opts := columnOpts{
			power: viper.GetBool("power"),
			time:  viper.GetBool("time"),
			all:   viper.GetBool("all"),
		}
		formatter := NewActivityFormatter(opts)
		for _, line := range formatter.Format(detailedActivities) {
			fmt.Println(line)
		}
	}

	s.writeActivityCache(activities)
	s.writeGearCache(gears)
}
