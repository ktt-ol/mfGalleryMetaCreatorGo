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

	"image/jpeg"

	"image"

	"sort"
	"time"

	"math"

	"runtime/debug"

	"errors"

	"github.com/disintegration/imaging"
	"github.com/go-ini/ini"
	mfg "github.com/ktt-ol/mfGalleryMetaCreatorGo"
	"github.com/xor-gate/goexif2/exif"
	"github.com/xor-gate/goexif2/tiff"
)

const (
	FILE_REGEXP          = `(?i)\.jpe?g$`
	THUMB_DIR            = ".go_thumbs"
	CONTENT_INI          = "content.ini"
	META_NAME            = "go_meta.json"
	META_NAME_CHROMECAST = "go_meta_cc.jsonp.js"
	MAX_PROCESS_SPAWNS   = 5
	CC_PREFIX            = "ifsImagesDataCallback("
	CC_SUFFIX            = ");"
)

type intList []int

func (i *intList) String() string {
	return fmt.Sprintf("%d", *i)
}
func (i *intList) Set(value string) error {
	tmp, err := strconv.ParseUint(value, 10, 16)

	if err != nil {
		*i = append(*i, 0)
	} else {
		*i = append(*i, int(tmp))
	}
	return nil
}

var filePattern = regexp.MustCompile(FILE_REGEXP)

