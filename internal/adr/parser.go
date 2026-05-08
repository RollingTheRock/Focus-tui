package adr

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"focus/internal/models"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var (
	adrTitlePattern = regexp.MustCompile(`^ADR-(\d{4}):\s*(.+)$`)
	adrFilePattern  = regexp.MustCompile(`^\d{4}-`)
	datePattern     = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
)

// LoadAll reads all ADR markdown files from dir and returns parsed ADR records
// sorted by ID. Files whose names do not start with 4 digits followed by a hyphen
// or whose first H1 does not match "# ADR-NNNN: Title" are skipped silently.
func LoadAll(dir string) ([]models.ADRRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var records []models.ADRRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if !adrFilePattern.MatchString(entry.Name()) {
			continue
		}
		record, err := parseADRFile(filepath.Join(dir, entry.Name()))
		if err != nil || record == nil {
			continue
		}
		records = append(records, *record)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records, nil
}

// LoadConstraints reads a single ADR markdown file and extracts constraint
// records from the "## Constraints" section. Returns nil, nil if the file
// does not exist or has no constraints section.
func LoadConstraints(dir, adrID string) ([]models.ADRConstraintRecord, error) {
	// adrID is like "ADR-0000", filename is "0000-*.md"
	numeric := strings.TrimPrefix(adrID, "ADR-")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), numeric+"-") {
			continue
		}

		source, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		return parseConstraints(source)
	}
	return nil, nil
}

// ---------- internal helpers ----------

func parseADRFile(path string) (*models.ADRRecord, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	var record models.ADRRecord
	record.FilePath = path
	record.Version = 1

	sections := make(map[string]string)
	var currentSection string
	var sectionLines []string

	flushSection := func() {
		if currentSection != "" && len(sectionLines) > 0 {
			sections[currentSection] = strings.TrimSpace(strings.Join(sectionLines, "\n"))
			sectionLines = nil
		}
	}

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *ast.Heading:
			headingText := extractInline(source, n)
			if n.Level == 1 {
				matches := adrTitlePattern.FindStringSubmatch(headingText)
				if matches == nil {
					return nil, nil
				}
				record.ID = "ADR-" + matches[1]
				record.Title = matches[2]
			} else if n.Level == 2 {
				flushSection()
				currentSection = strings.ToLower(strings.TrimSpace(headingText))
			} else {
				// H3+ is part of current section content
				if currentSection != "" {
					sectionLines = append(sectionLines, headingText)
				}
			}
		case *ast.ThematicBreak:
			// skip
		default:
			if currentSection != "" {
				text := blockText(source, child)
				if text != "" {
					sectionLines = append(sectionLines, text)
				}
			}
		}
	}
	flushSection()

	record.Context = sections["context"]
	record.Decision = sections["decision"]
	record.Consequences = sections["consequences"]

	parseMetadata(doc, source, &record, sections)

	return &record, nil
}

func parseMetadata(doc ast.Node, source []byte, record *models.ADRRecord, sections map[string]string) {
	// First pass: look for metadata list (standard ADR format)
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		if list, ok := child.(*ast.List); ok {
			if isMetadataList(list, source) {
				for li := list.FirstChild(); li != nil; li = li.NextSibling() {
					if item, ok := li.(*ast.ListItem); ok {
						key, value := extractKV(item, source)
						if key != "" {
							applyMeta(key, value, record)
						}
					}
				}
				break // only first metadata-like list
			}
		}
	}

	// Second pass: fallback for "## Status" h2 style (ADR-0003)
	if record.Status == "" {
		if statusText, ok := sections["status"]; ok {
			record.Status = parseStatusFromText(statusText)
			if record.Date == "" {
				record.Date = datePattern.FindString(statusText)
			}
		}
	}
}

func isMetadataList(list *ast.List, source []byte) bool {
	first := list.FirstChild()
	if first == nil {
		return false
	}
	li, ok := first.(*ast.ListItem)
	if !ok {
		return false
	}
	key, _ := extractKV(li, source)
	return key != ""
}

func extractKV(li *ast.ListItem, source []byte) (key, value string) {
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		if !child.HasChildren() {
			continue
		}
		firstChild := child.FirstChild()
		if firstChild == nil {
			continue
		}
		em, ok := firstChild.(*ast.Emphasis)
		if !ok || em.Level != 2 {
			continue
		}

		key = strings.TrimRight(extractInline(source, em), ":： ")

		var valueBuf bytes.Buffer
		for c := firstChild.NextSibling(); c != nil; c = c.NextSibling() {
			writeInline(source, c, &valueBuf)
		}
		value = strings.TrimSpace(valueBuf.String())
		return key, value
	}
	return "", ""
}

