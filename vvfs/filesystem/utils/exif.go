package utils

import (
	"os"

	exiflib "github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

// ExtractEXIF returns a flat map of EXIF tag names to their string values.
// On any error (non-image, missing EXIF, read failure) it returns nil.
func ExtractEXIF(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	x, err := exiflib.Decode(f)
	if err != nil {
		return nil
	}
	out := make(map[string]string)
	_ = x.Walk(exifWalker{m: out})
	if len(out) == 0 {
		return nil
	}
	return out
}

type exifWalker struct{ m map[string]string }

func (w exifWalker) Walk(name exiflib.FieldName, tag *tiff.Tag) error {
	w.m[string(name)] = tag.String()
	return nil
}
