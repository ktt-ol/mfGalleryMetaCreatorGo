FROM golang:1.6
ENV GOOS linux
ENV GOARCH amd64

RUN apt-get update && apt-get -y upgrade && apt-get install -y libjpeg62-turbo-dev

WORKDIR /go/src/github.com/ktt-ol/mfGalleryMetaCreatorGo
