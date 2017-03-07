package mfGalleryMetaCreatorGo

import (
	"log"
	"sort"
)

// available image order functions
var IMAGE_ORDER_FUNCTIONS = [...]string{"exifTimeAsc", "exifTimeDesc", "filenameAsc", "filenameDesc"}

func SortImages(orderFunctionName string, images JsonImages) {
	switch orderFunctionName {
	case IMAGE_ORDER_FUNCTIONS[0]:
		sort.Sort(byExifTimeAsc{images})
		break
	case IMAGE_ORDER_FUNCTIONS[1]:
		sort.Sort(byExifTimeDesc{images})
		break
	case IMAGE_ORDER_FUNCTIONS[2]:
		sort.Sort(byFilenameAsc{images})
		break
	case IMAGE_ORDER_FUNCTIONS[3]:
		sort.Sort(byExifTimeDesc{images})
		break
	default:
		log.Fatal("Not supported:", orderFunctionName)
	}

}

// sort functions for images

type JsonImages []MetaJsonImage

func (s JsonImages) Len() int      { return len(s) }
func (s JsonImages) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type byExifTimeAsc struct{ JsonImages }

func (s byExifTimeAsc) Less(i, j int) bool {
	return lessTimeNameAsc(s.JsonImages[i].Exif.Time, s.JsonImages[j].Exif.Time,
		s.JsonImages[i].Filename, s.JsonImages[j].Filename)
}

type byExifTimeDesc struct{ JsonImages }

func (s byExifTimeDesc) Less(i, j int) bool {
	return lessTimeNameDesc(s.JsonImages[i].Exif.Time, s.JsonImages[j].Exif.Time,
		s.JsonImages[i].Filename, s.JsonImages[j].Filename)
}

type byFilenameAsc struct{ JsonImages }

func (s byFilenameAsc) Less(i, j int) bool {
	return s.JsonImages[i].Filename < s.JsonImages[j].Filename
}

type byFilenameDesc struct{ JsonImages }

func (s byFilenameDesc) Less(i, j int) bool {
	return s.JsonImages[i].Filename >= s.JsonImages[j].Filename
}

// sort functions for directories

type JsonDirs []MetaJsonSubDir

func (s JsonDirs) Len() int      { return len(s) }
func (s JsonDirs) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByTimeDesc struct{ JsonDirs }

func (s ByTimeDesc) Less(i, j int) bool {
	return lessTimeNameDesc(s.JsonDirs[i].Time, s.JsonDirs[j].Time,
		s.JsonDirs[i].Title, s.JsonDirs[j].Title)
}

// generic sort functsion

// lowest date first, 'without time' is always the last
func lessTimeNameAsc(timeA, timeB *int64, nameA, nameB string) bool {
	if timeA != nil && timeB != nil {
		return *timeA < *timeB
	}
	if timeA == nil && timeB == nil {
		return nameA < nameB
	}
	return timeA != nil
}

// highest date first, 'without time' is always the last
func lessTimeNameDesc(timeA, timeB *int64, nameA, nameB string) bool {
	if timeA != nil && timeB != nil {
		return *timeA >= *timeB
	}
	if timeA == nil && timeB == nil {
		return nameA < nameB
	}
	return timeA != nil
}
