PROJECTNAME=$(shell basename "$(PWD)")

NAME=unigma
BIN=$(GOPATH)/bin
VERSION=`bash version.sh`
MAIN=github.com/z0rr0/$(NAME)
SOURCEDIR=src/$(MAIN)
CONTAINER=docker/build.sh
DOCKER_TAG=z0rr0/$(NAME)
CONFIG=config.example.json
SCHEMA=schema.sql
WEBDEBUG=http://localhost:18090
TMPDB=unigma_db.sqlite
TMPSTORAGE=unigma_storage
TMPCONF=unigma.json

PID=/tmp/.$(PROJECTNAME).pid
STDERR=/tmp/.$(PROJECTNAME)-stderr.txt

# MAKEFLAGS += --silent

all: test

install:
	go install -ldflags "$(VERSION)" $(MAIN)

lint: install
	go vet $(MAIN)
	golint $(MAIN)
	go vet $(MAIN)/db
	golint $(MAIN)/db
	go vet $(MAIN)/conf
	golint $(MAIN)/conf
	go vet $(MAIN)/web
	golint $(MAIN)/web
	go vet $(MAIN)/page
	golint $(MAIN)/page

prepare:
	@-cp -r config.example.json /tmp/$(TMPCONF)
	@-sed -i 's/"db.sqlite"/"\/tmp\/$(TMPDB)"/g' /tmp/$(TMPCONF)
	@-sed -i 's/"storage",/"\/tmp\/$(TMPSTORAGE)",/g' /tmp/$(TMPCONF)
	@-rm -f /tmp/$(TMPDB)
	@-mkdir -p /tmp/$(TMPSTORAGE)
	@-rm -f /tmp/$(TMPSTORAGE)/*
	@-cat $(GOPATH)/$(SOURCEDIR)/$(SCHEMA) | sqlite3 /tmp/$(TMPDB)

test: lint prepare
#	go test -race -v -cover -coverprofile=conf_coverage.out -trace conf_trace.out $(MAIN)/conf
	go test -race -v -cover -coverprofile=db_coverage.out -trace db_trace.out $(MAIN)/db
#	go test -race -v -cover -coverprofile=page_coverage.out -trace page_trace.out $(MAIN)/page
#	go test -race -v -cover -coverprofile=web_coverage.out -trace web_trace.out $(MAIN)/web
	# go tool cover -html=coverage.out
	# go tool trace ratest.test trace.out
	# go test -race -v -cover -coverprofile=coverage.out -trace trace.out $(MAIN)
	# go test -v -race -benchmem -bench=. $(MAIN)/<NAME>

docker: lint
	bash $(CONTAINER)
	docker build -t $(DOCKER_TAG) -f docker/Dockerfile .

docker-no-cache: lint
	bash $(CONTAINER)
	docker build --no-cache -t $(DOCKER_TAG) -f docker/Dockerfile

start: install
	@echo "  >  $(PROJECTNAME)"
	@-$(BIN)/$(PROJECTNAME) -config config.example.json & echo $$! > $(PID)
	@-cat $(PID)
#	@-sensible-browser $(WEBDEBUG)

stop:
	@-touch $(PID)
	@-cat $(PID)
	@-kill `cat $(PID)` 2> /dev/null || true
	@-rm $(PID)

restart: stop start

arm:
	env GOOS=linux GOARCH=arm go install -ldflags "$(VERSION)" $(MAIN)

linux:
	env GOOS=linux GOARCH=amd64 go install -ldflags "$(VERSION)" $(MAIN)

clean: stop
	rm -rf $(BIN)/*
	rm -f $(TMPDB)
	find $(GOPATH)/$(SOURCEDIR)/ -type f -name "*.out" -print0 -delete
