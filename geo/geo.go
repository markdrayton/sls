package geo

import (
	"math"
)

const (
	earthRadiusKm = 6378
	roundKm       = 2
)

type LatLng [2]float64

func (l *LatLng) Lat() float64 {
	return l[0]
}

func (l *LatLng) Lng() float64 {
	return l[1]
}

func (l *LatLng) IsZero() bool {
	return l.Lat() == 0 && l.Lng() == 0
}

func RoundLatLng(l LatLng) LatLng {
	// returns a LatLng with lat and lng rounded down to the next roundKm boundary
	flooredLat := round(l.Lat(), earthRadiusKm, roundKm)
	circumference := 2 * math.Pi * earthRadiusKm * math.Cos(flooredLat)
	flooredLng := round(l.Lng(), circumference, roundKm)
	return LatLng{flooredLat, flooredLng}
}

func deg2rad(degrees float64) float64 {
	return degrees * (math.Pi / 180)
}

func rad2deg(radians float64) float64 {
	return radians * (180 / math.Pi)
}

func round(degrees, circumference float64, round int64) float64 {
	arc := circumference * deg2rad(degrees)
	return rad2deg(float64((int64(arc)/round)*round) / circumference)
}
