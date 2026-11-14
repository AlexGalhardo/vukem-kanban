// Package resources embeds the views and the static files.
package resources

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
)

//go:embed all:views
var views embed.FS

//go:embed all:public
var public embed.FS

// Views returns the html/template views (resources/views).
func Views() fs.FS {
	sub, err := fs.Sub(views, "views")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}

// Public returns the static files served at the site root (resources/public).
func Public() fs.FS {
	sub, err := fs.Sub(public, "public")
	if err != nil {
		panic(err) // the directory is embedded at build time
	}
	return sub
}

// AssetHash fingerprints the static files. Asset URLs carry it as ?v=, so
// any change to CSS or JS busts the immutable browser cache even when the
// app version string did not change (dev builds, rebuilt releases).
func AssetHash() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(public, "public/static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := public.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(path))
		_, _ = h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12], err
}
