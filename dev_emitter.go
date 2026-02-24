package wideevent

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

// ANSI escape codes for terminal colors.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// DevEmitterOption configures a dev emitter.
type DevEmitterOption func(*devEmitterConfig)

// categoryRule maps a named category to a set of path prefixes.
type categoryRule struct {
	name     string
	prefixes []string
}

type devEmitterConfig struct {
	color          *bool // nil = auto-detect
	categories     []categoryRule
	muteCategories map[string]bool
}

// WithColor forces color output on or off, overriding TTY auto-detection.
func WithColor(enabled bool) DevEmitterOption {
	return func(cfg *devEmitterConfig) {
		cfg.color = &enabled
	}
}

// WithCategory groups request paths under a named category using prefix matching.
// Events whose request.path starts with any of the given prefixes are assigned to this category.
// Categories can be displayed in the header and selectively muted via WithMuteCategories.
func WithCategory(name string, pathPrefixes ...string) DevEmitterOption {
	return func(cfg *devEmitterConfig) {
		cfg.categories = append(cfg.categories, categoryRule{name: name, prefixes: pathPrefixes})
	}
}

// WithMuteCategories silently drops events belonging to the named categories from dev output.
// Events in muted categories produce no output at all.
func WithMuteCategories(names ...string) DevEmitterOption {
	return func(cfg *devEmitterConfig) {
		if cfg.muteCategories == nil {
			cfg.muteCategories = make(map[string]bool)
		}
		for _, n := range names {
			cfg.muteCategories[n] = true
		}
	}
}

// DevStdoutEmitter returns an Emitter that writes human-readable, colorized
// wide events to stdout using unicode tree-line formatting.
// Colors are auto-detected based on whether stdout is a terminal.
func DevStdoutEmitter(opts ...DevEmitterOption) Emitter {
	autoColor := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	return newDevEmitter(os.Stdout, autoColor, opts...)
}

// DevWriterEmitter returns a dev emitter that writes to the given writer.
// Colors are disabled by default; use WithColor(true) to force them on.
func DevWriterEmitter(w io.Writer, opts ...DevEmitterOption) Emitter {
	return newDevEmitter(w, false, opts...)
}

func newDevEmitter(w io.Writer, defaultColor bool, opts ...DevEmitterOption) Emitter {
	cfg := &devEmitterConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	useColor := defaultColor
	if cfg.color != nil {
		useColor = *cfg.color
	}
	return &devEmitter{
		w:              w,
		color:          useColor,
		categories:     cfg.categories,
		muteCategories: cfg.muteCategories,
	}
}

type devEmitter struct {
	w              io.Writer
	color          bool
	categories     []categoryRule
	muteCategories map[string]bool
}

// Fields shown in the header/footer lines, excluded from the grouped body.
var headerFooterKeys = map[string]bool{
	"request.method":      true,
	"request.path":        true,
	"response.status":     true,
	"response.latency_ms": true,
	"name":                true,
	"outcome":             true,
	"error":               true,
	"duration":            true,
}

func (d *devEmitter) Emit(evt *Event) {
	fields := evt.Fields()
	level := inferLevel(evt)

	// Extract well-known fields for header and footer.
	var (
		method, path, name, outcome, errMsg string
		status                              int
		latencyMs                           float64
		duration                            time.Duration
		hasDuration                         bool
	)
	for _, f := range fields {
		switch f.Key {
		case "request.method":
			method, _ = f.Value.(string)
		case "request.path":
			path, _ = f.Value.(string)
		case "response.status":
			status, _ = f.Value.(int)
		case "response.latency_ms":
			latencyMs, _ = f.Value.(float64)
		case "name":
			name, _ = f.Value.(string)
		case "outcome":
			outcome, _ = f.Value.(string)
		case "error":
			errMsg, _ = f.Value.(string)
		case "duration":
			if dur, ok := f.Value.(time.Duration); ok {
				duration = dur
				hasDuration = true
			}
		}
	}

	// Match category and check muting.
	category := d.matchCategory(path)
	if category != "" && d.muteCategories[category] {
		return
	}

	// Build grouped fields, excluding header/footer fields.
	groups, ungrouped := d.groupFields(fields)

	var b strings.Builder

	// Header: ┌ METHOD PATH  [category]  STATUS  LATENCYms
	d.writeHeader(&b, level, method, path, name, category, status, latencyMs)

	// Body: grouped field sections
	hasBody := len(groups) > 0 || len(ungrouped) > 0
	if hasBody {
		d.writeDim(&b, "│")
		b.WriteByte('\n')
		d.writeGroupedBody(&b, groups, ungrouped)
		d.writeDim(&b, "│")
		b.WriteByte('\n')
	}

	// Footer: └ outcome=success  duration=12.5ms
	d.writeFooter(&b, outcome, errMsg, hasDuration, duration)

	fmt.Fprint(d.w, b.String())
}

