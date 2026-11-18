package datastar_test

import (
	"math"
	"testing"
	"time"

	ds "github.com/matbur/stargo/datastar"
)

func TestOnIntersect(t *testing.T) {
	if got := renderNode(t, ds.OnIntersect("load()")); got != ` data-on-intersect="load()"` {
		t.Fatalf("OnIntersect() = %q", got)
	}
}

func TestOnIntersectModifierOrder(t *testing.T) {
	got := renderNode(t, ds.OnIntersect("load()",
		ds.WithIntersectViewTransition(),
		ds.WithIntersectThrottle(300*time.Millisecond),
		ds.WithIntersectThreshold(0.25),
		ds.WithIntersectDebounce(200*time.Millisecond),
		ds.WithIntersectExit(),
		ds.WithIntersectDelay(100*time.Millisecond),
		ds.WithIntersectOnce(),
	))
	want := ` data-on-intersect__threshold.25__once__exit__delay.100ms__debounce.200ms__throttle.300ms__viewtransition="load()"`
	if got != want {
		t.Fatalf("OnIntersect() = %q, want %q", got, want)
	}
}

func TestOnIntersectThresholds(t *testing.T) {
	tests := []struct {
		name string
		opt  ds.IntersectOption
		want string
	}{
		{"zero", ds.WithIntersectThreshold(0), ` data-on-intersect__threshold.0="x"`},
		{"half fraction", ds.WithIntersectThreshold(0.5), ` data-on-intersect__threshold.50="x"`},
		{"rounded", ds.WithIntersectThreshold(0.333), ` data-on-intersect__threshold.33="x"`},
		{"full fraction", ds.WithIntersectThreshold(1), ` data-on-intersect__threshold.100="x"`},
		{"half keyword", ds.WithIntersectHalf(), ` data-on-intersect__half="x"`},
		{"full keyword", ds.WithIntersectFull(), ` data-on-intersect__full="x"`},
		{"custom keyword", ds.WithIntersectThresholdKeyword("any-visible"), ` data-on-intersect__threshold.any-visible="x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.OnIntersect("x", tt.opt)); got != tt.want {
				t.Fatalf("OnIntersect() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnIntersectPanics(t *testing.T) {
	cases := map[string]func(){
		"below zero":         func() { ds.OnIntersect("x", ds.WithIntersectThreshold(-0.1)) },
		"above one":          func() { ds.OnIntersect("x", ds.WithIntersectThreshold(1.1)) },
		"NaN":                func() { ds.OnIntersect("x", ds.WithIntersectThreshold(math.NaN())) },
		"infinite":           func() { ds.OnIntersect("x", ds.WithIntersectThreshold(math.Inf(1))) },
		"invalid keyword":    func() { ds.OnIntersect("x", ds.WithIntersectThresholdKeyword("Bad")) },
		"empty keyword":      func() { ds.OnIntersect("x", ds.WithIntersectThresholdKeyword("")) },
		"duplicate fraction": func() { ds.OnIntersect("x", ds.WithIntersectThreshold(0.5), ds.WithIntersectThreshold(0.5)) },
		"duplicate keywords": func() { ds.OnIntersect("x", ds.WithIntersectHalf(), ds.WithIntersectFull()) },
		"short delay":        func() { ds.OnIntersect("x", ds.WithIntersectDelay(time.Microsecond)) },
		"short debounce":     func() { ds.OnIntersect("x", ds.WithIntersectDebounce(0)) },
		"short throttle":     func() { ds.OnIntersect("x", ds.WithIntersectThrottle(500*time.Microsecond)) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			assertPanic(t, "datastar:", fn)
		})
	}
}

func TestOnInterval(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.IntervalOption
		want string
	}{
		{"default", nil, ` data-on-interval="tick()"`},
		{"custom seconds", []ds.IntervalOption{ds.WithIntervalDuration(2 * time.Second)}, ` data-on-interval__duration.2s="tick()"`},
		{"leading", []ds.IntervalOption{ds.WithIntervalDuration(2 * time.Second), ds.WithIntervalLeading()}, ` data-on-interval__duration.2s.leading="tick()"`},
		{"milliseconds", []ds.IntervalOption{ds.WithIntervalDuration(1500 * time.Millisecond)}, ` data-on-interval__duration.1500ms="tick()"`},
		{"default duration with leading", []ds.IntervalOption{ds.WithIntervalLeading()}, ` data-on-interval__duration.1s.leading="tick()"`},
		{"explicit default duration", []ds.IntervalOption{ds.WithIntervalDuration(time.Second)}, ` data-on-interval="tick()"`},
		{"view transition", []ds.IntervalOption{ds.WithIntervalViewTransition()}, ` data-on-interval__viewtransition="tick()"`},
		{"all", []ds.IntervalOption{ds.WithIntervalViewTransition(), ds.WithIntervalLeading(), ds.WithIntervalDuration(2 * time.Second)}, ` data-on-interval__duration.2s.leading__viewtransition="tick()"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.OnInterval("tick()", tt.opts...)); got != tt.want {
				t.Fatalf("OnInterval() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnIntervalPanics(t *testing.T) {
	cases := map[string]func(){
		"zero":   func() { ds.OnInterval("x", ds.WithIntervalDuration(0)) },
		"sub-ms": func() { ds.OnInterval("x", ds.WithIntervalDuration(500*time.Microsecond)) },
		"sub-ms leading": func() {
			ds.OnInterval("x", ds.WithIntervalLeading(), ds.WithIntervalDuration(500*time.Microsecond))
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			assertPanic(t, "datastar:", fn)
		})
	}
}

func TestOnSignalPatch(t *testing.T) {
	got := renderNode(t, ds.OnSignalPatch("save()"))
	if got != ` data-on-signal-patch="save()"` {
		t.Fatalf("OnSignalPatch() = %q", got)
	}
	group := ds.OnSignalPatch("save()")
	if len(group) != 1 {
		t.Fatalf("OnSignalPatch() has %d attributes, want one", len(group))
	}
}

func TestOnSignalPatchModifiers(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.SignalPatchOption
		want string
	}{
		{"delay", []ds.SignalPatchOption{ds.WithSignalPatchDelay(100 * time.Millisecond)}, ` data-on-signal-patch__delay.100ms="save()"`},
		{"debounce", []ds.SignalPatchOption{ds.WithSignalPatchDebounce(500 * time.Millisecond)}, ` data-on-signal-patch__debounce.500ms="save()"`},
		{"throttle", []ds.SignalPatchOption{ds.WithSignalPatchThrottle(250 * time.Millisecond)}, ` data-on-signal-patch__throttle.250ms="save()"`},
		{"order", []ds.SignalPatchOption{
			ds.WithSignalPatchThrottle(300 * time.Millisecond),
			ds.WithSignalPatchDelay(100 * time.Millisecond),
			ds.WithSignalPatchDebounce(200 * time.Millisecond),
		}, ` data-on-signal-patch__delay.100ms__debounce.200ms__throttle.300ms="save()"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.OnSignalPatch("save()", tt.opts...)); got != tt.want {
				t.Fatalf("OnSignalPatch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnSignalPatchFilters(t *testing.T) {
	tests := []struct {
		name string
		opts []ds.SignalPatchOption
		want string
	}{
		{
			"include",
			[]ds.SignalPatchOption{ds.WithSignalPatchDebounce(500 * time.Millisecond), ds.WithSignalPatchInclude("draft")},
			` data-on-signal-patch__debounce.500ms="save()" data-on-signal-patch-filter="{&#39;include&#39;:&#39;draft&#39;}"`,
		},
		{
			"exclude",
			[]ds.SignalPatchOption{ds.WithSignalPatchExclude("secret")},
			` data-on-signal-patch="save()" data-on-signal-patch-filter="{&#39;exclude&#39;:&#39;secret&#39;}"`,
		},
		{
			"both",
			[]ds.SignalPatchOption{ds.WithSignalPatchExclude("secret"), ds.WithSignalPatchInclude("draft")},
			` data-on-signal-patch="save()" data-on-signal-patch-filter="{&#39;include&#39;:&#39;draft&#39;,&#39;exclude&#39;:&#39;secret&#39;}"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderNode(t, ds.OnSignalPatch("save()", tt.opts...)); got != tt.want {
				t.Fatalf("OnSignalPatch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnSignalPatchPrefix(t *testing.T) {
	got := renderNode(t, ds.New("data-star-").OnSignalPatch("save()", ds.WithSignalPatchInclude("draft")))
	want := ` data-star-on-signal-patch="save()" data-star-on-signal-patch-filter="{&#39;include&#39;:&#39;draft&#39;}"`
	if got != want {
		t.Fatalf("OnSignalPatch() = %q, want %q", got, want)
	}
}

func TestOnSignalPatchPanics(t *testing.T) {
	cases := map[string]func(){
		"short delay":    func() { ds.OnSignalPatch("x", ds.WithSignalPatchDelay(0)) },
		"short debounce": func() { ds.OnSignalPatch("x", ds.WithSignalPatchDebounce(time.Microsecond)) },
		"short throttle": func() { ds.OnSignalPatch("x", ds.WithSignalPatchThrottle(0)) },
		"empty include":  func() { ds.OnSignalPatch("x", ds.WithSignalPatchInclude("")) },
		"bad exclude":    func() { ds.OnSignalPatch("x", ds.WithSignalPatchExclude("a/b")) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			assertPanic(t, "datastar:", fn)
		})
	}
}
