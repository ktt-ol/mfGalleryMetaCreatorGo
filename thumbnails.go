package mfGalleryMetaCreatorGo

import (
	"fmt"
	"log"
	"os"
	"path"

	"runtime"

	"github.com/pixiv/go-libjpeg/jpeg"
	"github.com/disintegration/imaging"
	"image"
	"bufio"
)

type payload struct {
	input          string
	output         string
	size           int
	rotationAction RotationAction
}

// Creates thumbnails recursively for the given folder using a thread pool with NumCPU of threads.
// folder - works on this folder
// sizeList - creates thumbnails for this sizes. The size represents the maximum bounding box.
func UpdateThumbnails(folder *FolderContent, sizeList IntList, maxThreads int) {
	var maxWorker int
	if maxThreads <= 0 {
		maxWorker = runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		maxWorker = maxThreads
	}

	jobs := make(chan payload)
	workerDone := make(chan bool)

	for workerId := 1; workerId <= maxWorker; workerId++ {
		go thumbnailWorker(workerId, jobs, workerDone)
	}

	addThumbnailJobs(folder, sizeList, jobs)

	// no more jobs coming in
	close(jobs)

	// waiting on the worker to finish
	for workerId := 1; workerId <= maxWorker; workerId++ {
		<-workerDone
	}
}

func thumbnailWorker(id int, jobs <-chan payload, done chan<- bool) {
	counter := 0
	for job := range jobs {
		createThumbnail(job.input, job.output, job.size, job.rotationAction)
		// this helps to reduce the max memory usage
		runtime.GC()
		counter++
	}
	log.Printf("Thumbnail worker (%d) finished. Jobs done: %d.", id, counter)
	done <- true
}

func addThumbnailJobs(folder *FolderContent, sizeList IntList, jobs chan<- payload) {
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
				jobs <- payload{fullPathImage, targetFile, size, meta.Rotate}
			}
		}
	}

	for i := range folder.Folder {
		addThumbnailJobs(&folder.Folder[i], sizeList, jobs)
	}
}

func createThumbnail(input string, output string, size int, rotationAction RotationAction) {
	log.Printf("Create thumbnail (%d) for %s (%d)\n", size, input, rotationAction)
	if size <= 0 {
		log.Fatal("Invalid thumbnail size: ", size)
	}

	// https://github.com/libjpeg-turbo/libjpeg-turbo/issues/206#issuecomment-357151653
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	file, err := os.Open(input)
	CheckError(err, "Can't open image file.")
	defer file.Close()

	img, err := jpeg.Decode(file, &jpeg.DecoderOptions{ScaleTarget: image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: size, Y: size},
	}})
	CheckError(err, "Can't decode image file.")

	img = imaging.Fit(img, size, size, imaging.Linear)

	switch rotationAction {
	case ROTATE_90:
		img = imaging.Rotate90(img)
		break
	case ROTATE_180:
		img = imaging.Rotate180(img)
		break
	case ROTATE_270:
		img = imaging.Rotate270(img)
		break
	}

	// libjpeg can't handle NRGBA
	var rgba *image.RGBA
	if nrgba, ok := img.(*image.NRGBA); ok {
		if nrgba.Opaque() {
			rgba = &image.RGBA{
				Pix:    nrgba.Pix,
				Stride: nrgba.Stride,
				Rect:   nrgba.Rect,
			}
		}
	}

	out, err := os.Create(output)
	CheckError(err, "Can't write jpeg file.")
	defer out.Close()

	w := bufio.NewWriter(out)

	if rgba == nil {
		err = jpeg.Encode(w, img, &jpeg.EncoderOptions{Quality: 75})
	} else {
		err = jpeg.Encode(w, rgba, &jpeg.EncoderOptions{Quality: 75})
	}
	CheckError(err, "Can't encode image file")
	w.Flush()
}
