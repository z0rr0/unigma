package conf

import (
	"log"
	"os"
	"testing"
)

const (
	testConfig = "/tmp/unigma.json"
)

var (
	loggerInfo = log.New(os.Stdout, "[TEST]", log.Ltime|log.Lshortfile)
)

func TestNew(t *testing.T) {
	if _, err := New("/bad_file_path.json", loggerInfo); err == nil {
		t.Error("unexpected behavior")
	}
	cfg, err := New(testConfig, loggerInfo)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr() == "" {
		t.Error("empty address")
	}
	cfg.Settings.Size = 4
	if m := cfg.MaxFileSize(); m != (1048576 * 4) {
		t.Error(m)
	}
	err = cfg.Close()
	if err != nil {
		t.Errorf("close error: %v", err)
	}
}
