# Turns "benchstat -format csv base=... head=..." output into one Markdown table with a row per package and a column
# per unit: "base → head (change)". benchstat prints one CSV section per unit and "~" when a change is not
# significant.
BEGIN { FS = "," }

/^,base,/ { getline; unit = $2; next }

unit != "" && $1 != "" && $1 != "geomean" && NF >= 6 {
	name = $1
	sub(/^Run\//, "", name)
	sub(/-[0-9]+$/, "", name)
	if (!(name in seen)) { seen[name] = 1; names[++nnames] = name }
	cell[name, unit] = fmt(unit, $2) " → " fmt(unit, $4) (unit == "diags" ? "" : " (" $6 ")")
}

/^$/ { unit = "" }

function fmt(unit, v) {
	if (unit == "sec/op") {
		if (v >= 1) return sprintf("%.2f s", v)
		if (v >= 1e-3) return sprintf("%.2f ms", v * 1e3)
		return sprintf("%.2f µs", v * 1e6)
	}
	if (unit == "B/op") {
		if (v >= 2^30) return sprintf("%.2f GiB", v / 2^30)
		if (v >= 2^20) return sprintf("%.2f MiB", v / 2^20)
		return sprintf("%.1f KiB", v / 2^10)
	}
	if (unit == "allocs/op" && v >= 1000) return sprintf("%.1fk", v / 1000)
	return sprintf("%g", v)
}

END {
	n = split("sec/op B/op allocs/op diags", order, " ")
	label["sec/op"] = "time/op"; label["B/op"] = "memory/op"; label["allocs/op"] = "allocs/op"; label["diags"] = "diagnostics"
	header = "| Package |"
	sep = "|---|"
	for (i = 1; i <= n; i++) { header = header " " label[order[i]] " |"; sep = sep "---|" }
	print header
	print sep
	for (j = 1; j <= nnames; j++) {
		row = "| " names[j] " |"
		for (i = 1; i <= n; i++) row = row " " cell[names[j], order[i]] " |"
		print row
	}
}
