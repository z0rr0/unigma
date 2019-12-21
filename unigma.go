package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/z0rr0/unigma/conf"
	"github.com/z0rr0/unigma/db"
	"github.com/z0rr0/unigma/web"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

const (
	// Name is a program name.
	Name = "Enigma"
	// Config is default configuration file name.
	Config = "config.json"
)

var (
	// Version is git version
	Version = ""
	// Revision is revision number
	Revision = ""
	// BuildDate is build date
	BuildDate = ""
	// GoVersion is runtime Go language version
	GoVersion = runtime.Version()

	// internal loggers
	loggerError = log.New(os.Stderr, fmt.Sprintf("ERROR [%v]: ", Name),
		log.Ldate|log.Ltime|log.Lshortfile)
	loggerInfo = log.New(os.Stdout, fmt.Sprintf("INFO [%v]: ", Name),
		log.Ldate|log.Ltime|log.Lshortfile)
)

func getVersion(w http.ResponseWriter) error {
	_, err := fmt.Fprintf(w,
		"%v\nVersion: %v\nRevision: %v\nBuild date: %v\nGo version: %v\n",
		Name, Version, Revision, BuildDate, GoVersion,
	)
	return err
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			loggerError.Printf("abnormal termination [%v]: \n\t%v\n", Version, r)
		}
	}()
	version := flag.Bool("version", false, "show version")
	config := flag.String("config", Config, "configuration file")
	flag.Parse()

	versionInfo := fmt.Sprintf("\tVersion: %v\n\tRevision: %v\n\tBuild date: %v\n\tGo version: %v",
		Version, Revision, BuildDate, GoVersion)
	if *version {
		fmt.Println(versionInfo)
		return
	}
	cfg, err := conf.New(*config, loggerError)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := cfg.Close(); err != nil {
			loggerError.Println(err)
		}
	}()
	timeout := cfg.HandleTimeout()
	srv := &http.Server{
		Addr:           cfg.Addr(),
		Handler:        http.DefaultServeMux,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: cfg.MaxFileSize(),
		ErrorLog:       loggerInfo,
	}
	loggerInfo.Printf("\n%v\nstorage: %v\nlisten addr: %v\n", versionInfo, cfg.StorageDir, srv.Addr)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var err error
		start, code := time.Now(), http.StatusOK
		defer func() {
			loggerInfo.Printf("%-5v %v\t%-12v\t%v",
				r.Method,
				code,
				time.Since(start),
				r.URL.String(),
			)
		}()
		switch r.URL.Path {
		case "/version":
			code, err = http.StatusOK, getVersion(w)
		case "/":
			code, err = web.Index(w, r, cfg)
		case "/upload":
			code, err = web.Upload(w, r, cfg)
		case "/u":
			code, err = web.UploadShort(w, r, cfg)
		default:
			code, err = web.Download(w, r, cfg)
		}
		if err != nil {
			loggerError.Println(err)
		}
	})
	monitorClosed := make(chan struct{})
	go db.GCMonitor(cfg.Ch, monitorClosed, cfg.Db, loggerInfo, loggerError, time.Duration(cfg.GCPeriod)*time.Second)

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, os.Signal(syscall.SIGTERM), os.Signal(syscall.SIGQUIT))
		<-sigint

		if err := srv.Shutdown(context.Background()); err != nil {
			loggerInfo.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
		close(monitorClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		loggerInfo.Printf("HTTP server ListenAndServe: %v", err)
	}
	<-idleConnsClosed
	<-monitorClosed
	loggerInfo.Println("stopped")
}
