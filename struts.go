package mfGalleryMetaCreatorGo

import (
	"fmt"
	"path"
	"strings"
	"strconv"
)

type IntList []int

func (i *IntList) String() string {
	return fmt.Sprintf("%d", *i)
}
func (i *IntList) Set(value string) error {
	tmp, err := strconv.ParseUint(value, 10, 16)

	if err != nil {
		*i = append(*i, 0)
	} else {
		*i = append(*i, int(tmp))
	}
	return nil
}

type FolderContent struct {
	FullPath      string
	Name          string
	Time          *int64
	Title         string
	Config        FolderConfig
	Files         []string
	ImageMetadata map[string]MetaJsonImage
	Folder        []FolderContent
}

func (fc *FolderContent) GetFullPathFile(file string) string {
	return path.Join(fc.FullPath, file)
}

func (fc *FolderContent) GetFolderTitle() string {
	if len(fc.Config.Title) > 0 {
		return fc.Config.Title
	}
	return fc.Name
}

func (fc *FolderContent) String() string {
	return fc.StringWithIntent(0)
}

func (fc *FolderContent) StringWithIntent(intention int) string {
	var sub string = ""
	for _, subFolder := range fc.Folder {
		sub = sub + subFolder.StringWithIntent(intention+4)
	}

	intent := strings.Repeat(" ", intention)

	return fmt.Sprintf(`%sfullPath: %s
%sfolderName: %s
%sconfig: %s
%sfiles: len -> %d
%stime: %d
%stitle: %s
%sprevImages: %s
%sfolder:
%s`, intent, fc.FullPath,
		intent, fc.Name,
		intent, fc.Config,
		intent, len(fc.Files),
		intent, fc.Time,
		intent, fc.Title,
		intent, "TODO",
		intent,
		sub)
}

type FolderConfig struct {
	// sets the title of the album. The default is the folder name.
	Title string
	// description of the folder. The default is none.
	Description string
	// sets the cover image for the album. The default is the first image in the album.
	Cover string
}

type MetaJson struct {
	Meta MetaJsonMeta `json:"meta"`
	// TODO use pointer
	Images  []MetaJsonImage  `json:"images"`
	SubDirs []MetaJsonSubDir `json:"subDirs"`
}

type MetaJsonMeta struct {
	Title       string `json:"title"`
	Time        *int64 `json:"time"`
	Description string `json:"description"`
}

type RotationAction int

const (
	NO_ROTATION = iota
	ROTATE_90
	ROTATE_180
	ROTATE_270
)

type MetaJsonImage struct {
	Filename string         `json:"filename"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	Exif     metaJsonExif   `json:"exif"`
	Rotate   RotationAction `json:"-"`
}

type MetaJsonSubDir struct {
	FolderName string  `json:"foldername"`
	Title      string  `json:"title"`
	Time       *int64  `json:"time"`
	Cover      *string `json:"cover"`
	ImageCount int     `json:"imageCount"`
}

type metaJsonExif struct {
	// all values are optional
	Make  *string `json:"make"`
	Model *string `json:"model"`
	Time  *int64  `json:"time"`
}

type ChromecastImage struct {
	Filename string `json:"filename"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Time     *int64 `json:"time"`
}
