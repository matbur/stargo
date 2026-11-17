// Package console renders span-like blocks for human-readable terminal output.
//
// The layout mirrors Stario's TTY renderer: a clock, a status glyph, a name,
// an optional status code, and a right-aligned duration, followed by indented
// attributes and nested events. Rendering is pure; a Writer adds the mutex
// and I/O.
package console

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxWidth = 100
	stampMin = 50
	durWidth = 6
)

const (
	glyphDone   = "●"
	glyphFailed = "×"
	glyphEvent  = "◦"
	glyphElbow  = "└"
	glyphPipe   = "│"
	ellipsis    = "…"
)

const (
	styleReset = "\x1b[0m"
	styleDim   = "\x1b[2m"
	styleBold  = "\x1b[1m"
	styleRed   = "\x1b[31m"
	styleGreen = "\x1b[32m"
	styleAmber = "\x1b[33m"
)

// Attr is one attribute of a block.
type Attr struct {
	Key   string
	Value any
}

// Event is a nested item shown with a connector under the block.
type Event struct {
	Name   string
	Offset time.Duration
}

// Block is one rendered span-like item.
type Block struct {
	Start    time.Time
	Name     string
	Status   int
	Failed   bool
	Duration time.Duration
	Attrs    []Attr
	Event    *Event
}

type pair struct {
	key   string
	value string
}

// Renderer turns blocks into text. Width is capped at 100 cells.
type Renderer struct {
	width int
	color bool
}

// NewRenderer returns a renderer. A non-positive width uses 100.
func NewRenderer(width int, color bool) *Renderer {
	if width <= 0 {
		width = maxWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	return &Renderer{width: width, color: color}
}

// Width returns the effective line width.
func (r *Renderer) Width() int {
	return r.width
}

// Render returns the block as one or more lines without a trailing newline.
func (r *Renderer) Render(block Block) string {
	lines := []string{r.header(block)}
	body := r.bodyPrefix(block.Event != nil)
	avail := r.width - len([]rune(body))
	lines = append(lines, r.attrLines(block, body, avail)...)
	if block.Event != nil {
		lines = append(lines, r.eventLine(*block.Event))
	}
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func (r *Renderer) header(block Block) string {
	line := r.line()
	line.write(r.stamp(block.Start), styleDim)

	marker := glyphDone
	styles := []string{styleGreen}
	if block.Failed {
		marker = glyphFailed
		styles = []string{styleRed}
	}
	line.write(marker, styles...)
	line.write(" ")

	status := ""
	if block.Status != 0 {
		status = "  " + strconv.Itoa(block.Status)
	}
	nameWidth := r.width - line.width() - runeLen(status) - (2 + durWidth)
	if nameWidth < 8 {
		nameWidth = 8
	}
	line.write(clip(block.Name, nameWidth), styleBold)
	if status != "" {
		line.write(status, r.statusStyle(block.Status)...)
	}
	duration := formatDuration(block.Duration)
	pad := r.width - line.width() - max(durWidth, runeLen(duration))
	if pad < 2 {
		pad = 2
	}
	line.write(strings.Repeat(" ", pad))
	line.write(fmt.Sprintf("%*s", durWidth, duration), styleBold)
	return line.String()
}

func (r *Renderer) eventLine(event Event) string {
	line := r.line()
	line.write(strings.Repeat(" ", r.stampWidth()))
	line.write(glyphElbow+glyphEvent+" ", styleDim)

	offset := "+" + formatDuration(event.Offset)
	nameWidth := r.width - line.width() - (2 + max(durWidth, runeLen(offset)))
	if nameWidth < 8 {
		nameWidth = 8
	}
	line.write(clip(event.Name, nameWidth))
	pad := r.width - line.width() - max(durWidth, runeLen(offset))
	if pad < 2 {
		pad = 2
	}
	line.write(strings.Repeat(" ", pad))
	line.write(fmt.Sprintf("%*s", durWidth, offset), styleBold)
	return line.String()
}

func (r *Renderer) stamp(start time.Time) string {
	if r.width < stampMin {
		return ""
	}
	return start.Format("15:04:05") + " "
}

func (r *Renderer) bodyPrefix(nested bool) string {
	prefix := strings.Repeat(" ", r.stampWidth())
	rail := " "
	if nested {
		rail = glyphPipe
	}
	return prefix + rail + " "
}

func (r *Renderer) stampWidth() int {
	if r.width < stampMin {
		return 0
	}
	return len("15:04:05 ")
}

func (r *Renderer) statusStyle(status int) []string {
	switch {
	case status >= 500:
		return []string{styleRed, styleBold}
	case status >= 300:
		return []string{styleAmber, styleBold}
	default:
		return []string{styleGreen, styleBold}
	}
}

func (r *Renderer) attrLines(block Block, body string, avail int) []string {
	if len(block.Attrs) == 0 {
		return nil
	}
	if avail < 1 {
		avail = 1
	}

	fold := ""
	if i := strings.IndexByte(block.Name, '.'); i > 0 {
		fold = block.Name[:i+1]
	}

	type keyed struct {
		key   string
		value any
	}
	items := make([]keyed, 0, len(block.Attrs))
	for _, attr := range block.Attrs {
		key := attr.Key
		if fold != "" && strings.HasPrefix(key, fold) && len(key) > len(fold) {
			key = key[len(fold):]
		}
		items = append(items, keyed{key: key, value: attr.Value})
	}

	counts := make(map[string]int, len(items))
	for _, item := range items {
		if label, _, ok := splitLabel(item.key); ok {
			counts[label]++
		}
	}

	var flat []pair
	var groupOrder []string
	groups := make(map[string][]pair)
	for _, item := range items {
		label, leaf, ok := splitLabel(item.key)
		if ok && counts[label] >= 2 {
			if _, seen := groups[label]; !seen {
				groupOrder = append(groupOrder, label)
			}
			groups[label] = append(groups[label], pair{key: leaf, value: formatValue(item.value)})
			continue
		}
		flat = append(flat, pair{key: item.key, value: formatValue(item.value)})
	}

	var lines []string
	for _, row := range r.flow(flat, avail) {
		lines = append(lines, body+row)
	}

	labelWidth := 0
	widest := 0
	for _, label := range groupOrder {
		labelWidth = max(labelWidth, runeLen(label))
		for _, p := range groups[label] {
			widest = max(widest, runeLen(p.key)+1+runeLen(p.value))
		}
	}
	labelWidth = min(20, labelWidth)
	side := avail-labelWidth-2 >= max(24, min(40, widest))
	for _, label := range groupOrder {
		if side {
			rows := r.flow(groups[label], avail-labelWidth-2)
			prefix := padRight(clip(label, labelWidth), labelWidth) + "  "
			lines = append(lines, body+r.dim(prefix)+rows[0])
			for _, row := range rows[1:] {
				lines = append(lines, body+strings.Repeat(" ", labelWidth+2)+row)
			}
			continue
		}
		lines = append(lines, body+r.dim(clip(label, avail)))
		for _, row := range r.flow(groups[label], avail-2) {
			lines = append(lines, body+"  "+row)
		}
	}
	return lines
}

// flow packs key=value pairs into rows of avail cells.
func (r *Renderer) flow(pairs []pair, avail int) []string {
	if avail < 1 {
		avail = 1
	}
	var rows []string
	var current []string
	currentWidth := 0
	flush := func() {
		if len(current) > 0 {
			rows = append(rows, strings.Join(current, " "))
			current = nil
			currentWidth = 0
		}
	}

	for _, p := range pairs {
		value := quote(p.value)
		token := r.dim(p.key+"=") + value
		plainWidth := runeLen(p.key) + 1 + runeLen(value)
		if plainWidth > avail {
			flush()
			rows = append(rows, r.wrapToken(p.key, value, avail)...)
			continue
		}
		if len(current) > 0 && currentWidth+1+plainWidth <= avail {
			current = append(current, token)
			currentWidth += 1 + plainWidth
			continue
		}
		flush()
		current = []string{token}
		currentWidth = plainWidth
	}
	flush()
	return rows
}

func (r *Renderer) wrapToken(key, value string, avail int) []string {
	token := key + "=" + value
	rows := wrap(token, avail)
	if len(rows) == 0 {
		return nil
	}
	if strings.HasPrefix(rows[0], key+"=") {
		rows[0] = r.dim(key+"=") + rows[0][len(key)+1:]
	}
	for i := 1; i < len(rows); i++ {
		rows[i] = "  " + rows[i]
	}
	return rows
}

func splitLabel(key string) (label, leaf string, ok bool) {
	i := strings.LastIndexByte(key, '.')
	if i <= 0 || i == len(key)-1 {
		return "", "", false
	}
	return key[:i], key[i+1:], true
}

func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return v
	case time.Duration:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	default:
		return fmt.Sprint(value)
	}
}

