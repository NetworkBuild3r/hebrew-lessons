package web

import (
	"fmt"
	"html"
	"math"
	"strings"
	"time"
)

// The SVG charts of Progress (INIT-015/SPEC-018). They are built here, as strings, with no
// chart library. Nothing in the markup names a colour: every shape carries a class (pc__...) and
// progress.css paints the class from the Lamplight tokens. Every number and label that goes into
// the markup is escaped, and the only unescaped part is the fixed skeleton below.
//
// A chart is also always given as a table (views.ChartTable), so a screen reader has the same
// values, and the state bar does not rely on colour alone: the two states that need attention
// carry a hatch, and the legend beside the bar names and counts every state.

const (
	// chartW is the width of every chart's viewBox. It is close to the inside of a card on a
	// phone, so the tick text is near its real size there.
	chartW = 340
	// barH is the height of the state bar.
	barH = 18
	// lineH is the height of a line chart's viewBox.
	lineH = 170

	barGap = 2
	// barMin is the narrowest a segment is drawn, so a state with a few words is still seen.
	barMin = 4

	padLeft   = 38
	padRight  = 10
	padTop    = 10
	padBottom = 24
)

// barSeg is one state of the stacked bar.
type barSeg struct {
	State string
	Count int
}

// hatchStates are the states drawn with a hatch on top of their fill. They are the two that
// need a second look, so they stay apart from their neighbours without a colour.
var hatchStates = map[string]string{"struggling": "a", "regressed": "b"}