func applyMeta(key, value string, record *models.ADRRecord) {
	switch strings.ToLower(key) {
	case "date":
		record.Date = value
	case "status":
		record.Status = strings.ToLower(value)
	case "supersedes":
		record.SupersededBy = parseSupersedes(value)
	}
}

func parseStatusFromText(text string) string {
	lower := strings.ToLower(text)
	firstLine := strings.SplitN(lower, "\n", 2)[0]
	for _, status := range []string{"accepted", "proposed", "deprecated", "superseded"} {
		if strings.Contains(firstLine, status) {
			return status
		}
	}
	return strings.ToLower(strings.Fields(firstLine)[0])
}

func parseSupersedes(value string) *string {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil
	}
	// Take the first ADR ID (comma-separated list)
	first := strings.SplitN(s, ",", 2)[0]
	first = strings.TrimSpace(first)
	return &first
}

func blockText(source []byte, node ast.Node) string {
	switch n := node.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return extractInline(source, node)
	case *ast.List:
		var lines []string
		for li := n.FirstChild(); li != nil; li = li.NextSibling() {
			if item, ok := li.(*ast.ListItem); ok {
				text := extractInline(source, item)
				if text != "" {
					lines = append(lines, "• "+text)
				}
			}
		}
		return strings.Join(lines, "\n")
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		var lines []string
		for i := 0; i < node.Lines().Len(); i++ {
			line := node.Lines().At(i)
			lines = append(lines, "  "+string(line.Value(source)))
		}
		return strings.Join(lines, "\n")
	case *ast.Heading:
		return extractInline(source, node)
	case *ast.ThematicBreak:
		return ""
	default:
		return extractInline(source, node)
	}
}

func extractInline(source []byte, node ast.Node) string {
	var buf bytes.Buffer
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		writeInline(source, child, &buf)
	}
	return strings.TrimSpace(buf.String())
}

func writeInline(source []byte, node ast.Node, buf *bytes.Buffer) {
	switch n := node.(type) {
	case *ast.Text:
		buf.Write(n.Segment.Value(source))
	case *ast.String:
		buf.Write(n.Value)
	case *ast.Emphasis:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			writeInline(source, child, buf)
		}
	case *ast.Link:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			writeInline(source, child, buf)
		}
	case *ast.CodeSpan:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			writeInline(source, child, buf)
		}
	case *ast.Image:
		// skip
	case *ast.RawHTML:
		// skip
	}
}

func parseConstraints(source []byte) ([]models.ADRConstraintRecord, error) {
	md := goldmark.New()
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	var inConstraints bool
	var records []models.ADRConstraintRecord

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		h, ok := child.(*ast.Heading)
		if !ok {
			if inConstraints {
				if list, ok := child.(*ast.List); ok {
					for li := list.FirstChild(); li != nil; li = li.NextSibling() {
						if item, ok := li.(*ast.ListItem); ok {
							if rec := parseConstraintItem(source, item); rec != nil {
								records = append(records, *rec)
							}
						}
					}
				}
				// Only parse the first list in the constraints section
				break
			}
			continue
		}

		if h.Level == 2 {
			headingText := strings.ToLower(strings.TrimSpace(extractInline(source, h)))
			inConstraints = headingText == "constraints"
			continue
		}
		if inConstraints {
			// H3+ inside constraints, stop the list search
			break
		}
	}

	return records, nil
}

func parseConstraintItem(source []byte, li *ast.ListItem) *models.ADRConstraintRecord {
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		if !child.HasChildren() {
			continue
		}
		firstChild := child.FirstChild()
		if firstChild == nil {
			continue
		}
		em, ok := firstChild.(*ast.Emphasis)
		if !ok || em.Level < 1 {
			continue
		}

		category := strings.TrimRight(extractInline(source, em), ":： ")

		var restBuf bytes.Buffer
		for c := firstChild.NextSibling(); c != nil; c = c.NextSibling() {
			writeInline(source, c, &restBuf)
		}
		rest := strings.TrimSpace(restBuf.String())
		rest = strings.TrimLeft(rest, ":： ")

		rule, rationale := splitRuleRationale(rest)
		return &models.ADRConstraintRecord{
			Category:  strings.ToLower(category),
			Rule:      rule,
			Rationale: rationale,
		}
	}
	return nil
}

func splitRuleRationale(text string) (rule, rationale string) {
	for _, sep := range []string{" -- ", " — ", " - "} {
		if idx := strings.Index(text, sep); idx >= 0 {
			return strings.TrimSpace(text[:idx]), strings.TrimSpace(text[idx+len(sep):])
		}
	}
	return text, ""
}
