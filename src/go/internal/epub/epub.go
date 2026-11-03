// Package epub parsar e-pub-filer utan externa beroenden.
//
// Filen läses som ett zip-arkiv: META-INF/container.xml pekar ut OPF-filen,
// som innehåller manifest, spine och (valfritt) en innehållsförteckning.
package epub

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"golang.org/x/net/html"

	"github.com/simon/repo-template/internal/model"
)

// ErrInvalidEpub returneras när filen inte är en läsbar e-pub.
var ErrInvalidEpub = errors.New("filen är inte en giltig e-pub")

// Metadata beskriver bokens metadata.
type Metadata struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	Language    string `json:"language"`
	Description string `json:"description"`
	Identifier  string `json:"identifier"`
}

// Book är resultatet av en e-pub-parsning.
type Book struct {
	Metadata Metadata
	Episodes []model.Episode
}

// Title ger bokens titel.
func (b *Book) Title() string { return b.Metadata.Title }

// Author ger bokens författare.
func (b *Book) Author() string { return b.Metadata.Author }

type containerXML struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfPackage struct {
	Metadata struct {
		Title       string `xml:"title"`
		Creator     string `xml:"creator"`
		Language    string `xml:"language"`
		Description string `xml:"description"`
		Identifier  string `xml:"identifier"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		ItemRefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type ncxDocument struct {
	NavMap struct {
		NavPoints []struct {
			NavLabel struct {
				Text string `xml:"text"`
			} `xml:"navLabel"`
			Content struct {
				Src string `xml:"src,attr"`
			} `xml:"content"`
		} `xml:"navPoint"`
	} `xml:"navMap"`
}

// Parse läser in en e-pub-fil och returnerar metadata och kapitel.
func Parse(filePath string) (*Book, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("filen finns inte: %w", err)
	}
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEpub, err)
	}
	defer reader.Close()
	_ = info

	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}

	opfPath, err := findOPFPath(files)
	if err != nil {
		return nil, err
	}
	opfData, err := readFile(files, opfPath)
	if err != nil {
		return nil, err
	}
	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return nil, fmt.Errorf("kunde inte tolka OPF-filen: %w", err)
	}

	metadata := Metadata{
		Title:       strings.TrimSpace(pkg.Metadata.Title),
		Author:      strings.TrimSpace(pkg.Metadata.Creator),
		Language:    strings.TrimSpace(pkg.Metadata.Language),
		Description: strings.TrimSpace(pkg.Metadata.Description),
		Identifier:  strings.TrimSpace(pkg.Metadata.Identifier),
	}
	if metadata.Language == "" {
		metadata.Language = "en"
	}

	manifest := map[string]manifestItem{}
	for _, item := range pkg.Manifest.Items {
		manifest[item.ID] = manifestItem{
			href:       item.Href,
			mediaType:  item.MediaType,
			properties: item.Properties,
		}
	}

	base := ""
	if idx := strings.LastIndex(opfPath, "/"); idx >= 0 {
		base = opfPath[:idx]
	}

	tocTitles := parseTOC(files, base, manifest)

	var episodes []model.Episode
	for _, ref := range pkg.Spine.ItemRefs {
		item, ok := manifest[ref.IDRef]
		if !ok {
			continue
		}
		if item.mediaType != "" && !isXHTML(item.mediaType) {
			continue
		}
		member := joinPath(base, item.href)
		data, err := readFile(files, member)
		if err != nil {
			continue
		}
		headings, blocks := extractHTML(string(data))
		if len(blocks) == 0 && len(headings) == 0 {
			continue
		}
		title := tocTitles[member]
		if title == "" && len(headings) > 0 {
			title = headings[0]
		}
		if title == "" {
			title = fmt.Sprintf("Avsnitt %d", len(episodes)+1)
		}
		episodes = append(episodes, model.Episode{
			Index:  len(episodes),
			Title:  title,
			Text:   strings.Join(blocks, "\n\n"),
			Blocks: blocks,
		})
	}

	if len(episodes) == 0 {
		return nil, fmt.Errorf("%w: ingen text hittades", ErrInvalidEpub)
	}
	return &Book{Metadata: metadata, Episodes: episodes}, nil
}

type manifestItem struct {
	href       string
	mediaType  string
	properties string
}

func isXHTML(mediaType string) bool {
	switch mediaType {
	case "application/xhtml+xml", "text/html", "application/xml", "text/xml":
		return true
	}
	return false
}

func findOPFPath(files map[string]*zip.File) (string, error) {
	if container, ok := files["META-INF/container.xml"]; ok {
		data, err := readFile(files, container.Name)
		if err == nil {
			var doc containerXML
			if err := xml.Unmarshal(data, &doc); err == nil {
				for _, rootfile := range doc.Rootfiles {
					if rootfile.FullPath != "" {
						return rootfile.FullPath, nil
					}
				}
			}
		}
	}
	var candidates []string
	for name := range files {
		if strings.HasSuffix(strings.ToLower(name), ".opf") {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: kunde inte hitta OPF-filen", ErrInvalidEpub)
	}
	sortStrings(candidates)
	return candidates[0], nil
}

func parseTOC(files map[string]*zip.File, base string, manifest map[string]manifestItem) map[string]string {
	titles := map[string]string{}

	for _, item := range manifest {
		if item.mediaType != "application/x-dtbncx+xml" {
			continue
		}
		data, err := readFile(files, joinPath(base, item.href))
		if err != nil {
			continue
		}
		var doc ncxDocument
		if err := xml.Unmarshal(data, &doc); err != nil {
			continue
		}
		for _, point := range doc.NavMap.NavPoints {
			if point.Content.Src == "" {
				continue
			}
			member := strings.Split(point.Content.Src, "#")[0]
			if _, exists := titles[member]; !exists {
				titles[member] = strings.TrimSpace(point.NavLabel.Text)
			}
		}
		break
	}

	for _, item := range manifest {
		if !strings.Contains(item.properties, "nav") {
			continue
		}
		data, err := readFile(files, joinPath(base, item.href))
		if err != nil {
			continue
		}
		headings, _ := extractHTML(string(data))
		for _, heading := range headings {
			if _, exists := titles[item.href]; !exists {
				titles[item.href] = heading
			}
		}
	}
	return titles
}

func readFile(files map[string]*zip.File, name string) ([]byte, error) {
	file, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s saknas", ErrInvalidEpub, name)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func joinPath(base, href string) string {
	if base == "" {
		return href
	}
	return path.Join(base, href)
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// ---------------------------------------------------------------------------
// HTML-textextraktion
// ---------------------------------------------------------------------------

var (
	skipTags = map[string]bool{
		"script": true, "style": true, "head": true, "title": true,
		"svg": true, "nav": true, "meta": true, "link": true,
	}
	headingTags = map[string]bool{
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	}
	leafBlockTags = map[string]bool{
		"p": true, "blockquote": true, "figcaption": true, "dd": true,
		"dt": true, "pre": true, "address": true, "caption": true,
	}
)

func extractHTML(source string) ([]string, []string) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, nil
	}
	var headings, blocks []string
	walkHTML(doc, &headings, &blocks)
	return headings, blocks
}

func walkHTML(node *html.Node, headings, blocks *[]string) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		tag := strings.ToLower(child.Data)
		if skipTags[tag] {
			continue
		}
		switch {
		case headingTags[tag]:
			if text := elementText(child); text != "" {
				*headings = append(*headings, text)
			}
		case leafBlockTags[tag]:
			if text := elementText(child); text != "" {
				*blocks = append(*blocks, text)
			}
		case hasBlockDescendant(child):
			walkHTML(child, headings, blocks)
		default:
			if text := elementText(child); text != "" {
				*blocks = append(*blocks, text)
			}
		}
	}
}

func hasBlockDescendant(node *html.Node) bool {
	found := false
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode {
				continue
			}
			tag := strings.ToLower(child.Data)
			if leafBlockTags[tag] || headingTags[tag] {
				found = true
				return
			}
			walk(child)
			if found {
				return
			}
		}
	}
	walk(node)
	return found
}

func elementText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			switch child.Type {
			case html.TextNode:
				builder.WriteString(child.Data)
			case html.ElementNode:
				if skipTags[strings.ToLower(child.Data)] {
					continue
				}
				walk(child)
				builder.WriteString(" ")
			}
		}
	}
	walk(node)
	return strings.TrimSpace(strings.Join(strings.Fields(builder.String()), " "))
}
