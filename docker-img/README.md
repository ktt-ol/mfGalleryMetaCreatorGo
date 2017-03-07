# build_with_docker

A docker image that builds the server. Create the image with
```sh
docker build -t mfg-go-linux-build .
```

Now you can use the command from the make file to build the server for linux (amd64):
```sh
docker run -it --rm -v "$(pwd)":/go/src/github.com/ktt-ol/mfGalleryMetaCreatorGo mfg-go-linux-build go build -v cli/makeMeta.go
```
