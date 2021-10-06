package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"time"

	"github.com/markdrayton/sls/strava"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type GearMap map[string]strava.Gear

type CompositeActivity struct {
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

func (s *sls) activities() (strava.Activities, error) {
	var cached strava.Activities
	if !s.refreshCache {
		cached = s.readActivityCache()
	}

	epoch := time.Unix(0, 0)
	if len(cached) > 0 {
		epoch = cached[len(cached)-1].StartDate
	}

	new, err := s.c.Activities(s.athleteId, epoch)
	if err != nil {
		return nil, err
	}

	all := append(cached, new...)
	sort.Sort(all)
	return all, nil
}

// Return a list of unique gear IDs
func gearIds(activities strava.Activities) []string {
	gearIDMap := make(map[string]struct{})
	for _, a := range activities {
		if a.GearId != "" {
			gearIDMap[a.GearId] = struct{}{}
		}
	}

	gearIds := make([]string, 0, len(gearIDMap))
	for id := range gearIDMap {
		gearIds = append(gearIds, id)
	}
	return gearIds
}

func (s *sls) gears(activities strava.Activities) (GearMap, error) {
	gm := make(GearMap)
	if !s.refreshCache {
		gm = s.readGearCache()
	}

	missing := make([]string, 0)
	for _, gearId := range gearIds(activities) {
		_, ok := gm[gearId]
		if !ok {
			missing = append(missing, gearId)
		}
	}

	gears, err := s.c.Gears(missing)
	if err != nil {
		return gm, err
	}

	for _, gear := range gears {
		gm[gear.Id] = gear
	}

	return gm, nil
}

func (s *sls) readActivityCache() strava.Activities {
	var activities strava.Activities
	readCache(s.activityCache, &activities)
	return activities
}

func (s *sls) writeActivityCache(activities strava.Activities) {
	writeCache(s.activityCache, activities)
}

func (s *sls) readGearCache() GearMap {
	gm := make(GearMap)
	readCache(s.gearCache, &gm)
	return gm
}

func (s *sls) writeGearCache(gm GearMap) {
	writeCache(s.gearCache, gm)
}

func init() {
	pflag.BoolP("all", "a", false, "show all columns")
	pflag.BoolP("power", "p", false, "show power-related columns")
	pflag.BoolP("time", "t", false, "show activity duration")
	pflag.BoolP("json", "j", false, "JSON output")
	pflag.BoolP("refresh", "r", false, "fully refresh cache")
	pflag.CommandLine.SortFlags = false
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
		athleteId:     viper.GetInt64("athlete_id"),
		activityCache: viper.GetString("activity_cache"),
		gearCache:     viper.GetString("gear_cache"),
		refreshCache:  viper.GetBool("refresh"),
		c: strava.NewClient(
			viper.GetInt("client_id"),
			viper.GetString("client_secret"),
			viper.GetString("token_path"),
		),
	}

	activities, err := s.activities()
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	gears, err := s.gears(activities)
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	compositeActivities := make([]CompositeActivity, 0, len(activities))
	for _, a := range activities {
		var gear strava.Gear
		if _, ok := gears[a.GearId]; ok {
			gear = gears[a.GearId]
		}
		compositeActivities = append(compositeActivities, CompositeActivity{a, gear})
	}

	if viper.GetBool("json") {
		j, err := json.Marshal(compositeActivities)
		if err != nil {
			log.Fatalf("Coudln't marshal to JSON: %s", err)
		}
		fmt.Print(string(j))
	} else {
		opts := columnOpts{
			power: viper.GetBool("power"),
			time:  viper.GetBool("time"),
			all:   viper.GetBool("all"),
		}
		formatter := NewActivityFormatter(opts)
		for _, line := range formatter.Format(compositeActivities) {
			fmt.Println(line)
		}
	}

	s.writeActivityCache(activities)
	s.writeGearCache(gears)
}
