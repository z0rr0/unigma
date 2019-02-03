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
	"testing"

	"github.com/z0rr0/unigma/conf"
)

const (
	testConfig = "/tmp/unigma.json"
)

var (
	loggerInfo = log.New(os.Stdout, "[TEST]", log.Ltime|log.Lshortfile)
	rgCheck    = regexp.MustCompile(`href="http(s)?://.+/(?P<key>[0-9a-z]{64})"`)
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
	
}