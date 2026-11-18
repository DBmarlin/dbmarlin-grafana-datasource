package plugin

// The colours the DBmarlin UI gives its top wait events and top statements
// (CHART_COLOURS_WAITS, CHART_COLOURS_STATEMENTS and OTHER_COLOUR_CODE in
// www/src/component/Constants.ts). The UI colours by rank, biggest first, not
// by name, so the same event can have another colour in another time range.
var (
	waitColours      = []string{"#9e0142", "#d53e4f", "#f46d43", "#fdae61", "#fee08b", "#e6f598", "#abdda4", "#66c2a5", "#3288bd", "#5e4fa2"}
	statementColours = []string{"#6610f2", "#c29ffa", "#20c997", "#a6e9d5", "#0dcaf0", "#9eeaf9", "#fd7e14", "#fecba1", "#d63384", "#efadce"}
	otherColour      = "#DE1173"

	// The first ten of TARGET_COLOUR_CODES, which the instances screen gives its
	// top instances by total time. Beyond ten the UI shows no colour.
	instanceColours = []string{"#2d4bff", "#2da2ff", "#2ee5ff", "#1effb1", "#05ff00", "#c6ff03", "#fcff03", "#fcde00", "#fcb101", "#fc5a02"}
)

// seriesColour is the UI's colour for the series at rank (0 is the biggest) of
// a dimension, or "" to leave the colour to Grafana's palette.
func seriesColour(dimension string, rank int, other bool) string {
	if other {
		return otherColour
	}
	var palette []string
	switch dimension {
	case "waitevent":
		palette = waitColours
	case "sqlhash":
		palette = statementColours
	}
	if rank < 0 || rank >= len(palette) {
		return ""
	}
	return palette[rank]
}

// instanceColour is the UI's colour for the instance at rank (0 is the one with
// the most time), or "" to leave the colour to Grafana's palette.
func instanceColour(rank int) string {
	if rank < 0 || rank >= len(instanceColours) {
		return ""
	}
	return instanceColours[rank]
}
