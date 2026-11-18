package plugin

// grafanaUnit maps a DBmarlin statistic unit to a Grafana unit id. Units
// Grafana has no id for are shown as a suffix, so the value is still labelled.
func grafanaUnit(units string) string {
	switch units {
	case "":
		return "short"
	case "%":
		return "percent"
	case "KB":
		return "kbytes"
	case "MB":
		return "mbytes"
	case "KB/s":
		return "KBs"
	case "ms", "s", "µs":
		return units
	default:
		return "suffix:" + units
	}
}
