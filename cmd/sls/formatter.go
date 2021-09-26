package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type alignment int

const (
	alignLeft alignment = iota
	alignRight
)

const alwaysShow bool = true

type columnOpts struct {
	power bool
	time  bool
	start bool
	all   bool
}

type column struct {
	header string
	align  alignment
	show   bool
	format func(af *ActivityFormatter, ca CompositeActivity) string
}

type ActivityFormatter struct {
	columns []column
}

func NewActivityFormatter(opts columnOpts) *ActivityFormatter {
	return &ActivityFormatter{
		columns: []column{
			{"#     Date", alignRight, alwaysShow, formatDate},
			{"ID", alignRight, alwaysShow, formatId},
			{"Type", alignRight, alwaysShow, formatType},
			{"ExID", alignRight, opts.all, formatExternalId},
			{"Dist", alignRight, alwaysShow, formatDistance},
			{"Elev", alignRight, alwaysShow, formatElevation},
			{"Work", alignRight, opts.all || opts.power, formatWork},
			{"AP", alignRight, opts.all || opts.power, formatAveragePower},
			{"Time", alignRight, opts.all || opts.time, formatTime},
			{"Start", alignRight, opts.all || opts.start, formatStartLocation},
			{"Gear", alignLeft, alwaysShow, formatGear},
			{"Name", alignLeft, alwaysShow, formatName},
		},
	}
}

func (af *ActivityFormatter) headers() []string {
	headers := make([]string, 0, len(af.columns))
	for _, col := range af.columns {
		headers = append(headers, col.header)
	}
	return headers
}

func (af *ActivityFormatter) formatActivity(ca CompositeActivity) []string {
	cols := make([]string, 0, len(af.columns))
	for _, col := range af.columns {
		cols = append(cols, col.format(af, ca))
	}
	return cols
}

func (af *ActivityFormatter) formatColumns(cols []string, widths []int) string {
	vals := make([]string, 0, len(cols))
	for i, col := range af.columns {
		if col.show {
			pattern := "%*s" // alignRight
			if col.align == alignLeft {
				pattern = "%-*s"
			}
			vals = append(vals, fmt.Sprintf(pattern, widths[i], cols[i]))
		}
	}
	return strings.Join(vals, "  ")
}

func (af *ActivityFormatter) columnWidths(lines [][]string) []int {
	widths := make([]int, len(af.columns))
	for _, line := range lines {
		for i, col := range line {
			width := utf8.RuneCountInString(col)
			if width > widths[i] {
				widths[i] = width
			}
		}
	}
	return widths
}

func (af *ActivityFormatter) Format(activities []CompositeActivity) []string {
	lines := make([][]string, 0, len(activities)+1)
	lines = append(lines, af.headers())
	for _, da := range activities {
		lines = append(lines, af.formatActivity(da))
	}

	output := make([]string, 0, len(lines))
	widths := af.columnWidths(lines)
	for _, cols := range lines {
		output = append(output, af.formatColumns(cols, widths))
	}

	return output
}

func formatDate(af *ActivityFormatter, ca CompositeActivity) string {
	return ca.A.StartDateLocal[:10]
}

func formatId(af *ActivityFormatter, ca CompositeActivity) string {
	return strconv.FormatInt(ca.A.Id, 10)
}

func formatType(af *ActivityFormatter, ca CompositeActivity) string {
	return ca.A.Type
}

func formatExternalId(af *ActivityFormatter, ca CompositeActivity) string {
	if len(ca.A.ExternalId) > 0 {
		return ca.A.ExternalId
	} else {
		return "-"
	}
}

func formatDistance(af *ActivityFormatter, ca CompositeActivity) string {
	return fmt.Sprintf("%4.1f", ca.A.Distance/1000)
}

func formatElevation(af *ActivityFormatter, ca CompositeActivity) string {
	return fmt.Sprintf("%4.0f", ca.A.TotalElevationGain)
}

func formatWork(af *ActivityFormatter, ca CompositeActivity) string {
	if ca.A.DeviceWatts {
		return fmt.Sprintf("%4.0f", ca.A.Kilojoules)
	} else {
		return "-"
	}
}

func formatAveragePower(af *ActivityFormatter, ca CompositeActivity) string {
	if ca.A.DeviceWatts {
		return fmt.Sprintf("%4.0f", ca.A.AverageWatts)
	} else {
		return "-"
	}
}

func formatTime(af *ActivityFormatter, ca CompositeActivity) string {
	t := ca.A.MovingTime
	h := t / 3600
	t = t - (h * 3600)
	m := t / 60
	s := t - (m * 60)
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func formatStartLocation(af *ActivityFormatter, ca CompositeActivity) string {
	if ca.A.StartLatLng.IsZero() || ca.A.Type == "VirtualRide" || len(ca.SL.Results) == 0 {
		return "-"
	}

	component := func(match string) string {
		for _, ac := range ca.SL.Results[0].AddressComponents {
			for _, typ := range ac.Types {
				// Returns first match so only suitable for tags that are
				// applied to a single component.
				if typ == match {
					return ac.ShortName
				}
			}
		}
		return ""
	}

	country := component("country")
	prefs := [][]string{
		{"locality", "country"},
		{"postal_town", "country"},
	}

	switch country {
	case "BR":
		prefs = [][]string{
			{"administrative_area_level_3", "administrative_area_level_1", "country"},
			{"administrative_area_level_4", "administrative_area_level_1", "country"},
		}
	case "IT":
		prefs = [][]string{{"administrative_area_level_3", "country"}}
	case "US":
		prefs = [][]string{{"locality", "administrative_area_level_1", "country"}}
	}

	for _, pref := range prefs {
		parts := make([]string, 0)
		for _, tag := range pref {
			match := component(tag)
			if match != "" {
				parts = append(parts, match)
			}
		}
		if len(parts) == len(pref) {
			return strings.Join(parts, ", ")
		}
	}
	return "?"
}

func formatGear(af *ActivityFormatter, ca CompositeActivity) string {
	if ca.A.GearId != "" {
		return ca.G.Name
	} else {
		return "-"
	}
}

func formatName(af *ActivityFormatter, ca CompositeActivity) string {
	return ca.A.Name
}
