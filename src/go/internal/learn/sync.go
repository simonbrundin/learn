package learn

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/simon/repo-template/internal/calibre"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
)

// SyncOptions styr hur en bibliotekssynk går till.
type SyncOptions struct {
	// MaxConcepts begränsar antalet koncept per bok.
	MaxConcepts int
	// Refresh importerar om även böcker som redan finns i databasen.
	Refresh bool
	// Limit begränsar antalet böcker som importeras i en körning.
	Limit int
	// MaxFileSize hoppar över böcker vars fil är större än gränsen.
	// Skannade böcker utan text kan vara flera gigabyte stora och
	// ger inget kunskapsvärde.
	MaxFileSize int64
	// DryRun rapporterar vad som skulle importeras utan att skriva.
	DryRun bool
}

// SyncResult sammanfattar vad en synk gjorde.
type SyncResult struct {
	// Imported är de böcker som lästes in.
	Imported []ImportResult
	// Updated är böcker som importerades om.
	Updated []ImportResult
	// Skipped är böcker som redan fanns och inte ändrats.
	Skipped int
	// Waiting är böcker som misslyckades tidigare och inte ändrats sedan.
	Waiting int
	// TooLarge är böcker som hoppades över för att filen var för stor.
	TooLarge int
	// Failed är böcker som inte kunde importeras.
	Failed []error
}

// Total ger det totala antalet böcker som lästes in.
func (r *SyncResult) Total() int {
	return len(r.Imported) + len(r.Updated)
}

// Summary ger en kort sammanfattning av synken.
func (r *SyncResult) Summary() string {
	text := fmt.Sprintf("%d nya, %d uppdaterade, %d redan synkade",
		len(r.Imported), len(r.Updated), r.Skipped)
	if r.Waiting > 0 {
		text += fmt.Sprintf(", %d väntar på fix", r.Waiting)
	}
	if r.TooLarge > 0 {
		text += fmt.Sprintf(", %d för stora filer", r.TooLarge)
	}
	if len(r.Failed) > 0 {
		text += fmt.Sprintf(", %d misslyckade", len(r.Failed))
	}
	return text
}

// SyncCalibre importerar de e-puber i ett Calibre-bibliotek som saknas i
// kunskapsbasen. Böcker som redan importerats hoppas över, om inte
// filen ändrats sedan importen eller Refresh är satt.
func SyncCalibre(lib *calibre.Library, repo store.Repository, opts SyncOptions) (*SyncResult, error) {
	books, err := lib.EPUBs()
	if err != nil {
		return nil, err
	}

	existing, err := repo.ListBooks()
	if err != nil {
		return nil, err
	}
	byPath := map[string]model.Book{}
	for _, book := range existing {
		byPath[filepath.Clean(book.SourcePath)] = book
	}
	failures, err := repo.Failures()
	if err != nil {
		return nil, err
	}
	failedAt := map[string]model.ImportFailure{}
	for _, failure := range failures {
		failedAt[filepath.Clean(failure.SourcePath)] = failure
	}

	result := &SyncResult{}
	for _, candidate := range books {
		if opts.Limit > 0 && result.Total() >= opts.Limit {
			break
		}
		known, ok := byPath[filepath.Clean(candidate.Path)]
		if opts.MaxFileSize > 0 {
			if size, err := fileSize(candidate.Path); err == nil && size > opts.MaxFileSize {
				result.TooLarge++
				continue
			}
		}
		// En bok som redan misslyckades prövas inte igen förrän filen ändras.
		if failure, failed := failedAt[filepath.Clean(candidate.Path)]; failed && !ok {
			if !changedSince(candidate.Path, failure.FailedAt) {
				result.Waiting++
				continue
			}
		}
		if ok && !opts.Refresh && !changedSince(candidate.Path, known.ImportedAt) {
			result.Skipped++
			continue
		}
		if opts.DryRun {
			if ok {
				result.Updated = append(result.Updated, ImportResult{Book: model.Book{
					ID:         known.ID,
					Title:      candidate.Title,
					Author:     candidate.Author(),
					SourcePath: candidate.Path,
				}})
			} else {
				result.Imported = append(result.Imported, ImportResult{Book: model.Book{
					Title:      candidate.Title,
					Author:     candidate.Author(),
					SourcePath: candidate.Path,
				}})
			}
			continue
		}
		if ok {
			if err := repo.DeleteBook(known.ID); err != nil {
				result.Failed = append(result.Failed, fmt.Errorf("%s: %w", candidate.Title, err))
				continue
			}
		}
		imported, err := ImportBook(candidate.Path, repo, opts.MaxConcepts)
		if err != nil {
			result.Failed = append(result.Failed, fmt.Errorf("%s: %w", candidate.Title, err))
			if recordErr := repo.RecordFailure(model.ImportFailure{
				SourcePath: candidate.Path,
				Title:      candidate.Title,
				Reason:     err.Error(),
				FailedAt:   time.Now(),
			}); recordErr != nil {
				result.Failed = append(result.Failed, recordErr)
			}
			continue
		}
		if err := repo.ClearFailure(candidate.Path); err != nil {
			result.Failed = append(result.Failed, err)
		}
		if ok {
			result.Updated = append(result.Updated, *imported)
		} else {
			result.Imported = append(result.Imported, *imported)
		}
	}
	return result, nil
}

// fileSize ger filens storlek i byte.
func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// changedSince svarar på om filen ändrats efter att boken importerades.
func changedSince(path string, importedAt time.Time) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.ModTime().After(importedAt)
}
