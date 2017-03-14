# mfGalleryMetaCreatorGo

Replacement for https://github.com/ktt-ol/mfGalleryMetaCreator

## Build 

* Install https://github.com/Masterminds/glide 
* `glide install`
* `go build cli/makeMeta.go`

Or use the docker image to build for linux amd64. See [docker-img/README.md] for details. 

## Usage

Run the `makeMeta` binary with proper options.
```
$ ./makeMeta
Usage of ./makeMeta:
  -cc-size int
    	creates a jsonp file for the Chromecast for this thumbnail size. (default -1)
  -debug
    	activates debug logging.
  -force-update
    	ignores the existing meta.json files.
  -order string
    	exifTimeAsc,exifTimeDesc,filenameAsc,filenameDesc (default "exifTimeAsc")
  -path string
    	the path to the images (required)
  -size value
    	the bounding box of the thumbnails (required). You can use this parameter more than once.
```

### Folder name

You can easily add the date of an album by encoding the date into the folder name. This script can parse the following date schemes:
* 2015-08-27_Title_boing
* 2015-08_Title_boing
* 2015_Title_boing

You can also skip the date, of course. Every underline will be converted into space.

### Folder config
Create a `content.ini` file to change some attributes for the containing folder. The format is like this:

```ini
title=Other name
description=Some description, \
even with line break
cover=someImage.jpg
```
Important: Don't add any spaces between the equal (=) sign.

**Possible settings:**
* `title` sets the title of the album. The default is the folder name.
* `description` of the folder. The default is none.
* `cover` sets the cover image for the album. The default is the first image in the album.

# Libs

* Uses https://github.com/Masterminds/glide for dependency management
