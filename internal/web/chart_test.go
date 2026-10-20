package web

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Unit tests of the SVG builders (INIT-015/SPEC-018). The page tests in progress_test.go check
// the numbers; these check that the geometry is sound for any input.

var numberAttr = regexp.MustCompile(`\b(?:x|y|x1|x2|y1|y2|cx|cy|width|height|r|rx)="(-?[0-9.]+)"`)

func TestStackedBarWidthsFitTheViewBoxAndTrackTheCounts(t *testing.T) {
	segs := []barSeg{{"unseen", 212}, {"seen", 96}, {"struggling", 7}, {"regressed", 0}, {"learning", 58}, {"known", 52}, {"solid", 1}}
	svg := stackedBarSVG("pb", "Words by state", segs)
	rects := regexp.MustCompile(`<rect class="pc__seg" data-state="([a-z]+)" x="([0-9.]+)" y="0" width="([0-9.]+)"`).FindAllStringSubmatch(svg, -1)
	if len(rects) != 6 {
		t.Fatalf("%d segments, want 6 (the state with 0 words is left out):\n%s", len(rects), svg)
	}
	end := 0.0
	for i, r := range rects {
		x, _ := strconv.ParseFloat(r[2], 64)
		w, _ := strconv.ParseFloat(r[3], 64)
		if i > 0 && math.Abs(x-(end+barGap)) > 0.11 {
			t.Errorf("segment %s starts at %v, want %v: a gap of %d", r[1], x, end+barGap, barGap)
		}
		if w < barMin-0.01 {
			t.Errorf("segment %s is %v wide, under the minimum %d", r[1], w, barMin)
		}
		end = x + w
	}
	if end > chartW+0.2 {
		t.Errorf("the bar is %v wide, the box is %d", end, chartW)
	}
	// A bigger count is a wider segment, up to the minimum.
	wUnseen, _ := strconv.ParseFloat(rects[0][3], 64)
	wSeen, _ := strconv.ParseFloat(rects[1][3], 64)
	if wUnseen <= wSeen {
		t.Errorf("212 words (%v) is not wider than 96 words (%v)", wUnseen, wSeen)
	}
}

func TestStackedBarHatchesStrugglingAndSlippingOnly(t *testing.T) {
	segs := []barSeg{{"seen", 5}, {"struggling", 5}, {"regressed", 5}, {"known", 5}}
	svg := stackedBarSVG("x", "l", segs)
	if got := strings.Count(svg, `class="pc__seg-hatch"`); got != 2 {
		t.Fatalf("%d hatched segments, want 2", got)
	}
	if !strings.Contains(svg, `data-state="struggling" x=`) || !regexp.MustCompile(`data-state="struggling"[^>]*/><rect class="pc__seg-hatch"[^>]*fill="url\(#x-ha\)"`).MatchString(svg) {
		t.Error("struggling is not hatched one way")
	}
	if !regexp.MustCompile(`data-state="regressed"[^>]*/><rect class="pc__seg-hatch"[^>]*fill="url\(#x-hb\)"`).MatchString(svg) {
		t.Error("slipping is not hatched the other way")
	}
}

func TestStackedBarOfNothingIsEmpty(t *testing.T) {
	if svg := stackedBarSVG("pb", "none", []barSeg{{"unseen", 0}, {"seen", 0}}); svg != "" {
		t.Errorf("an empty bar drew %q", svg)
	}
	if svg := stackedBarSVG("pb", "none", nil); svg != "" {
		t.Errorf("no states drew %q", svg)
	}
	one := stackedBarSVG("pb", "one", []barSeg{{"known", 9}})
	if !strings.Contains(one, `width="340"`) {
		t.Errorf("a single state should fill the bar:\n%s", one)
	}
}

func TestChartMarkupEscapesWhatItIsGiven(t *testing.T) {
	hostile := `"><script>alert(1)</script>`
	for name, svg := range map[string]string{
		"bar":  stackedBarSVG(hostile, hostile, []barSeg{{"known", 3}, {hostile, 2}}),
		"line": lineChartSVG(lineSpec{ID: hostile, Label: hostile, Points: []linePoint{{"2026-10-01", 1}, {"2026-10-02", 2}}, Min: 0, Max: 3, Ticks: []float64{0, 3}}),
	} {
		if strings.Contains(svg, "<script") || strings.Contains(svg, `"><`+"script") {
			t.Errorf("%s: a script tag got through:\n%s", name, svg)
		}
		if !strings.Contains(svg, "&lt;script&gt;") && !strings.Contains(svg, "&#34;&gt;&lt;script&gt;") {
			t.Errorf("%s: the label is not in the markup, escaped", name)
		}
	}
}

