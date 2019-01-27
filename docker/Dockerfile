FROM alpine:latest
MAINTAINER Alexander Zaytsev "me@axv.email"
RUN apk update && \
    apk upgrade && \
    apk add ca-certificates tzdata
ADD unigma /bin/unigma
RUN chmod 0755 /bin/unigma
EXPOSE 18090
VOLUME ["/data/conf/", "/data/unigma/"]
ENTRYPOINT ["unigma"]
CMD ["-config", "/data/conf/unigma.json"]