// formatDuration mirrors Stario's duration column.
func formatDuration(d time.Duration) string {
	ns := d.Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	if ns < 1_000_000 {
		us := float64(ns) / 1e3
		if us < 10 {
			return strconv.FormatFloat(us, 'f', 1, 64) + "µs"
		}
		return strconv.FormatFloat(us, 'f', 0, 64) + "µs"
	}
	ms := float64(ns) / 1e6
	if ms < 10 {
		return strconv.FormatFloat(ms, 'f', 1, 64) + "ms"
	}
	if ms < 1000 {
		return strconv.FormatFloat(ms, 'f', 0, 64) + "ms"
	}
	if ms < 60_000 {
		return strconv.FormatFloat(ms/1000, 'f', 2, 64) + "s"
	}
	minutes := int(ms/1000) / 60
	seconds := int(ms/1000) % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes %= 60
	return fmt.Sprintf("%dh%02dm", hours, minutes)
}

func quote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func padRight(text string, width int) string {
	if n := width - runeLen(text); n > 0 {
		return text + strings.Repeat(" ", n)
	}
	return text
}

func clip(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if runeLen(text) <= width {
		return text
	}
	runes := []rune(text)
	if width == 1 {
		return ellipsis
	}
	return string(runes[:width-1]) + ellipsis
}

func wrap(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	if runeLen(text) <= width {
		return []string{text}
	}
	var rows []string
	runes := []rune(text)
	for len(runes) > width {
		rows = append(rows, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		rows = append(rows, string(runes))
	}
	return rows
}

func runeLen(text string) int {
	return utf8.RuneCountInString(text)
}

func (r *Renderer) dim(text string) string {
	if !r.color || text == "" {
		return text
	}
	return styleDim + text + styleReset
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// line accumulates styled text while tracking its plain width.
type line struct {
	color bool
	plain strings.Builder
	out   strings.Builder
}

func (r *Renderer) line() *line {
	return &line{color: r.color}
}

func (l *line) write(text string, styles ...string) {
	l.plain.WriteString(text)
	if !l.color || len(styles) == 0 {
		l.out.WriteString(text)
		return
	}
	for _, style := range styles {
		l.out.WriteString(style)
	}
	l.out.WriteString(text)
	l.out.WriteString(styleReset)
}

func (l *line) width() int {
	return runeLen(l.plain.String())
}

func (l *line) String() string {
	return l.out.String()
}