// folder date pattern
var ymdPattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})_(.*)$`)
var ymPattern = regexp.MustCompile(`^(\d{4})-(\d{2})_(.*)$`)
var yPattern = regexp.MustCompile(`^(\d{4})_(.*)$`)

func main() {
	imagePathPtr := flag.String("path", "", "the path to the images (required)")
	var sizes intList
	flag.Var(&sizes, "size", "the bounding box of the thumbnails (required). You can use this parameter more than once.")
	orderPtr := flag.String("order", mfg.IMAGE_ORDER_FUNCTIONS[0], strings.Join(mfg.IMAGE_ORDER_FUNCTIONS[:], ","))
	ccSizePtr := flag.Int("cc-size", -1, "creates a jsonp file for the Chromecast for this thumbnail size.")
	forceUpdatePtr := flag.Bool("force-update", false, "ignores the existing "+META_NAME+" files.")

	flag.Parse()

	println(*imagePathPtr)
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
	log.Println(content)
	updateThumbnails(content, sizes)
	writeMetaFiles(content, *orderPtr, *ccSizePtr)
}

func checkSizes(sizes intList) {
	for _, size := range sizes {
		if size <= 0 {
			log.Fatal("Invalid size: ", size)
		}
	}
}

func addSizeIfNeeded(sizeList intList, ccSize int) intList {
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
	checkError(err)
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

		if file.Name() == CONTENT_INI {
			log.Println("Content INI file found in ", folder)
			content.Config = readIniFile(fullPath)
			continue
		}

		if !forceUpdate && file.Name() == META_NAME {
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
	var oldestTime int64 = math.MaxInt64
	for _, imgFile := range folder.Files {
		imgMeta, exists := folder.ImageMetadata[imgFile]
		if !exists {
			imgMeta = readImageInfo(imgFile, folder.GetFullPathFile(imgFile))
			folder.ImageMetadata[imgFile] = imgMeta
		}

		if imgMeta.Exif.Time != nil && *imgMeta.Exif.Time < oldestTime {
			oldestTime = *imgMeta.Exif.Time
		}
	}

	title, timestamp, ok := parseTitleAndDateFromFoldername(folder.Name)
	folder.Title = strings.Replace(title, "_", " ", -1)
	if ok {
		fTime := timestamp.UnixNano() / 1000
		folder.Time = &fTime
	} else if oldestTime != math.MaxInt64 {
		folder.Time = &oldestTime
	}

	for i := range folder.Folder {
		updateImageMetaInfos(&folder.Folder[i])
	}
}

func updateThumbnails(folder *mfg.FolderContent, sizeList intList) {
	thumbFolder := path.Join(folder.FullPath, THUMB_DIR)
	if _, err := os.Stat(thumbFolder); os.IsNotExist(err) {
		os.Mkdir(thumbFolder, 0755)
	}
	for _, imgFile := range folder.Files {
		meta, _ := folder.ImageMetadata[imgFile]
		fullPathImage := folder.GetFullPathFile(imgFile)
		for _, size := range sizeList {
			targetFile := path.Join(thumbFolder, fmt.Sprintf("%d-%s", size, imgFile))
			if _, err := os.Stat(targetFile); os.IsNotExist(err) {
				createThumbnail(fullPathImage, targetFile, size, meta.Rotate)
			}
		}
	}

	for i := range folder.Folder {
		updateThumbnails(&folder.Folder[i], sizeList)
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
	meta.Meta.Time = folder.Time
	meta.Meta.Description = folder.Config.Description

	meta.SubDirs = make([]mfg.MetaJsonSubDir, len(folder.Folder))
	for i := range folder.Folder {
		subFolder := &folder.Folder[i]
		sub := &meta.SubDirs[i]
		sub.FolderName = subFolder.Name
		sub.Title = subFolder.GetFolderTitle()
		sub.Time = subFolder.Time
		sub.ImageCount = len(subFolder.Files)
		if len(subFolder.Config.Cover) > 0 {
			sub.Cover = &subFolder.Config.Cover
		} else if sub.ImageCount > 0 {
			sub.Cover = &subFolder.Files[0]
		}

		writeMetaFiles(subFolder, imageOrderFunction, ccSize)
	}

	sort.Sort(mfg.ByTimeDesc{meta.SubDirs})

	metaFileFullPath := path.Join(folder.FullPath, META_NAME)
	bytes, err := json.Marshal(meta)
	checkError(err, "Can't write meta file.")
	ioutil.WriteFile(metaFileFullPath, bytes, 0644)

	if ccSize != -1 {
		writeChromecastMetaFile(ccSize, meta.Images, folder)
	}
}
func writeChromecastMetaFile(ccSize int, images []mfg.MetaJsonImage, folder *mfg.FolderContent) {
	log.Println("Writing Chromecast meta file for ", folder.Name)
	var ccImages = make([]mfg.ChromecastImage, len(images))
	for i, image := range images {
		filename := fmt.Sprintf("%s/%d-%s", THUMB_DIR, ccSize, image.Filename)
		ccImages[i] = mfg.ChromecastImage{filename, image.Width, image.Height, image.Exif.Time}
	}

	ccFilename := folder.FullPath + "/" + META_NAME_CHROMECAST
	bytes, err := json.Marshal(ccImages)
	checkError(err, "Can't write Chromecast meta file.")
	f, err := os.OpenFile(ccFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	checkError(err)
	defer f.Close()
	f.Write([]byte(CC_PREFIX))
	f.Write(bytes)
	f.Write([]byte(CC_SUFFIX))
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
	checkError(err)
	defer f.Close()

	imageConfig, _, err := image.DecodeConfig(f)
	checkError(err)
	imageMeta := mfg.MetaJsonImage{Filename: filename, Width: imageConfig.Width, Height: imageConfig.Height}

	// reset the file pointer
	f.Seek(0, 0)

	x, err := exif.Decode(f)
	if err != nil && exif.IsCriticalError(err) {
		log.Println("Warn: can't read exif. ", err)
		return imageMeta
	}

	camModel, err := x.Get(exif.Model)
	checkError(err)
	model, err := camModel.StringVal()
	checkError(err)
	model = strings.TrimSpace(model)
	imageMeta.Exif.Model = &model

	camMaker, err := x.Get(exif.Make)
	checkError(err)
	maker, err := camMaker.StringVal()
	checkError(err)
	maker = strings.TrimSpace(maker)
	imageMeta.Exif.Make = &maker

	datetime, err := getExifTime(x)
	checkError(err)
	timeInMS := datetime.UnixNano() / 1000 / 1000
	imageMeta.Exif.Time = &timeInMS

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

func checkError(err error, messages ...string) {
	if err == nil {
		return
	}
	debug.PrintStack()
	log.Fatal(strings.Join(messages, " "), "\nError: ", err)
}

func createThumbnail(input string, output string, size int, rotationAction mfg.RotationAction) {
	log.Printf("Create thumbnail (%d) for %s (%d)\n", size, input, rotationAction)
	if size <= 0 {
		log.Fatal("Invalid thumbnail size: ", size)
	}

	file, err := os.Open(input)
	checkError(err, "Can't open image file.")
	defer file.Close()

	// decode jpeg into image.Image
	img, err := jpeg.Decode(file)
	checkError(err, "Can't decode image file.")

	thumbnail := imaging.Fit(img, size, size, imaging.Linear)

	switch rotationAction {
	case mfg.ROTATE_90:
		thumbnail = imaging.Rotate90(thumbnail)
		break
	case mfg.ROTATE_180:
		thumbnail = imaging.Rotate180(thumbnail)
		break
	case mfg.ROTATE_270:
		thumbnail = imaging.Rotate270(thumbnail)
		break
	}

	out, err := os.Create(output)
	checkError(err, "Can't write jpeg file.")
	defer out.Close()

	jpeg.Encode(out, thumbnail, nil)
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
	checkError(err, "Error reading ini file.", iniFile)
	section, err := cfg.GetSection("")
	checkError(err, "Error reading section file.", iniFile)

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
	checkError(err, "Error reading json file.")

	var jsonContent mfg.MetaJson
	err = json.Unmarshal(bytes, &jsonContent)
	checkError(err, "Invalid json in file.")
	for _, imgInfo := range jsonContent.Images {
		metaMap[imgInfo.Filename] = imgInfo
	}
}
