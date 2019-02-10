package web

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver package
	"github.com/z0rr0/unigma/conf"
	"github.com/z0rr0/unigma/db"
)

const (
	testConfig  = "/tmp/unigma.json"
	testStorage = "/tmp/unigma_storage"
)

var (
	loggerInfo   = log.New(os.Stdout, "[TEST]", log.Ltime|log.Lshortfile)
	rgCheck      = regexp.MustCompile(`href="http(s)?://.+/(?P<key>[0-9a-z]{64})"`)
	rgShortCheck = regexp.MustCompile(`URL: http(s)?://.+/(?P<key>[0-9a-z]{64})`)
)

type formData struct {
	File     string
	FileName string
	TTL      string
	Times    string
	Password string
}

type uploadTestCase struct {
	F    *formData
	Code int
}

type downloadTestCase struct {
	Hash     string
	Password string
	Code     int
}

func createItem(cfg *conf.Cfg, secret, content string, expired time.Time) (*db.Item, error) {
	now := time.Now().UTC()
	item := &db.Item{
		Name:    "test.txt",
		Path:    testStorage,
		Salt:    "abc",
		Counter: 1,
		Created: now,
		Expired: expired,
	}
	f := strings.NewReader(content)
	err := item.Encrypt(f, cfg.Secret(secret), loggerInfo)
	if err != nil {
		return nil, err
	}
	err = item.Save(cfg.Db)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func createForm(f *formData) (io.Reader, string, error) {
	var b bytes.Buffer
	fw := multipart.NewWriter(&b)
	// file
	w, err := fw.CreateFormFile("file", f.FileName)
	if err != nil {
		return nil, "", err
	}
	_, err = w.Write([]byte(f.File))
	if err != nil {
		return nil, "", err
	}
	// ttl
	w, err = fw.CreateFormField("ttl")
	if err != nil {
		return nil, "", err
	}
	_, err = w.Write([]byte(f.TTL))
	if err != nil {
		return nil, "", err
	}
	// times
	w, err = fw.CreateFormField("times")
	if err != nil {
		return nil, "", err
	}
	_, err = w.Write([]byte(f.Times))
	if err != nil {
		return nil, "", err
	}
	// password
	w, err = fw.CreateFormField("password")
	if err != nil {
		return nil, "", err
	}
	_, err = w.Write([]byte(f.Password))
	if err != nil {
		return nil, "", err
	}
	err = fw.Close()
	if err != nil {
		return nil, "", err
	}
	return &b, fw.FormDataContentType(), nil
}

func TestIndex(t *testing.T) {
	cfg, err := conf.New(testConfig, loggerInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cfg.Close(); err != nil {
			t.Error(err)
		}
	}()
	w := httptest.NewRecorder()
	code, err := Index(w, nil, cfg)
	if err != nil {
		t.Error(err)
	}
	if code != http.StatusOK {
		t.Errorf("failed code: %v", code)
	}
}

func TestUpload(t *testing.T) {
	cfg, err := conf.New(testConfig, loggerInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cfg.Close(); err != nil {
			t.Error(err)
		}
	}()
	values := []*uploadTestCase{
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Times: "1", Password: "test"},
			Code: http.StatusOK,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", Times: "1", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Times: "1", Password: ""},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "604800", Times: "1000", Password: "test"},
			Code: http.StatusOK,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "604801", Times: "1000", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "604800", Times: "1001", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "a", Times: "1", Password: ""},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Times: "a", Password: ""},
			Code: http.StatusBadRequest,
		},
	}
	for i, tc := range values {
		body, contentType, err := createForm(tc.F)
		if err != nil {
			t.Fatal(err)
		}
		wr := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/upload", body)
		r.Header.Set("Content-Type", contentType)

		errExpected := tc.Code != http.StatusOK
		code, err := Upload(wr, r, cfg)
		if !errExpected && (err != nil) {
			t.Error(err)
		}
		if code != tc.Code {
			t.Errorf("[%v] failed code %v!=%v", i, code, tc.Code)
		}
		if errExpected {
			continue
		}
		// only status 200
		b := make([]byte, 1024)
		resp := wr.Result()
		_, err = resp.Body.Read(b)
		if err != nil {
			t.Error(err)
		}
		finds := rgCheck.FindStringSubmatch(string(b))
		if l := len(finds); l != 3 {
			t.Fatalf("failed result check lenght: %v", l)
		}
		key := finds[2]

		wr = httptest.NewRecorder()
		r = httptest.NewRequest("GET", "/"+key, nil)
		code, err = Download(wr, r, cfg)
		if err != nil {
			t.Error(err)
		}
		if code != http.StatusOK {
			t.Errorf("failed code: %v", code)
		}
	}
}

