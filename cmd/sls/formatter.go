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
}

type column struct {
	header string
	align  alignment
	show   bool
	format func(af *ActivityFormatter, da DetailedActivity) string
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
			{"Dist", alignRight, alwaysShow, formatDistance},
			{"Elev", alignRight, alwaysShow, formatElevation},
			{"Work", alignRight, opts.power, formatWork},
			{"AP", alignRight, opts.power, formatAveragePower},
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

func (af *ActivityFormatter) formatActivity(da DetailedActivity) []string {
	cols := make([]string, 0, len(af.columns))
	for _, col := range af.columns {
		cols = append(cols, col.format(af, da))
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

func (af *ActivityFormatter) Format(activities []DetailedActivity) []string {
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

func formatDate(af *ActivityFormatter, da DetailedActivity) string {
	return da.A.StartDateLocal[:10]
}

func formatId(af *ActivityFormatter, da DetailedActivity) string {
	return strconv.FormatInt(da.A.Id, 10)
}

func formatType(af *ActivityFormatter, da DetailedActivity) string {
	return da.A.Type
}

func formatDistance(af *ActivityFormatter, da DetailedActivity) string {
	return fmt.Sprintf("%4.1f", da.A.Distance/1000)
}

func formatElevation(af *ActivityFormatter, da DetailedActivity) string {
	return fmt.Sprintf("%4.0f", da.A.TotalElevationGain)
}

func formatWork(af *ActivityFormatter, da DetailedActivity) string {
	if da.A.DeviceWatts {
		return fmt.Sprintf("%4.0f", da.A.Kilojoules)
	} else {
		return "-"
	}
}

func formatAveragePower(af *ActivityFormatter, da DetailedActivity) string {
	if da.A.DeviceWatts {
		return fmt.Sprintf("%4.0f", da.A.AverageWatts)
	} else {
		return "-"
	}
}

func formatGear(af *ActivityFormatter, da DetailedActivity) string {
	return da.G.Name
}

func formatName(af *ActivityFormatter, da DetailedActivity) string {
	return da.A.Name
}