// fieldEntry is a field with its display key (prefix stripped).
type fieldEntry struct {
	displayKey string
	value      any
}

func (d *devEmitter) groupFields(fields []Field) (orderedGroups []groupEntry, ungrouped []fieldEntry) {
	groupMap := make(map[string][]fieldEntry)
	var groupOrder []string
	seen := make(map[string]bool)

	for _, f := range fields {
		if headerFooterKeys[f.Key] {
			continue
		}

		dotIdx := strings.IndexByte(f.Key, '.')
		if dotIdx < 0 {
			ungrouped = append(ungrouped, fieldEntry{displayKey: f.Key, value: f.Value})
			continue
		}

		prefix := f.Key[:dotIdx]
		suffix := f.Key[dotIdx+1:]
		groupMap[prefix] = append(groupMap[prefix], fieldEntry{displayKey: suffix, value: f.Value})
		if !seen[prefix] {
			seen[prefix] = true
			groupOrder = append(groupOrder, prefix)
		}
	}

	// Stable order: request, response, user first, then remaining in insertion order.
	priority := map[string]int{"request": 0, "response": 1, "user": 2}
	sort.SliceStable(groupOrder, func(i, j int) bool {
		pi, oki := priority[groupOrder[i]]
		pj, okj := priority[groupOrder[j]]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return false // keep insertion order for the rest
	})

	for _, prefix := range groupOrder {
		orderedGroups = append(orderedGroups, groupEntry{prefix: prefix, fields: groupMap[prefix]})
	}
	return orderedGroups, ungrouped
}

type groupEntry struct {
	prefix string
	fields []fieldEntry
}

// matchCategory returns the category name for a path, or "" if none match.
func (d *devEmitter) matchCategory(path string) string {
	for _, rule := range d.categories {
		for _, prefix := range rule.prefixes {
			if strings.HasPrefix(path, prefix) {
				return rule.name
			}
		}
	}
	return ""
}

func (d *devEmitter) writeHeader(b *strings.Builder, level, method, path, name, category string, status int, latencyMs float64) {
	levelColor := d.levelColor(level)
	d.writeColor(b, levelColor+ansiBold)
	b.WriteString("┌ ")

	if method != "" && path != "" {
		b.WriteString(method)
		b.WriteByte(' ')
		b.WriteString(path)
		d.writeColor(b, ansiReset)

		if category != "" {
			b.WriteString("  ")
			d.writeDim(b, "["+category+"]")
		}

		if status > 0 {
			b.WriteString("  ")
			d.writeColor(b, d.statusColor(status)+ansiBold)
			fmt.Fprintf(b, "%d", status)
			d.writeColor(b, ansiReset)
		}
		if latencyMs > 0 {
			b.WriteString("  ")
			fmt.Fprintf(b, "%.1fms", latencyMs)
		}
	} else if name != "" {
		b.WriteString(name)
		d.writeColor(b, ansiReset)
	} else {
		b.WriteString("wide_event")
		d.writeColor(b, ansiReset)
	}

	b.WriteByte('\n')
}

func (d *devEmitter) writeGroupedBody(b *strings.Builder, groups []groupEntry, ungrouped []fieldEntry) {
	for _, g := range groups {
		d.writeGroup(b, g.prefix, g.fields)
	}
	if len(ungrouped) > 0 {
		d.writeUngrouped(b, ungrouped)
	}
}

