#!/usr/bin/env bash
go build -a -ldflags '-w -extldflags "-static"' cli/makeMeta.go
