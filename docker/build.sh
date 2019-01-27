#!/usr/bin/env bash

NAME="unigma"
CONTAINER="gounigma"
SOURCES="${GOPATH}/src"
TARGET="${GOPATH}/bin/alpine"
ATTRS="`bash version.sh`"

rm -f ${TARGET}/bin/*
mkdir -p ${TARGET}/bin ${TARGET}/pkg

# build golang build container
docker build -t ${CONTAINER} -f docker/golang/Dockerfile .
if [[ $? -gt 0 ]]; then
	echo "ERROR: golang build container"
	exit 1
fi

/usr/bin/docker run --rm --user `id -u ${USER}`:`id -g ${USER}` \
    --volume ${SOURCES}:/usr/p/src:ro \
    --volume ${TARGET}/pkg:/usr/p/pkg \
    --volume ${TARGET}/bin:/usr/p/bin \
    --workdir /usr/p/src/github.com/z0rr0/${NAME} \
    --env GOPATH=/usr/p \
    --env GOCACHE=/tmp/.cache \
    ${CONTAINER} go install -v -ldflags "${ATTRS}" github.com/z0rr0/${NAME}

if [[ $? -gt 0 ]]; then
	echo "ERROR: build container"
	exit 1
fi
cp -v ${TARGET}/bin/${NAME}  ./