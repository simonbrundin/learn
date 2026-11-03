// Package calibre läser Calibres bibliotek och listar dess böcker.
//
// Calibre håller biblioteket i en katalog (vanligen ~/calibre) med en
// SQLite-databas, metadata.db, som beskriver varje bok, dess format och var
// filerna ligger. Learn behöver bara veta vilka e-puber som finns och var
// de ligger på disk – biblioteket läses skrivskyddat så att Calibre kan
// köra samtidigt.
package calibre

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Format är Calibres interna formatnamn.
const (
	FormatEPUB         = "EPUB"
	FormatOriginalEPUB = "ORIGINAL_EPUB"
)

// extensions mappar Calibres formatnamn till filändelser.
var extensions = map[string]string{
	"EPUB":          ".epub",
	"ORIGINAL_EPUB": ".epub",
	"AZW3":          ".azw3",
	"AZW":           ".azw",
	"MOBI":          ".mobi",
	"PDF":           ".pdf",
	"FB2":           ".fb2",
	"TXT":           ".txt",
	"ZIP":           ".zip",
	"CBZ":           ".cbz",
	"CBR":           ".cbr",
	"LRF":           ".lrf",
	"HTMLZ":         ".htmlz",
	"DOCX":          ".docx",
}

// formatPriority avgör vilket format som används när en bok finns i flera.
// Kalibres e-pub är det normaliserade formatet och väljs före originalfilen.
var formatPriority = map[string]int{
	FormatEPUB:         0,
	FormatOriginalEPUB: 1,
	"AZW3":             2,
	"MOBI":             3,
	"AZW":              4,
	"PDF":              5,
	"FB2":              6,
	"TXT":              7,
	"ZIP":              8,
	"CBZ":              9,
	"CBR":              10,
	"LRF":              11,
	"HTMLZ":            12,
	"DOCX":             13,
}

// Book är en bok i Calibre-biblioteket.
type Book struct {
	ID      int64
	Title   string
	Authors []string
	Format  string
	Path    string
	Size    int64
	Added   time.Time
}

// Author ger bokens författare som en kommaseparerad sträng.
func (b Book) Author() string {
	return strings.Join(b.Authors, ", ")
}

// IsEPUB svarar på om boken finns som e-pub.
func (b Book) IsEPUB() bool {
	return b.Format == FormatEPUB || b.Format == FormatOriginalEPUB
}

// Library är ett öppnat Calibre-bibliotek.
type Library struct {
	dir string
	db  *sql.DB
}

// DefaultPath ger standardplatsen för Calibre-biblioteket.
// Miljövariabeln LEARN_CALIBRE_LIBRARY har företräde.
func DefaultPath() string {
	if override := os.Getenv("LEARN_CALIBRE_LIBRARY"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "calibre"
	}
	return filepath.Join(home, "calibre")
}

// Open öppnar biblioteket i katalogen dir.
func Open(dir string) (*Library, error) {
	if dir == "" {
		dir = DefaultPath()
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		absolute = dir
	}
	dbPath := filepath.Join(absolute, "metadata.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("hittade inte Calibres metadata.db i %s: %w", absolute, err)
	}

	// URI-formen gör att databasen öppnas skrivskyddat och att vi väntar
	// i stället för att låsa Calibre som skriver samtidigt.
	uri := url.URL{Scheme: "file", Path: dbPath}
	query := uri.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "busy_timeout(5000)")
	uri.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, fmt.Errorf("kunde inte öppna Calibres databas: %w", err)
	}
	return &Library{dir: absolute, db: db}, nil
}

// Close stänger biblioteket.
func (l *Library) Close() error { return l.db.Close() }

// Dir ger sökvägen till biblioteket.
func (l *Library) Dir() string { return l.dir }

// Books listar alla böcker i biblioteket. En bok med flera format visas en
// gång, med det format som har högst prioritet.
func (l *Library) Books() ([]Book, error) {
	rows, err := l.db.Query(`
		SELECT b.id, b.title, b.path, b.timestamp, d.format, d.name, d.uncompressed_size
		FROM books b
		JOIN data d ON d.book = b.id
		ORDER BY b.id`)
	if err != nil {
		return nil, fmt.Errorf("kunde inte läsa Calibres böcker: %w", err)
	}
	defer rows.Close()

	books := map[int64]*Book{}
	chosen := map[int64]string{}
	for rows.Next() {
		var (
			id      int64
			title   string
			relPath string
			added   any
			format  string
			name    string
			size    int64
		)
		if err := rows.Scan(&id, &title, &relPath, &added, &format, &name, &size); err != nil {
			return nil, fmt.Errorf("kunde inte läsa Calibres böcker: %w", err)
		}
		book, ok := books[id]
		if !ok {
			book = &Book{ID: id, Title: title, Added: toTime(added)}
			books[id] = book
		}
		if current, ok := chosen[id]; ok && priority(current) <= priority(format) {
			continue
		}
		chosen[id] = format
		book.Format = format
		book.Size = size
		book.Path = l.filePath(relPath, name, format)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kunde inte läsa Calibres böcker: %w", err)
	}

	list := make([]Book, 0, len(books))
	for _, book := range books {
		authors, err := l.authors(book.ID)
		if err != nil {
			return nil, err
		}
		book.Authors = authors
		list = append(list, *book)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, nil
}

// EPUBs listar de böcker som finns som e-pub och vars fil finns på disk.
func (l *Library) EPUBs() ([]Book, error) {
	books, err := l.Books()
	if err != nil {
		return nil, err
	}
	epubs := make([]Book, 0, len(books))
	for _, book := range books {
		if !book.IsEPUB() {
			continue
		}
		if _, err := os.Stat(book.Path); err != nil {
			continue
		}
		epubs = append(epubs, book)
	}
	return epubs, nil
}

// authors ger författarna till en bok i Calibres ordning. Saknas
// författartabellerna i biblioteket returneras ingen författare.
func (l *Library) authors(bookID int64) ([]string, error) {
	rows, err := l.db.Query(`
		SELECT a.name
		FROM books_authors_link l
		JOIN authors a ON a.id = l.author
		WHERE l.book = ?
		ORDER BY l.id`, bookID)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	var authors []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("kunde inte läsa författare: %w", err)
		}
		authors = append(authors, name)
	}
	return authors, rows.Err()
}

// toTime tolkar Calibres tidsstämpel, som antingen är ett tal (sekunder
// sedan epoch) eller en datumsträng.
func toTime(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case float64:
		return time.Unix(int64(typed), 0)
	case int64:
		return time.Unix(typed, 0)
	case string:
		for _, layout := range []string{"2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05", time.RFC3339} {
			if parsed, err := time.Parse(layout, typed); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// filePath bygger sökvägen till en bokfil i biblioteket.
func (l *Library) filePath(relPath, name, format string) string {
	extension, ok := extensions[format]
	if !ok {
		extension = ""
	}
	return filepath.Join(l.dir, relPath, name+extension)
}

// priority ger ett formatets prioritet. Okända format sorteras sist.
func priority(format string) int {
	if value, ok := formatPriority[format]; ok {
		return value
	}
	return len(formatPriority)
}
