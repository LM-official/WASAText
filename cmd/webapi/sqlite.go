package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// prepareSQLiteDSN creates the database's parent directory and adds the connection options required by the API
// A file: URI must be decoded before touching the filesystem
func prepareSQLiteDSN(filename string) (string, error) {
	parts := strings.SplitN(filename, "?", 2)
	path := parts[0]
	query := ""
	if len(parts) == 2 {
		query = parts[1]
	}

	var uri *url.URL
	if strings.HasPrefix(filename, "file:") {
		var err error
		uri, err = url.Parse(filename)
		if err != nil {
			return "", fmt.Errorf("parsing SQLite URI: %w", err)
		}
		path = uri.Path
		// Relative file URIs are parsed as opaque URLs by net/url.
		if uri.Opaque != "" {
			path, err = url.PathUnescape(uri.Opaque)
			if err != nil {
				return "", fmt.Errorf("decoding SQLite path: %w", err)
			}
		}
		query = uri.RawQuery
		uri.Fragment = ""
	}

	params, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("parsing SQLite options: %w", err)
	}
	// SQLite creates the file, but not its parent directories
	// Memory and temporary databases have no disk path to prepare
	if path != "" && path != ":memory:" && !(uri != nil && params.Get("mode") == "memory") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("creating SQLite directory for %q: %w", path, err)
		}
	}

	// Apply foreign keys to every pooled connection,
	// and acquire transaction write locks immediately so concurrent read-then-write transactions do not deadlock
	params.Del("_fk") // Alias that would otherwise override _foreign_keys in the driver
	params.Set("_foreign_keys", "on")
	params.Set("_txlock", "immediate")
	if uri != nil {
		uri.RawQuery = params.Encode()
		return uri.String(), nil
	}
	return parts[0] + "?" + params.Encode(), nil
}
