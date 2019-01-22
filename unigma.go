package main

import (
	"flag"
	"fmt"
	"github.com/z0rr0/unigma/conf"
	"log"
	"os"
	"runtime"
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
	cfg, err := conf.New(*config)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := cfg.Close(); err != nil {
			loggerError.Println(err)
		}
	}()
}
