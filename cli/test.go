package main

import (
	"fmt"
	"os"

	"errors"
	"strings"
	"time"

	"image"
	_ "image/jpeg"

	"image/jpeg"
	"log"

	"io/ioutil"

	"github.com/disintegration/imaging"
	"github.com/xor-gate/goexif2/exif"
	"github.com/xor-gate/goexif2/tiff"
)

func main() {
	//check("/Users/holger/Seafile/devProjects/mainframe/mfGallery/app/test-album/album1/sub1/CIMG1825.JPG")
	//check("/Users/holger/Seafile/devProjects/mainframe/mfGallery/app/test-album/album1/sub1/CIMG1851.JPG")
	//check("/Users/holger/Seafile/devProjects/mainframe/mfGallery/app/test-album/album1/sub1/CIMG1840.JPG")

	testCreateThumbnail("/Users/holger/Seafile/devProjects/mainframe/mfGallery/app/test-album/album1/sub1/CIMG1825.JPG",
		"/Users/holger/tmp/imgtest.jpg", 400)
	//listOrientation("/Users/holger/Seafile/devProjects/mainframe/mfGallery/app/test-album/album1/sub1")
}

func listOrientation(folder string) {
	files, _ := ioutil.ReadDir(folder)
	for _, file := range files {
		f, err := os.Open(folder + "/" + file.Name())
		if err != nil {
			panic(err)
		}

		if !strings.HasSuffix(strings.ToLower(file.Name()), ".jpg") {
			continue
		}

		x, err := exif.Decode(f)
		if err != nil {
			fmt.Println(file.Name(), " -> No exif")
			continue
		}
		orientation, err := x.Get(exif.Orientation)
		if err != nil {
			fmt.Println(file.Name(), " -> No orientation")
			continue
		}
		orientationVal, err := orientation.Int(0)
		if err != nil {
			panic(err)
		}
		fmt.Println(file.Name(), " -> ", orientationVal)
		f.Close()
	}
}

func check(file string) {

	f, _ := os.Open(file)
	defer f.Close()

	imageConfig, _, err := image.DecodeConfig(f)
	if err != nil {
		panic(err)
	}
	w, h := imageConfig.Width, imageConfig.Height

	// reset the file pointer
	f.Seek(0, 0)
	x, _ := exif.Decode(f)
	orientation, _ := x.Get(exif.Orientation)
	orientationVal, _ := orientation.Int(0)
	fmt.Println("ori val", orientationVal, "orig w/h", w, h)
	if orientationVal > 4 {
		w, h = h, w
	}

	fmt.Println(file, "-->> w/h", w, h)
}

func testCreateThumbnail(input string, output string, size int) {
	log.Printf("Create thumbnail (%d) for %s\n", size, input)
	if size <= 0 {
		log.Fatal("Invalid thumbnail size: ", size)
	}

	file, _ := os.Open(input)
	defer file.Close()

	// decode jpeg into image.Image
	img, _ := jpeg.Decode(file)

	//thumbnail := resize.Thumbnail(uint(size), uint(size), img, resize.Bilinear)
	thumbnail := imaging.Fit(img, size, size, imaging.Linear)

	// rotate
	thumbnail = imaging.Rotate90(thumbnail)

	// write
	out, _ := os.Create(output)
	defer out.Close()

	jpeg.Encode(out, thumbnail, nil)
}

// from the exif package, but I use the UTC location as default (instead of the time.Local)
func DateTime(x *exif.Exif) (time.Time, error) {
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
