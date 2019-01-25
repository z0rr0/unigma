package conf

import (
	"log"
	"os"
	"testing"

	"github.com/z0rr0/unigma/db"
)

const (
	testDB = "/tmp/unigma_db.sqlite"
	testConfigName = "/tmp/config.example.json"
)

var (

	loggerInfo     = log.New(os.Stdout, "[TEST]", log.Ltime|log.Lshortfile)
)

func TestNew(t *testing.T) {
	err := db.CreateDB("/tmp/db.sqlite", "/tmp/schema.sql")
	if err != nil {
		t.Error(err)
	}


	//if _, err := New("/bad_file_path.json", loggerInfo); err == nil {
	//	t.Error("unexpected behavior")
	//}
	//cfg, err := New(testConfigName, loggerInfo)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//if cfg.Addr() == "" {
	//	t.Error("empty address")
	//}
	//err = cfg.Close()
	//if err != nil {
	//	t.Errorf("close error: %v", err)
	//}
}
