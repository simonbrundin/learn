// Package testutil innehåller hjälpare som används av testerna.
package testutil

import (
	"archive/zip"
	"os"
	"path/filepath"
)

const containerXML = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

const opf = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">urn:uuid:test-book</dc:identifier>
    <dc:title>Thinking About Thinking</dc:title>
    <dc:creator>Test Author</dc:creator>
    <dc:language>en</dc:language>
    <dc:description>En liten testbok om tänkande.</dc:description>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ch1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`

const ncx = `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <navMap>
    <navPoint id="np1" playOrder="1">
      <navLabel><text>Fast Thinking</text></navLabel>
      <content src="chapter1.xhtml"/>
    </navPoint>
    <navPoint id="np2" playOrder="2">
      <navLabel><text>Slow Thinking</text></navLabel>
      <content src="chapter2.xhtml"/>
    </navPoint>
  </navMap>
</ncx>`

const nav = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
<nav><ol><li><a href="chapter1.xhtml">Fast Thinking</a></li></ol></nav>
</body></html>`

const chapter1 = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Fast Thinking</title></head><body>
<h1>Fast Thinking</h1>
<p>System 1 is the fast, automatic and emotional thinking that runs without effort.</p>
<p>System 1 answers simple questions immediately. It is the reason you can read
a familiar word without noticing the letters.</p>
<p>The key insight is that System 1 feels certain even when it is wrong.</p>
<p>Unlike System 2, System 1 cannot be switched off at will.</p>
</body></html>`

const chapter2 = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Slow Thinking</title></head><body>
<h1>Slow Thinking</h1>
<p>System 2 is the slow, deliberate and analytical thinking that requires attention.</p>
<p>System 2 is effortful. It is needed whenever you must hold information in mind
and compare it with what you already know.</p>
<p>Attention is the scarce resource for System 2. Because attention is limited,
we prefer the easy answer, and this leads to biased decisions.</p>
<p>Whereas System 1 is automatic, System 2 is deliberate.</p>
</body></html>`

// BuildEpub skriver en minimal e-pub och returnerar sökvägen till den.
func BuildEpub(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "thinking.epub")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	files := []struct {
		name    string
		content string
	}{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", containerXML},
		{"OEBPS/content.opf", opf},
		{"OEBPS/toc.ncx", ncx},
		{"OEBPS/nav.xhtml", nav},
		{"OEBPS/chapter1.xhtml", chapter1},
		{"OEBPS/chapter2.xhtml", chapter2},
	}
	for _, item := range files {
		entry, err := writer.Create(item.name)
		if err != nil {
			return "", err
		}
		if _, err := entry.Write([]byte(item.content)); err != nil {
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return path, nil
}
