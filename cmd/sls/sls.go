package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/markdrayton/sls/geo"
	"github.com/markdrayton/sls/googlemaps"
	"github.com/markdrayton/sls/strava"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type GearMap map[string]strava.Gear

type LocationMap map[geo.LatLng]googlemaps.GeocodeResult

type CompositeActivity struct {
	A  strava.Activity          `json:"activity"`
	G  strava.Gear              `json:"gear"`
	SL googlemaps.GeocodeResult `json:"start_location"`
}

type sls struct {
	athleteId     int64
	activityCache string
	gearCache     string
	locationCache string
	refreshCache  bool
	sc            *strava.Client
	gc            *googlemaps.Client
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

	new, err := s.sc.Activities(s.athleteId, epoch)
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

	gears, err := s.sc.Gears(missing)
	if err != nil {
		return gm, err
	}

	for _, gear := range gears {
		gm[gear.Id] = gear
	}

	return gm, nil
}

func roundedStartLocations(activities strava.Activities) []geo.LatLng {
	rounded := make(map[geo.LatLng]struct{})
	for _, a := range activities {
		rounded[geo.RoundLatLng(a.StartLatLng)] = struct{}{}
	}

	points := make([]geo.LatLng, 0)
	for latLng := range rounded {
		points = append(points, latLng)
	}
	return points
}

func (s *sls) startLocations(activities strava.Activities) (LocationMap, error) {
	lm := make(LocationMap)
	if !s.refreshCache {
		lm = s.readLocationCache()
	}

	missing := make([]geo.LatLng, 0)
	// round start locations down to 2km boundaries to reduce the number of API calls
	for _, point := range roundedStartLocations(activities) {
		_, ok := lm[point]
		if !ok {
			missing = append(missing, point)
		}
	}

	locations, err := s.gc.GeocodePoints(missing)
	if err != nil {
		return lm, err
	}

	for _, location := range locations {
		lm[location.LatLng] = location
	}

	return lm, nil
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

func (s *sls) readLocationCache() LocationMap {
	var locations []googlemaps.GeocodeResult
	readCache(s.locationCache, &locations)

	lm := make(LocationMap)
	for _, location := range locations {
		lm[location.LatLng] = location
	}
	return lm
}

func (s *sls) writeLocationCache(lm LocationMap) {
	var locations []googlemaps.GeocodeResult
	for _, location := range lm {
		locations = append(locations, location)
	}
	writeCache(s.locationCache, locations)
}

func init() {
	pflag.BoolP("all", "a", false, "show all columns")
	pflag.BoolP("power", "p", false, "show power-related columns")
	pflag.BoolP("start", "s", false, "show start location")
	pflag.BoolP("time", "t", false, "show activity duration")
	pflag.BoolP("json", "j", false, "JSON output")
	pflag.BoolP("refresh", "r", false, "fully refresh cache")
	pflag.BoolP("debug", "d", false, "debug logging")
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
	viper.SetDefault("location_cache", path.Join(slsDir, "locations.json"))
	viper.SetDefault("token_path", path.Join(slsDir, "token"))

	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Couldn't read config: %s", err)
	}

	viper.BindPFlags(pflag.CommandLine)

	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	s := sls{
		athleteId:     viper.GetInt64("athlete_id"),
		activityCache: viper.GetString("activity_cache"),
		gearCache:     viper.GetString("gear_cache"),
		locationCache: viper.GetString("location_cache"),
		refreshCache:  viper.GetBool("refresh"),
		sc: strava.NewClient(
			viper.GetInt("client_id"),
			viper.GetString("client_secret"),
			viper.GetString("token_path"),
		),
		gc: googlemaps.NewClient(
			viper.GetString("google_maps_api_key"),
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

	locations, err := s.startLocations(activities)
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	compositeActivities := make([]CompositeActivity, 0, len(activities))
	for _, a := range activities {
		var gear strava.Gear
		if _, ok := gears[a.GearId]; ok {
			gear = gears[a.GearId]
		}
		var location googlemaps.GeocodeResult
		if l, ok := locations[geo.RoundLatLng(a.StartLatLng)]; ok {
			location = l
		}
		compositeActivities = append(compositeActivities, CompositeActivity{a, gear, location})
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
			start: viper.GetBool("start"),
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
	s.writeLocationCache(locations)
}