const (
	maxLineWidth = 90
	prefixWidth  = 10
	bodyIndent   = 3 // "│  "
)

func (d *devEmitter) writeGroup(b *strings.Builder, prefix string, entries []fieldEntry) {
	d.writeDim(b, "│")
	b.WriteString("  ")
	d.writeColor(b, ansiCyan)
	fmt.Fprintf(b, "%-*s", prefixWidth, prefix)
	d.writeColor(b, ansiReset)

	lineLen := bodyIndent + prefixWidth
	for i, e := range entries {
		plainPair := fmt.Sprintf("%s=%s", e.displayKey, devFormatValue(e.value))
		if i > 0 && lineLen+len(plainPair)+2 > maxLineWidth {
			b.WriteByte('\n')
			d.writeDim(b, "│")
			b.WriteString(strings.Repeat(" ", 2+prefixWidth))
			lineLen = bodyIndent + prefixWidth
		} else if i > 0 {
			b.WriteString("  ")
			lineLen += 2
		}
		d.writeKeyValue(b, e.displayKey, devFormatValue(e.value))
		lineLen += len(plainPair)
	}
	b.WriteByte('\n')
}

func (d *devEmitter) writeUngrouped(b *strings.Builder, entries []fieldEntry) {
	d.writeDim(b, "│")
	b.WriteString(strings.Repeat(" ", 2+prefixWidth))
	lineLen := bodyIndent + prefixWidth
	for i, e := range entries {
		plainPair := fmt.Sprintf("%s=%s", e.displayKey, devFormatValue(e.value))
		if i > 0 && lineLen+len(plainPair)+2 > maxLineWidth {
			b.WriteByte('\n')
			d.writeDim(b, "│")
			b.WriteString(strings.Repeat(" ", 2+prefixWidth))
			lineLen = bodyIndent + prefixWidth
		} else if i > 0 {
			b.WriteString("  ")
			lineLen += 2
		}
		d.writeKeyValue(b, e.displayKey, devFormatValue(e.value))
		lineLen += len(plainPair)
	}
	b.WriteByte('\n')
}

func (d *devEmitter) writeFooter(b *strings.Builder, outcome, errMsg string, hasDuration bool, duration time.Duration) {
	d.writeDim(b, "└ ")

	first := true
	if outcome != "" {
		d.writeKeyValue(b, "outcome", outcome)
		first = false
	}
	if errMsg != "" {
		if !first {
			b.WriteString("  ")
		}
		d.writeDim(b, "error=")
		d.writeColor(b, ansiRed)
		b.WriteString(errMsg)
		d.writeColor(b, ansiReset)
		first = false
	}
	if hasDuration {
		if !first {
			b.WriteString("  ")
		}
		d.writeKeyValue(b, "duration", devFormatDuration(duration))
	}
	d.writeColor(b, ansiReset)
	b.WriteByte('\n')
}

// Color helpers

func (d *devEmitter) writeColor(b *strings.Builder, code string) {
	if d.color {
		b.WriteString(code)
	}
}

func (d *devEmitter) writeDim(b *strings.Builder, s string) {
	d.writeColor(b, ansiDim)
	b.WriteString(s)
	d.writeColor(b, ansiReset)
}

func (d *devEmitter) writeKeyValue(b *strings.Builder, key, value string) {
	d.writeDim(b, key+"=")
	b.WriteString(value)
}

func (d *devEmitter) levelColor(level string) string {
	switch level {
	case "error":
		return ansiRed
	case "warn":
		return ansiYellow
	default:
		return ansiGreen
	}
}

func (d *devEmitter) statusColor(code int) string {
	switch {
	case code >= 500:
		return ansiRed
	case code >= 400:
		return ansiYellow
	default:
		return ansiGreen
	}
}

// Value formatting for dev output

func devFormatValue(v any) string {
	switch val := v.(type) {
	case time.Duration:
		return devFormatDuration(val)
	case time.Time:
		return val.Format("15:04:05.000")
	case string:
		if val == "" {
			return `""`
		}
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

func devFormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
