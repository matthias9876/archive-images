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
		// Music folder heuristics: pictures in music folders should be Other
		{name: "album art in music folder", path: "Shared Music/predigt/AlbumArtSmall.jpg", want: CategoryOther},
		{name: "cover in music subfolder", path: "/home/user/Music/classical/cover.png", want: CategoryOther},
		{name: "artwork in audio folder", path: "backup/Audio/jazz/artwork.jpeg", want: CategoryOther},
		{name: "picture in non-music folder", path: "Shared/Other/photo.jpg", want: CategoryPictures},
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