// stackedBarSVG draws the segments of segs, in order, as one bar whose widths follow the counts.
// A state with a count of 0 is left out. It returns "" when every count is 0. id is made part of
// the pattern ids, so two charts on one page do not share them. label is the accessible name.
func stackedBarSVG(id, label string, segs []barSeg) string {
	total := 0
	var shown []barSeg
	for _, s := range segs {
		if s.Count > 0 {
			total += s.Count
			shown = append(shown, s)
		}
	}
	if total == 0 {
		return ""
	}
	avail := float64(chartW - barGap*(len(shown)-1))
	widths := make([]float64, len(shown))
	deficit := 0.0
	big := 0
	for i, s := range shown {
		w := avail * float64(s.Count) / float64(total)
		if w < barMin {
			deficit += barMin - w
			w = barMin
		}
		widths[i] = w
		if widths[i] > widths[big] {
			big = i
		}
	}
	// A narrow state is widened to barMin. The widest one gives that width back.
	widths[big] -= deficit

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="pc pc--bar" viewBox="0 0 %d %d" role="img" aria-label="%s" focusable="false">`, chartW, barH, esc(label))
	pid := esc(id)
	fmt.Fprintf(&b, `<defs><pattern id="%s-ha" class="pc__pattern" width="7" height="7" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><line class="pc__hatch" x1="0" y1="0" x2="0" y2="7"/></pattern>`, pid)
	fmt.Fprintf(&b, `<pattern id="%s-hb" class="pc__pattern" width="7" height="7" patternUnits="userSpaceOnUse" patternTransform="rotate(-45)"><line class="pc__hatch" x1="0" y1="0" x2="0" y2="7"/></pattern></defs>`, pid)
	x := 0.0
	for i, s := range shown {
		fmt.Fprintf(&b, `<rect class="pc__seg" data-state="%s" x="%s" y="0" width="%s" height="%d" rx="4"/>`, esc(s.State), num(x), num(widths[i]), barH)
		if kind, ok := hatchStates[s.State]; ok {
			fmt.Fprintf(&b, `<rect class="pc__seg-hatch" x="%s" y="0" width="%s" height="%d" rx="4" fill="url(#%s-h%s)"/>`, num(x), num(widths[i]), barH, pid, kind)
		}
		x += widths[i] + barGap
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// linePoint is one day on a line chart. OldRule is true when the day was written
// below the current rule version (ADR-0028 D3 / ADR-0030 D4); those segments are drawn dashed.
type linePoint struct {
	Day     string // YYYY-MM-DD
	Value   float64
	OldRule bool
}

// lineSeries is one named line on a multi-series chart (ADR-0028 D1).
type lineSeries struct {
	ID     string // "known", "read", …
	Label  string
	Thin   bool
	Points []linePoint
}

// lineSpec describes a line chart. Min and Max are the ends of the y axis. Ticks are the
// values labelled on it. Format writes a value as text. Band, when set, is the target band:
// a shaded stripe with dashed edges between its two values. Series, when set, draws several
// lines; Points is then ignored.
type lineSpec struct {
	ID     string
	Label  string
	Points []linePoint
	Series []lineSeries
	Min    float64
	Max    float64
	Ticks  []float64
	Format func(float64) string
	Band   *[2]float64
}

// lineChartSVG draws spec. It returns "" when there are no points. One point is a lone dot, with no
// path: a line needs two days. Old-rule runs are dashed (ADR-0028 D3).
func lineChartSVG(spec lineSpec) string {
	series := spec.Series
	if len(series) == 0 && len(spec.Points) > 0 {
		series = []lineSeries{{ID: "main", Label: spec.Label, Points: spec.Points}}
	}
	if len(series) == 0 {
		return ""
	}
	var all []linePoint
	for _, s := range series {
		all = append(all, s.Points...)
	}
	if len(all) == 0 {
		return ""
	}
	format := spec.Format
	if format == nil {
		format = func(v float64) string { return fmt.Sprintf("%.0f", v) }
	}
	plotW := float64(chartW - padLeft - padRight)
	plotH := float64(lineH - padTop - padBottom)
	span := spec.Max - spec.Min
	if span <= 0 {
		span = 1
	}
	y := func(v float64) float64 {
		v = math.Min(math.Max(v, spec.Min), spec.Max)
		return float64(padTop) + plotH*(1-(v-spec.Min)/span)
	}
	// X axis spans the earliest and latest day across every series.
	dayLo, dayHi := all[0].Day, all[0].Day
	for _, p := range all {
		if p.Day < dayLo {
			dayLo = p.Day
		}
		if p.Day > dayHi {
			dayHi = p.Day
		}
	}
	t0, _ := time.Parse("2006-01-02", dayLo)
	t1, _ := time.Parse("2006-01-02", dayHi)
	x := func(day string) float64 {
		if dayLo == dayHi || !t1.After(t0) {
			return float64(padLeft) + plotW/2
		}
		t, _ := time.Parse("2006-01-02", day)
		return float64(padLeft) + plotW*float64(t.Sub(t0))/float64(t1.Sub(t0))
	}

	desc := spec.Label + ": "
	first, last := all[0], all[len(all)-1]
	// Prefer the primary series' ends for the accessible summary when present.
	if len(series[0].Points) > 0 {
		first, last = series[0].Points[0], series[0].Points[len(series[0].Points)-1]
	}
	if len(series) == 1 && len(series[0].Points) == 1 {
		desc += fmt.Sprintf("%s on %s, one daily snapshot.", format(first.Value), first.Day)
	} else {
		desc += fmt.Sprintf("%s on %s, then %s on %s, from %d daily snapshots.", format(first.Value), first.Day, format(last.Value), last.Day, len(series[0].Points))
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="pc pc--line" viewBox="0 0 %d %d" role="img" aria-label="%s" focusable="false">`, chartW, lineH, esc(desc))
	for _, tick := range spec.Ticks {
		ty := y(tick)
		fmt.Fprintf(&b, `<line class="pc__grid" x1="%d" x2="%d" y1="%s" y2="%s"/>`, padLeft, chartW-padRight, num(ty), num(ty))
		fmt.Fprintf(&b, `<text class="pc__tick" x="%d" y="%s" text-anchor="end" dominant-baseline="middle">%s</text>`, padLeft-6, num(ty), esc(format(tick)))
	}
	if spec.Band != nil {
		top, bottom := y(spec.Band[1]), y(spec.Band[0])
		fmt.Fprintf(&b, `<rect class="pc__band" x="%d" y="%s" width="%s" height="%s"/>`, padLeft, num(top), num(plotW), num(bottom-top))
		for _, by := range []float64{top, bottom} {
			fmt.Fprintf(&b, `<line class="pc__band-edge" x1="%d" x2="%d" y1="%s" y2="%s"/>`, padLeft, chartW-padRight, num(by), num(by))
		}
	}
	for _, s := range series {
		writeLineSeries(&b, s, x, y)
	}
	anchor := "start"
	if dayLo == dayHi {
		anchor = "middle"
	}
	fmt.Fprintf(&b, `<text class="pc__tick" x="%s" y="%d" text-anchor="%s">%s</text>`, num(x(dayLo)), lineH-6, anchor, esc(shortDay(dayLo)))
	if dayLo != dayHi {
		fmt.Fprintf(&b, `<text class="pc__tick" x="%s" y="%d" text-anchor="end">%s</text>`, num(x(dayHi)), lineH-6, esc(shortDay(dayHi)))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// writeLineSeries draws one series. A segment is dashed when either end is an
// OldRule day (ADR-0028 D3 / ADR-0030 D4 / SPEC-007).
func writeLineSeries(b *strings.Builder, s lineSeries, x func(string) float64, y func(float64) float64) {
	if len(s.Points) == 0 {
		return
	}
	cls := "pc__line"
	if s.Thin {
		cls += " pc__line--thin"
	}
	for i := 1; i < len(s.Points); i++ {
		a, c := s.Points[i-1], s.Points[i]
		segCls := cls
		if a.OldRule || c.OldRule {
			segCls += " pc__line--old"
		}
		fmt.Fprintf(b, `<path class="%s" data-series="%s" d="M%s %s L%s %s"/>`,
			segCls, esc(s.ID), num(x(a.Day)), num(y(a.Value)), num(x(c.Day)), num(y(c.Value)))
	}
	n := len(s.Points)
	for i, p := range s.Points {
		if n <= 31 || i == n-1 {
			dotCls := "pc__dot"
			if i == n-1 {
				dotCls = "pc__dot pc__dot--last"
			}
			if s.Thin {
				dotCls += " pc__dot--thin"
			}
			fmt.Fprintf(b, `<circle class="%s" data-series="%s" cx="%s" cy="%s" r="%d"/>`,
				dotCls, esc(s.ID), num(x(p.Day)), num(y(p.Value)), dotR(i == n-1 && !s.Thin))
		}
	}
}

func dotR(last bool) int {
	if last {
		return 5
	}
	return 3
}

// num writes a coordinate with at most one decimal, and no trailing zero.
func num(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	return strings.TrimSuffix(s, ".0")
}

func esc(s string) string { return html.EscapeString(s) }

// shortDay is "Oct 18" for 2026-10-18.
func shortDay(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.Format("Jan 2")
}

// niceCount picks the y axis of the estimated-words chart: the data with a margin, at whole
// words, and three ticks (bottom, middle, top). The axis does not start at zero on purpose, so
// a slow climb is readable; the table beside the chart has every value.
func niceCount(lo, hi float64) (float64, float64, []float64) {
	pad := math.Max((hi-lo)*0.15, 2)
	min := math.Max(math.Floor(lo-pad), 0)
	max := math.Ceil(hi + pad)
	if max <= min {
		max = min + 1
	}
	mid := math.Round((min + max) / 2)
	return min, max, []float64{min, mid, max}
}

// niceShare picks the y axis of the retention chart: from a floor below the data (at most 70%, so the
// target band is always inside) up to 100%, with three ticks.
func niceShare(lo float64) (float64, float64, []float64) {
	min := math.Floor((lo-0.05)*10) / 10
	min = math.Max(0, math.Min(min, 0.7))
	return min, 1, []float64{min, (min + 1) / 2, 1}
}

func percent(v float64) string { return fmt.Sprintf("%.0f%%", v*100) }
