package classify

import "testing"

func TestCategoryFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "documents", path: "notes/report.PDF", want: CategoryDocuments},
		{name: "pictures", path: "images/family.JpEg", want: CategoryPictures},
		{name: "videos", path: "videos/clip.MKV", want: CategoryVideos},
		{name: "music", path: "audio/song.MP3", want: CategoryMusic},
		{name: "unknown extension", path: "misc/archive.bin", want: CategoryOther},
		{name: "no extension", path: "README", want: CategoryOther},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CategoryFor(tc.path)
			if got != tc.want {
				t.Fatalf("CategoryFor(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
