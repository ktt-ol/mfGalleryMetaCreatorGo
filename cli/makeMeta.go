package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"image"

	"sort"
	"time"

	"math"

	"errors"

	"github.com/go-ini/ini"
	mfg "github.com/ktt-ol/mfGalleryMetaCreatorGo"
	"github.com/xor-gate/goexif2/exif"
	"github.com/xor-gate/goexif2/tiff"
)

var filePattern = regexp.MustCompile(mfg.FILE_REGEXP)

// folder date pattern
var ymdPattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})_(.*)$`)
var ymPattern = regexp.MustCompile(`^(\d{4})-(\d{2})_(.*)$`)
var yPattern = regexp.MustCompile(`^(\d{4})_(.*)$`)

func main() {
	imagePathPtr := flag.String("path", "", "the path to the images (required)")
	var sizes mfg.IntList
	flag.Var(&sizes, "size", "the bounding box of the thumbnails (required). You can use this parameter more than once.")
	orderPtr := flag.String("order", mfg.IMAGE_ORDER_FUNCTIONS[0], strings.Join(mfg.IMAGE_ORDER_FUNCTIONS[:], ","))
	ccSizePtr := flag.Int("cc-size", -1, "creates a jsonp file for the Chromecast for this thumbnail size.")
	forceUpdatePtr := flag.Bool("force-update", false, "ignores the existing "+mfg.META_NAME+" files.")
	maxThreads := flag.Int("max-threads", -1, "The maximum amount of threads to use. Default is the number of cpu.")
	debug := flag.Bool("debug", false, "activates debug logging.")

	flag.Parse()

	if *imagePathPtr == "" || len(sizes) == 0 || !isValidOrder(*orderPtr) {
		flag.Usage()
		os.Exit(1)
	}

	// add the requested size for the Chromecast to size slice
	sizes = addSizeIfNeeded(sizes, *ccSizePtr)

	checkSizes(sizes)

	log.Printf("Reading '%s' for images...", *imagePathPtr)
	content := readFolder(*imagePathPtr, *forceUpdatePtr)

	updateImageMetaInfos(content)
	if *debug {
		log.Printf("Data model:\n%s\n", content)
	}
	mfg.UpdateThumbnails(content, sizes, *maxThreads)
	writeMetaFiles(content, *orderPtr, *ccSizePtr)
}

func checkSizes(sizes mfg.IntList) {
	for _, size := range sizes {
		if size <= 0 {
			log.Fatal("Invalid size: ", size)
		}
	}
}

func addSizeIfNeeded(sizeList mfg.IntList, ccSize int) mfg.IntList {
	if ccSize == -1 {
		return sizeList
	}
	for _, size := range sizeList {
		if size == ccSize {
			return sizeList
		}
	}

	return append(sizeList, ccSize)
}

func readFolder(folder string, forceUpdate bool) *mfg.FolderContent {
	content := mfg.FolderContent{FullPath: folder, Name: path.Base(folder)}
	content.ImageMetadata = make(map[string]mfg.MetaJsonImage)

	files, err := ioutil.ReadDir(folder)
	mfg.CheckError(err)
	for _, file := range files {
		// skip .xxxx folder/files
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		var fullPath = content.GetFullPathFile(file.Name())
		if file.IsDir() {
			content.Folder = append(content.Folder, *readFolder(fullPath, forceUpdate))
			//content.AddSubFolder(*readFolder(fullPath, forceUpdate))
			continue
		}

		if file.Name() == mfg.CONTENT_INI {
			log.Println("Content INI file found in ", folder)
			content.Config = readIniFile(fullPath)
			continue
		}

		if !forceUpdate && file.Name() == mfg.META_NAME {
			log.Println("Previous generated meta file found in ", folder)
			readPrevImageInfos(content.ImageMetadata, fullPath)
			continue
		}

		if !filePattern.MatchString(file.Name()) {
			continue
		}

		content.Files = append(content.Files, file.Name())
	}

	return &content
}

// reads recursively all meta data, if needed
func updateImageMetaInfos(folder *mfg.FolderContent) {
	var newestTime int64 = math.MinInt64
	for _, imgFile := range folder.Files {
		imgMeta, exists := folder.ImageMetadata[imgFile]
		if !exists {
			imgMeta = readImageInfo(imgFile, folder.GetFullPathFile(imgFile))
			folder.ImageMetadata[imgFile] = imgMeta
		}

		if imgMeta.Exif.Time != nil && *imgMeta.Exif.Time > newestTime {
			newestTime = *imgMeta.Exif.Time
		}
	}

	title, timestamp, ok := parseTitleAndDateFromFoldername(folder.Name)
	folder.Title = strings.Replace(title, "_", " ", -1)

	if ok {
		fTime := timestamp.UnixNano() / 1000 / 1000
		folder.Time = &fTime
	} else if newestTime != math.MinInt64 {
		folder.Time = &newestTime
	}

	for i := range folder.Folder {
		updateImageMetaInfos(&folder.Folder[i])
		// update the folder time if any sub folder has a newer time
		if folder.Time == nil || (folder.Folder[i].Time != nil && *folder.Time < *folder.Folder[i].Time) {
			folder.Time = folder.Folder[i].Time
		}
	}
}

func writeMetaFiles(folder *mfg.FolderContent, imageOrderFunction string, ccSize int) {
	log.Println("Writing meta file for ", folder.Name)
	meta := mfg.MetaJson{}
	meta.Images = make([]mfg.MetaJsonImage, len(folder.Files))
	for i, imgFile := range folder.Files {
		imgMeta, found := folder.ImageMetadata[imgFile]
		if !found {
			log.Fatal("Expected to find '", imgMeta, "' in imageMetadata. Folder: ", folder.Name)
		}
		meta.Images[i] = imgMeta
	}

	mfg.SortImages(imageOrderFunction, meta.Images)

	meta.Meta.Title = folder.GetFolderTitle()
	meta.Meta.Description = folder.Config.Description

	meta.SubDirs = make([]mfg.MetaJsonSubDir, len(folder.Folder))
	for i := range folder.Folder {
		subFolder := &folder.Folder[i]
		sub := &meta.SubDirs[i]
		sub.FolderName = subFolder.Name
		sub.Title = subFolder.GetFolderTitle()
		sub.Time = subFolder.Time
		sub.ImageCount = sumFolderImageCount(subFolder)
		if len(subFolder.Config.Cover) > 0 {
			sub.Cover = &subFolder.Config.Cover
		} else if len(subFolder.Files) > 0 {
			sub.Cover = &subFolder.Files[0]
		}

		writeMetaFiles(subFolder, imageOrderFunction, ccSize)
	}

	// all sub dirs are read -> sets the time
	meta.Meta.Time = folder.Time // default

	sort.Sort(mfg.ByTimeDesc{meta.SubDirs})

	metaFileFullPath := path.Join(folder.FullPath, mfg.META_NAME)
	bytes, err := json.Marshal(meta)
	mfg.CheckError(err, "Can't write meta file.")
	ioutil.WriteFile(metaFileFullPath, bytes, 0644)

	if ccSize != -1 {
		writeChromecastMetaFile(ccSize, meta.Images, folder)
	}
}

// calculates the amount of photos of this folder inclusive all images in sub folders
func sumFolderImageCount(folder *mfg.FolderContent) int {
	sum := len(folder.Files)
	for _, sub := range folder.Folder {
		sum += sumFolderImageCount(&sub)
	}
	return sum
}

func writeChromecastMetaFile(ccSize int, images []mfg.MetaJsonImage, folder *mfg.FolderContent) {
	log.Println("Writing Chromecast meta file for ", folder.Name)
	var ccImages = make([]mfg.ChromecastImage, len(images))
	for i, image := range images {
		filename := fmt.Sprintf("%s/%d-%s", mfg.THUMB_DIR, ccSize, image.Filename)
		ccImages[i] = mfg.ChromecastImage{filename, image.Width, image.Height, image.Exif.Time}
	}

	ccFilename := folder.FullPath + "/" + mfg.META_NAME_CHROMECAST
	bytes, err := json.Marshal(ccImages)
	mfg.CheckError(err, "Can't write Chromecast meta file.")
	f, err := os.OpenFile(ccFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	mfg.CheckError(err)
	defer f.Close()
	f.Write([]byte(mfg.CC_PREFIX))
	f.Write(bytes)
	f.Write([]byte(mfg.CC_SUFFIX))
}

func parseTitleAndDateFromFoldername(filename string) (string, time.Time, bool) {
	result := ymdPattern.FindStringSubmatch(filename)
	if len(result) > 0 {
		year, _ := strconv.Atoi(result[1])
		month, _ := strconv.Atoi(result[2])
		day, _ := strconv.Atoi(result[3])
		return result[4], time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
	}

	result = ymPattern.FindStringSubmatch(filename)
	if len(result) > 0 {
		year, _ := strconv.Atoi(result[1])
		month, _ := strconv.Atoi(result[2])
		return result[3], time.Date(year, time.Month(month), 0, 0, 0, 0, 0, time.UTC), true
	}

	result = yPattern.FindStringSubmatch(filename)
	if len(result) > 0 {
		year, _ := strconv.Atoi(result[1])
		return result[2], time.Date(year, 0, 0, 0, 0, 0, 0, time.UTC), true
	}

	return filename, time.Time{}, false
}

func readImageInfo(filename, input string) mfg.MetaJsonImage {
	log.Println("Read image meta info from ", input)

	f, err := os.Open(input)
	mfg.CheckError(err)
	defer f.Close()

	imageConfig, _, err := image.DecodeConfig(f)
	mfg.CheckError(err)
	imageMeta := mfg.MetaJsonImage{Filename: filename, Width: imageConfig.Width, Height: imageConfig.Height}

	// reset the file pointer
	f.Seek(0, 0)

	x, err := exif.Decode(f)
	if err != nil && exif.IsCriticalError(err) {
		log.Println("Warn: can't read exif. ", err)
		return imageMeta
	}

	if camModel, err := x.Get(exif.Model); err == nil {
		if model, err := camModel.StringVal(); err == nil {
			model = strings.TrimSpace(model)
			imageMeta.Exif.Model = &model
		}
	}

	if camMaker, err := x.Get(exif.Make); err == nil {
		if maker, err := camMaker.StringVal(); err == nil {
			maker = strings.TrimSpace(maker)
			imageMeta.Exif.Make = &maker
		}
	}

	if datetime, err := getExifTime(x); err == nil {
		timeInMS := datetime.UnixNano() / 1000 / 1000
		imageMeta.Exif.Time = &timeInMS
	}

	imageMeta.Rotate = mfg.NO_ROTATION
	if orientation, err := x.Get(exif.Orientation); err == nil {
		if orientationVal, err := orientation.Int(0); err == nil {
			// http://jpegclub.org/exif_orientation.html
			switch orientationVal {
			case 3:
			case 4:
				imageMeta.Rotate = mfg.ROTATE_180
				break
			case 5:
			case 6:
				imageMeta.Rotate = mfg.ROTATE_270
				imageMeta.Width, imageMeta.Height = imageMeta.Height, imageMeta.Width
				break
			case 7:
			case 8:
				imageMeta.Rotate = mfg.ROTATE_90
				imageMeta.Width, imageMeta.Height = imageMeta.Height, imageMeta.Width
				break
			}
		}
	}

	return imageMeta
}

// from the exif package (exif.DateTime), but I use the UTC location as default (instead of the time.Local)
func getExifTime(x *exif.Exif) (time.Time, error) {
	var dt time.Time
	tag, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		tag, err = x.Get(exif.DateTime)
		if err != nil {
			return dt, err
		}
	}
	if tag.Format() != tiff.StringVal {
		return dt, errors.New("DateTime[Original] not in string format")
	}
	exifTimeLayout := "2006:01:02 15:04:05"
	dateStr := strings.TrimRight(string(tag.Val), "\x00")
	timeZone := time.UTC
	if tz, _ := x.TimeZone(); tz != nil {
		fmt.Println("tz found: ", tz)
		timeZone = tz
	}
	return time.ParseInLocation(exifTimeLayout, dateStr, timeZone)
}

func isValidOrder(value string) bool {
	for i := 0; i < len(mfg.IMAGE_ORDER_FUNCTIONS); i++ {
		if value == mfg.IMAGE_ORDER_FUNCTIONS[i] {
			return true
		}
	}
	return false
}

func readIniFile(iniFile string) mfg.FolderConfig {
	cfg, err := ini.Load(iniFile)
	mfg.CheckError(err, "Error reading ini file.", iniFile)
	section, err := cfg.GetSection("")
	mfg.CheckError(err, "Error reading section file.", iniFile)

	config := mfg.FolderConfig{}
	title, err := section.GetKey("title")
	if err == nil {
		config.Title = title.Value()
	}
	description, err := section.GetKey("description")
	if err == nil {
		config.Description = description.Value()
	}
	cover, err := section.GetKey("cover")
	if err == nil {
		config.Cover = cover.Value()
	}

	return config
}

func readPrevImageInfos(metaMap map[string]mfg.MetaJsonImage, jsonFile string) {
	bytes, err := ioutil.ReadFile(jsonFile)
	mfg.CheckError(err, "Error reading json file.")

	var jsonContent mfg.MetaJson
	err = json.Unmarshal(bytes, &jsonContent)
	mfg.CheckError(err, "Invalid json in file.")
	for _, imgInfo := range jsonContent.Images {
		metaMap[imgInfo.Filename] = imgInfo
	}
}
