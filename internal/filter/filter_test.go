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
		{name: "wiso steuer is never treated as program", path: "/mnt/backup/Program Files/Wiso Steuer/setup.exe", want: false},
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

func TestShouldSkipDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "recycle bin", path: "/mnt/win/$Recycle.Bin/S-1-5-21", want: true},
		{name: "windows folder", path: "/mnt/win/Windows/System32", want: true},
		{name: "windows upgrade staging", path: "/mnt/win/$Windows.~WS/Sources", want: true},
		{name: "appdata roaming", path: "/mnt/win/Users/matth/AppData/Roaming", want: true},
		{name: "appdata local", path: "/mnt/win/Users/matth/AppData/Local", want: true},
		{name: "program files", path: "/mnt/win/Program Files/Adobe", want: true},
		{name: "program files x86", path: "/mnt/win/Program Files (x86)/Common Files", want: true},
		{name: "programdata", path: "/mnt/win/ProgramData/Microsoft", want: true},
		{name: "onedrivetemp", path: "/mnt/win/OneDriveTemp/S-1-5-21", want: true},
		{name: "cache", path: "/mnt/win/Users/alice/cache", want: true},
		{name: "dot cache linux", path: "/mnt/win/.cache/AMD", want: true},
		{name: "xboxgames", path: "/mnt/win/XboxGames/GameSave", want: true},
		{name: "amd chipset root", path: "/mnt/win/AMD/Chipset_Driver_Installer", want: true},
		{name: "perflogs", path: "/mnt/win/PerfLogs", want: true},
		{name: "config.msi", path: "/mnt/win/Config.Msi", want: true},
		{name: "wiso steuer override", path: "/mnt/win/Program Files/Wiso Steuer", want: false},
		{name: "normal user documents folder", path: "/mnt/win/Users/matth/Documents", want: false},
		{name: "personal pictures folder", path: "/mnt/win/Users/matth/Pictures", want: false},
		{name: "personal music folder", path: "/mnt/win/Users/matth/Music", want: false},
		{name: "personal videos folder", path: "/mnt/win/Users/matth/Videos", want: false},
		{name: "onedrive personal data", path: "/mnt/win/Users/matth/OneDrive/Bilder", want: false},
		// User-profile noise
		{name: "espressif toolchain", path: "/mnt/win/Users/matth/.espressif/tools", want: true},
		{name: "vscode extensions", path: "/mnt/win/Users/matth/.vscode/extensions", want: true},
		{name: "lmstudio models", path: "/mnt/win/Users/matth/.lmstudio/models", want: true},
		{name: "ollama models", path: "/mnt/win/Users/matth/.ollama/models", want: true},
		{name: "virtualbox vms", path: "/mnt/win/Users/matth/VirtualBox VMs/ubuntu", want: true},
		{name: "virtualbox config", path: "/mnt/win/Users/matth/.VirtualBox", want: true},
		{name: "thumbnails cache", path: "/mnt/win/Users/matth/.thumbnails/large", want: true},
		{name: "saved games", path: "/mnt/win/Users/matth/Saved Games", want: true},
		{name: "searches", path: "/mnt/win/Users/matth/Searches", want: true},
		{name: "contacts", path: "/mnt/win/Users/matth/Contacts", want: true},
		{name: "favorites", path: "/mnt/win/Users/matth/Favorites/Links", want: true},
		{name: "oracle jre telemetry", path: "/mnt/win/Users/matth/.oracle_jre_usage", want: true},
		{name: "ms account data", path: "/mnt/win/Users/matth/.ms-ad", want: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldSkipDirectory(tc.path)
			if got != tc.want {
				t.Fatalf("ShouldSkipDirectory(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
