package datadir

import "os"

// Get returns the writable runtime data directory.
// Priority matches setup.GetDataDir without importing setup and creating cycles.
func Get() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}

	dockerDataDir := "/app/data"
	if info, err := os.Stat(dockerDataDir); err == nil && info.IsDir() {
		testFile := dockerDataDir + "/.write_test"
		if f, err := os.Create(testFile); err == nil {
			_ = f.Close()
			_ = os.Remove(testFile)
			return dockerDataDir
		}
	}

	return "."
}
