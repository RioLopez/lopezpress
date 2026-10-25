package config

import "testing"

// R2-04: the startup backfill recorded every file with an image extension,
// including the thumbnail worker's own <id>_t.<ext> copies. The reported run
// inserted 63 owner-less rows — about half the admin gallery, none attributable
// to an author, and all of them inflating the site's used-bytes figure. A
// derived file is not an upload.
func TestBackfillSkipsGeneratedThumbnails(t *testing.T) {
	// The real naming scheme is id + "_t" + ext (see services.buildThumb).
	derived := []string{
		"9f2c1b_t.jpg",
		"9f2c1b_t.webp",
		"abc123_t.png",
		"9f2c1b_t.PNG", // extension case must not matter
	}
	for _, name := range derived {
		if !isDerivedImageName(name) {
			t.Errorf("%q should be recognised as a generated thumbnail", name)
		}
	}

	ordinary := []string{
		"abc123.jpg",
		"my_trip_photos.png", // contains _t but does not end with it
		"chart_t.png.txt",    // not an image at all
		"",
	}
	for _, name := range ordinary {
		if isDerivedImageName(name) {
			t.Errorf("%q must not be treated as a generated thumbnail", name)
		}
	}
}