func TestDownload(t *testing.T) {
	cfg, err := conf.New(testConfig, loggerInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cfg.Close(); err != nil {
			t.Error(err)
		}
	}()
	now := time.Now().UTC()
	secret := "secret"
	content := "content"

	item, err := createItem(cfg, secret, content, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	period := 500 * time.Millisecond
	monitorClosed := make(chan struct{})
	go db.GCMonitor(cfg.Ch, monitorClosed, cfg.Db, loggerInfo, loggerInfo, period)
	defer func() {
		close(monitorClosed)
		time.Sleep(period)
	}()

	values := []*downloadTestCase{
		{Hash: "abc", Password: secret, Code: http.StatusNotFound},
		{Hash: "", Password: secret, Code: http.StatusNotFound},
		{Hash: "ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc2", Password: secret, Code: http.StatusNotFound},
		{Hash: item.Hash, Password: "bad", Code: http.StatusBadRequest},
		{Hash: item.Hash, Password: "", Code: http.StatusBadRequest},
		{Hash: item.Hash, Password: secret, Code: http.StatusOK}, // delete
	}
	for i, tc := range values {
		body := strings.NewReader("password=" + tc.Password)
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/"+tc.Hash, body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		errExpected := tc.Code != http.StatusOK
		code, err := Download(w, r, cfg)
		if !errExpected && (err != nil) {
			t.Error(err)
		}
		if code != tc.Code {
			t.Errorf("[%v] failed code %v!=%v", i, code, tc.Code)
		}
		if errExpected {
			continue
		}
		// only status 200
		b := make([]byte, 1024)
		resp := w.Result()
		_, err = resp.Body.Read(b)
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(string(b), content) {
			t.Errorf("missed content [%v]", i)
		}
	}
}

func TestUploadShort(t *testing.T) {
	cfg, err := conf.New(testConfig, loggerInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cfg.Close(); err != nil {
			t.Error(err)
		}
	}()
	values := []*uploadTestCase{
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Times: "1", Password: "test"},
			Code: http.StatusOK,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt"},
			Code: http.StatusOK,
		},
		{
			F:    &formData{File: "content", TTL: "10", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Times: "1", Password: ""},
			Code: http.StatusOK,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "604800", Times: "1000", Password: "test"},
			Code: http.StatusOK,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "604801", Times: "1000", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "604800", Times: "1001", Password: "test"},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "a", Times: "1", Password: ""},
			Code: http.StatusBadRequest,
		},
		{
			F:    &formData{File: "content", FileName: "test.txt", TTL: "10", Times: "a", Password: ""},
			Code: http.StatusBadRequest,
		},
	}
	for i, tc := range values {
		body, contentType, err := createForm(tc.F)
		if err != nil {
			t.Fatal(err)
		}
		wr := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/u", body)
		r.Header.Set("Content-Type", contentType)

		errExpected := tc.Code != http.StatusOK
		code, err := UploadShort(wr, r, cfg)
		if !errExpected && (err != nil) {
			t.Error(err)
		}
		if code != tc.Code {
			t.Errorf("[%v] failed code %v!=%v", i, code, tc.Code)
		}
		if errExpected {
			continue
		}
		// only status 200
		b := make([]byte, 1024)
		resp := wr.Result()
		_, err = resp.Body.Read(b)
		if err != nil {
			t.Error(err)
		}
		finds := rgShortCheck.FindStringSubmatch(string(b))
		if l := len(finds); l != 3 {
			t.Fatalf("failed result check lenght: %v", l)
		}
		key := finds[2]

		wr = httptest.NewRecorder()
		r = httptest.NewRequest("GET", "/"+key, nil)
		code, err = Download(wr, r, cfg)
		if err != nil {
			t.Error(err)
		}
		if code != http.StatusOK {
			t.Errorf("failed code: %v", code)
		}
	}
}
