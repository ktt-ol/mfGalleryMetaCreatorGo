package mfGalleryMetaCreatorGo

import (
	"runtime/debug"
	"log"
	"strings"
)

func CheckError(err error, messages ...string) {
	if err == nil {
		return
	}
	debug.PrintStack()
	log.Fatal(strings.Join(messages, " "), "\nError: ", err)
}
