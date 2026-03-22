package filter

import "testing"

func TestIsLikelyProgram(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "blocked extension lower", path: "/backup/tools/setup.exe", want: true},
		{name: "blocked extension upper", path: "/backup/tools/INSTALLER.MSI", want: true},
		{name: "blocked folder downloads", path: "/mnt/backup/Downloads/photo.jpg", want: true},
		{name: "blocked folder steamapps", path: "/mnt/backup/games/steamapps/appmanifest.acf", want: true},
		{name: "personal image", path: "/mnt/backup/family/photo.jpg", want: false},
		{name: "personal document", path: "/mnt/backup/docs/resume.pdf", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsLikelyProgram(tc.path)
			if got != tc.want {
				t.Fatalf("IsLikelyProgram(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
