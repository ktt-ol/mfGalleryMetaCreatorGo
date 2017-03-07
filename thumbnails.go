package mfGalleryMetaCreatorGo

import (
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"path"

	"runtime"

	"github.com/disintegration/imaging"
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
func UpdateThumbnails(folder *FolderContent, sizeList IntList) {
	maxWorker := runtime.GOMAXPROCS(runtime.NumCPU())

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

	file, err := os.Open(input)
	CheckError(err, "Can't open image file.")
	defer file.Close()

	// decode jpeg into image.Image
	img, err := jpeg.Decode(file)
	CheckError(err, "Can't decode image file.")

	thumbnail := imaging.Fit(img, size, size, imaging.Linear)

	switch rotationAction {
	case ROTATE_90:
		thumbnail = imaging.Rotate90(thumbnail)
		break
	case ROTATE_180:
		thumbnail = imaging.Rotate180(thumbnail)
		break
	case ROTATE_270:
		thumbnail = imaging.Rotate270(thumbnail)
		break
	}

	out, err := os.Create(output)
	CheckError(err, "Can't write jpeg file.")
	defer out.Close()

	jpeg.Encode(out, thumbnail, nil)
}
