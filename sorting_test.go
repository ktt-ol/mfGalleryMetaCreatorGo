package mfGalleryMetaCreatorGo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_sorting_MetaJsonImage(t *testing.T) {
	testData := []MetaJsonImage{
		makeTestData("a", 20),
		makeTestData("b", 1),
		makeTestData("c", 5),
		makeTestData("d", 23),
		makeTestData("e", 7),
		makeTestData("f", 1),
	}

	SortImages("exifTimeAsc", testData)
	assertExif(t, testData, []int{1, 1, 5, 7, 20, 23})
	require.Equal(t, "b", testData[0].Filename)

	SortImages("filenameAsc", testData)
	assertFilename(t, testData, []string{"a", "b", "c", "d", "e", "f"})

	SortImages("exifTimeDesc", testData)
	assertExif(t, testData, []int{23, 20, 7, 5, 1, 1})
	require.Equal(t, "b", testData[5].Filename)

	SortImages("filenameDesc", testData)
	assertFilename(t, testData, []string{"f", "e", "d", "c", "b", "a"})
}

func makeTestData(filename string, time int64) MetaJsonImage {
	return MetaJsonImage{filename, 100, 200, metaJsonExif{nil, nil, &time}, 0}
}

func assertExif(t *testing.T, data []MetaJsonImage, expected []int) {
	require.Equal(t, len(data), len(expected), "not matching len")
	for i, d := range data {
		require.Equal(t, int64(expected[i]), *d.Exif.Time)
	}
}

func assertFilename(t *testing.T, data []MetaJsonImage, expected []string) {
	require.Equal(t, len(data), len(expected), "not matching len")
	for i, d := range data {
		require.Equal(t, expected[i], d.Filename)
	}
}