func TestLineChartStaysInsideTheViewBoxForAnyData(t *testing.T) {
	days := func(n int) []string {
		var out []string
		for i := 0; i < n; i++ {
			out = append(out, addDaysStr("2026-07-01", i))
		}
		return out
	}
	cases := map[string][]float64{
		"one point":    {5},
		"two points":   {5, 6},
		"flat":         {7, 7, 7, 7},
		"zero":         {0, 0, 0},
		"falling":      {90, 60, 30, 0},
		"outside axis": {-50, 500, 20},
		"ninety": func() []float64 {
			v := make([]float64, 90)
			for i := range v {
				v[i] = 100 + float64(i)*0.4
			}
			return v
		}(),
	}
	for name, vals := range cases {
		t.Run(name, func(t *testing.T) {
			ds := days(len(vals))
			var pts []linePoint
			lo, hi := vals[0], vals[0]
			for i, v := range vals {
				pts = append(pts, linePoint{Day: ds[i], Value: v})
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
			minY, maxY, ticks := niceCount(lo, hi)
			svg := lineChartSVG(lineSpec{ID: "l", Label: name, Points: pts, Min: minY, Max: maxY, Ticks: ticks})
			if svg == "" || strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") || strings.Contains(svg, `d=""`) {
				t.Fatalf("broken svg:\n%s", svg)
			}
			for _, m := range numberAttr.FindAllStringSubmatch(svg, -1) {
				f, _ := strconv.ParseFloat(m[1], 64)
				if f < -0.01 || f > chartW+0.01 {
					t.Errorf("coordinate %s is outside the box", m[0])
				}
			}
			for _, m := range regexp.MustCompile(`\b(?:y|y1|y2|cy)="(-?[0-9.]+)"`).FindAllStringSubmatch(svg, -1) {
				f, _ := strconv.ParseFloat(m[1], 64)
				if f < -0.01 || f > lineH+0.01 {
					t.Errorf("y coordinate %s is outside the box", m[0])
				}
			}
			wantLine := len(vals) > 1
			if has := strings.Contains(svg, `class="pc__line"`); has != wantLine {
				t.Errorf("line present = %v, want %v", has, wantLine)
			}
		})
	}
	if svg := lineChartSVG(lineSpec{ID: "l", Label: "none"}); svg != "" {
		t.Errorf("no points drew %q", svg)
	}
}

// A value outside the axis it was given is drawn on the edge, not off the chart, and an axis with
// no height does not divide by zero.
func TestLineChartClampsToItsAxisAndSurvivesAFlatOne(t *testing.T) {
	over := lineChartSVG(lineSpec{
		ID: "l", Label: "over", Points: []linePoint{{"2026-10-01", 50}, {"2026-10-02", -20}, {"2026-10-03", 5}},
		Min: 0, Max: 10, Ticks: []float64{0, 10},
	})
	for _, m := range regexp.MustCompile(`\b(?:cy|y1|y2)="(-?[0-9.]+)"`).FindAllStringSubmatch(over, -1) {
		f, _ := strconv.ParseFloat(m[1], 64)
		if f < float64(padTop)-0.01 || f > float64(lineH-padBottom)+0.01 {
			t.Errorf("%s is off the plot area (%d to %d)", m[0], padTop, lineH-padBottom)
		}
	}
	flat := lineChartSVG(lineSpec{ID: "f", Label: "flat", Points: []linePoint{{"2026-10-01", 5}, {"2026-10-02", 5}}, Min: 5, Max: 5, Ticks: []float64{5}})
	if flat == "" || strings.Contains(flat, "NaN") || strings.Contains(flat, "Inf") {
		t.Errorf("a flat axis broke the chart:\n%s", flat)
	}
}

func TestLineChartBandSitsBetweenItsTwoValues(t *testing.T) {
	b := [2]float64{0.85, 0.90}
	svg := lineChartSVG(lineSpec{
		ID: "r", Label: "Retention", Points: []linePoint{{"2026-10-01", 0.8}, {"2026-10-02", 0.9}},
		Min: 0.7, Max: 1, Ticks: []float64{0.7, 0.85, 1}, Format: percent, Band: &b,
	})
	m := regexp.MustCompile(`<rect class="pc__band" x="[0-9.]+" y="([0-9.]+)" width="[0-9.]+" height="([0-9.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no band:\n%s", svg)
	}
	top, _ := strconv.ParseFloat(m[1], 64)
	height, _ := strconv.ParseFloat(m[2], 64)
	plotH := float64(lineH - padTop - padBottom)
	wantTop := float64(padTop) + plotH*(1-(0.90-0.7)/0.3)
	wantH := plotH * 0.05 / 0.3
	if math.Abs(top-wantTop) > 0.11 || math.Abs(height-wantH) > 0.11 {
		t.Errorf("band at y=%v h=%v, want y=%.1f h=%.1f", top, height, wantTop, wantH)
	}
	if strings.Count(svg, `class="pc__band-edge"`) != 2 {
		t.Error("the band has no dashed edges")
	}
	if !strings.Contains(svg, ">85%</text>") || !strings.Contains(svg, ">70%</text>") {
		t.Error("the axis does not name its values")
	}
}

func TestAxesCoverTheirData(t *testing.T) {
	lo, hi, ticks := niceCount(118, 164)
	if lo > 118 || hi < 164 || len(ticks) != 3 || ticks[0] != lo || ticks[2] != hi {
		t.Errorf("count axis %v..%v ticks %v", lo, hi, ticks)
	}
	if lo, hi, _ := niceCount(5, 5); hi <= lo || lo < 0 {
		t.Errorf("flat data gives axis %v..%v", lo, hi)
	}
	for _, v := range []float64{0.2, 0.55, 0.78, 0.95, 1} {
		lo, hi, ticks := niceShare(v)
		if lo > v || hi != 1 || lo > 0.7 || lo < 0 || len(ticks) != 3 {
			t.Errorf("share axis for %v = %v..%v %v", v, lo, hi, ticks)
		}
	}
}
