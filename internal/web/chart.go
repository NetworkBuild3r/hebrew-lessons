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

// linePoint is one snapshot day on a line chart.
type linePoint struct {
	Day   string // YYYY-MM-DD
	Value float64
}

// lineSpec describes a line chart. Min and Max are the ends of the y axis. Ticks are the
// values labelled on it. Format writes a value as text. Band, when set, is the target band:
// a shaded stripe with dashed edges between its two values.
type lineSpec struct {
	ID     string
	Label  string
	Points []linePoint
	Min    float64
	Max    float64
	Ticks  []float64
	Format func(float64) string
	Band   *[2]float64
}

// lineChartSVG draws spec. It returns "" when there are no points. One point is a lone dot, with no
// path: a line needs two days.
func lineChartSVG(spec lineSpec) string {
	if len(spec.Points) == 0 {
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
	t0, _ := time.Parse("2006-01-02", spec.Points[0].Day)
	t1, _ := time.Parse("2006-01-02", spec.Points[len(spec.Points)-1].Day)
	x := func(day string) float64 {
		if len(spec.Points) == 1 || !t1.After(t0) {
			return float64(padLeft) + plotW/2
		}
		t, _ := time.Parse("2006-01-02", day)
		return float64(padLeft) + plotW*float64(t.Sub(t0))/float64(t1.Sub(t0))
	}

	first, last := spec.Points[0], spec.Points[len(spec.Points)-1]
	desc := spec.Label + ": "
	if len(spec.Points) == 1 {
		desc += fmt.Sprintf("%s on %s, one daily snapshot.", format(first.Value), first.Day)
	} else {
		desc += fmt.Sprintf("%s on %s, then %s on %s, from %d daily snapshots.", format(first.Value), first.Day, format(last.Value), last.Day, len(spec.Points))
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
	if len(spec.Points) > 1 {
		var d strings.Builder
		for i, p := range spec.Points {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%s %s ", cmd, num(x(p.Day)), num(y(p.Value)))
		}
		fmt.Fprintf(&b, `<path class="pc__line" d="%s"/>`, strings.TrimSpace(d.String()))
	}
	// Every day gets a dot while there are few enough to tell apart, else only the newest does.
	for i, p := range spec.Points {
		if len(spec.Points) <= 31 || i == len(spec.Points)-1 {
			cls := "pc__dot"
			if i == len(spec.Points)-1 {
				cls = "pc__dot pc__dot--last"
			}
			fmt.Fprintf(&b, `<circle class="%s" cx="%s" cy="%s" r="%d"/>`, cls, num(x(p.Day)), num(y(p.Value)), dotR(i == len(spec.Points)-1))
		}
	}
	anchor := "start"
	if len(spec.Points) == 1 {
		anchor = "middle"
	}
	fmt.Fprintf(&b, `<text class="pc__tick" x="%s" y="%d" text-anchor="%s">%s</text>`, num(x(first.Day)), lineH-6, anchor, esc(shortDay(first.Day)))
	if len(spec.Points) > 1 {
		fmt.Fprintf(&b, `<text class="pc__tick" x="%s" y="%d" text-anchor="end">%s</text>`, num(x(last.Day)), lineH-6, esc(shortDay(last.Day)))
	}
	b.WriteString(`</svg>`)
	return b.String()
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
