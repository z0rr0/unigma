#!/usr/bin/env bash

UNIGMA="https://unigma.lus.su/u"
FILE=""
TTL=""
TIMES=""
PASSWORD=""
CURL=`which curl`
VERBOSE=""

if [[ ! -x "$CURL" ]]; then
    echo "ERROR: curl is not found"
    exit 1
fi

function usage() {
    echo "Usage: $0: [-v] [-t ttl] [-c times] [-p password] -f FILE"
}

while getopts "f:t:c:p:v" name; do
    case $name in
        f)
            FILE="$OPTARG"
            ;;
        t)
            TTL="$OPTARG"
            ;;
        c)
            TIMES="$OPTARG"
            ;;
        p)
            PASSWORD="$OPTARG"
            ;;
        v)
            VERBOSE="-v"
            ;;
        h)
            usage
            exit 0
            ;;
        *)
            echo "Invalid option: -$OPTARG" >&2
            usage
            exit 2
            ;;
  esac
done


if [[ ! -f "$FILE" ]]; then
    echo "ERROR: file is required"
    usage
    exit 3
fi

${CURL} ${VERBOSE} -F "ttl=${TTL}" -F "times=${TIMES}" -F "password=${PASSWORD}" -F "file=@${FILE}" ${UNIGMA}
