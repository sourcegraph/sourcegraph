// Code generated by vfsgen; DO NOT EDIT.

// +build dist

package templates

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	pathpkg "path"
	"time"
)

// Data statically implements the virtual filesystem provided to vfsgen.
var Data = func() http.FileSystem {
	fs := vfsgen۰FS{
		"/": &vfsgen۰DirInfo{
			name:    "/",
			modTime: time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
		},
		"/styles.css": &vfsgen۰CompressedFileInfo{
			name:             "styles.css",
			modTime:          time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
			uncompressedSize: 5262,

			compressedContent: []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xc4\x58\x4b\x8f\xe3\xb8\x11\x3e\x5b\xbf\xa2\x60\x20\xe8\x07\x24\xb5\x24\xdb\xfd\x50\x5f\x16\x3b\x9b\x49\x02\xcc\x2e\x82\xe9\x9d\xdc\x29\x89\xb6\x89\xa6\x49\x85\xa4\xda\xf6\x1a\xfe\xef\x41\x51\x2f\xca\xb2\xa7\x7b\x0e\xc1\xcc\xc0\x06\x58\xfc\xea\x5d\xac\xaa\xf6\xe1\x50\xd0\x25\x13\x14\xa6\x2f\x66\xcf\xa9\x9e\x1e\x8f\xde\xdd\xad\x07\xb7\xf0\xe7\x9a\x69\xd0\x96\xb8\xa6\xd4\x40\x83\xab\x29\xb0\x94\x0a\xcc\x9a\x02\xb2\xfb\x8a\x96\xf2\x18\x70\x22\x0a\x26\x56\x50\x92\x15\xd5\xa1\x07\xb7\x77\x9e\xe7\x01\x0a\x9b\xfc\x87\x28\x46\x32\x4e\xb5\x87\xc4\x5b\x38\x78\x93\x20\xc8\x78\x45\x83\x78\x32\x99\x40\x0a\x6a\x95\x5d\x47\x3e\x3c\x26\x3e\xc4\xf3\xe8\xe6\xb9\xbb\x4f\x06\xf7\xf1\xfc\xde\x87\x24\x49\x1c\xc0\xac\x07\xc4\x0b\x1f\x62\x14\x91\xcc\x5d\xc4\xdc\x41\xc4\x4f\x3e\x24\xd1\xd3\x29\x64\xe1\x40\x9e\x50\xc0\x6c\xe1\x43\xb2\x40\x43\x10\x53\x56\xaa\xe4\xd6\xd6\x16\x94\xcc\x7c\x88\xf1\xf3\xd0\x88\x69\x20\x49\x0f\x79\x78\x40\x55\xa8\xe9\x61\x00\x99\xf5\x90\x79\xd2\x40\x16\x8b\x46\x93\x54\x44\xac\x5c\x4d\xc9\x2c\xf2\xe1\xfe\xc9\x87\x26\x28\x0d\xa0\xd7\x93\xcc\x1f\x7d\x78\xba\x47\xcf\x07\x88\x5e\x4d\xb2\xc0\xb8\xdc\xe3\x57\xd2\x7a\xb4\x52\x94\x0a\xab\x06\x1a\x94\xbd\x7e\xb4\x5f\xb5\x9c\x1a\x92\x38\x10\x34\x35\xbe\x47\x55\xf3\x01\x66\xe6\x60\x30\x28\x49\x64\xcd\x99\x35\xba\x14\x2d\xea\x34\x77\x72\x30\x8b\x71\x54\x7f\xd5\x82\x10\x93\x0c\x31\x89\x0f\x0f\x33\xfc\x34\x62\x72\x29\x79\xb0\x52\x64\x1f\xc4\x9d\x67\xf7\x3e\xa0\xfb\x0f\x4d\x84\x7b\x48\xd2\x42\x16\x8d\x98\x38\x4a\x4e\x31\xb3\x41\x55\xc4\xf3\x07\x4c\xe7\x7c\x84\xea\xd4\x91\x21\xd0\x87\x28\x8c\xc7\xe8\xe4\x32\x7a\x64\xc1\xbc\x73\xc4\x86\x2d\xe9\xea\xb2\x8e\x2d\xd9\x4f\x26\x4e\x16\x6d\xa1\x44\x6e\x5d\x1a\xba\x33\xd0\xfe\x3b\x1f\x12\xce\xc4\xeb\x09\xa4\x7b\x47\xf1\xbc\xc7\x04\x6b\xf9\x46\x95\x8b\xe9\xdf\x22\x62\x34\x11\x3a\xd0\x54\xb1\x65\x0a\xd3\x9c\xa9\xbc\xe2\x44\x05\xda\x14\x53\x1f\x02\x52\x62\x61\xeb\xbd\x36\x74\xe3\xc3\xaf\x28\xef\x77\x92\xbf\xd8\xf3\x67\x29\x8c\x0f\xd3\xf0\xe5\xf3\x1f\x2f\x7f\xd2\x9d\x09\xbe\xd2\x15\xf2\x4e\x7d\x98\xbe\x10\x01\x9f\x15\x11\x39\xd3\xb9\xb4\x84\xcf\x7f\xbc\xc0\x6f\x4c\x97\x9c\xec\xa7\x3e\x7c\x95\x99\x34\xd2\x87\xe9\x97\x2a\x67\x05\x81\x7f\x28\x22\x0a\x8a\x40\xf2\x46\x05\x53\x20\xe8\xce\x4c\x7d\xa8\x4f\x3e\xfc\x93\xf2\x37\x6a\x58\x4e\x7c\xf8\x96\x55\xc2\x54\x3e\x4c\x35\x5d\x49\x0a\x15\x43\x98\x62\x84\xfb\xd0\xbb\x62\xbd\xdf\x48\x21\x75\x49\x72\x9a\x76\x7a\x3e\x49\xa1\x25\x47\x45\xbf\x4b\x41\x72\xe9\x43\x07\xaa\xa3\x91\x11\x4d\x83\xa5\x14\x26\xd0\xec\x2f\x9a\x42\xbc\x28\x77\xcf\xdd\x05\x67\x82\x06\x6b\xca\x56\x6b\x93\x42\x1c\x2e\xda\xfa\x2d\x06\x3c\x51\xf8\x44\x37\xcf\xdd\xcd\x09\xd3\x63\xf3\x76\x48\xc1\x2a\x9d\xc2\x0c\xe5\x7b\x47\xcf\xfb\xc5\x4a\x58\x92\x9c\xc2\xc1\x03\x68\x4e\x1b\xc6\xf7\x29\x5c\xb9\x79\xb9\x7a\xf6\x00\xb4\xca\x53\xa8\x14\xbf\xbe\x3a\x1c\x88\xd6\xd4\x7c\xfb\xfa\x05\xa6\x77\xc8\xa5\xef\x5c\x74\xa0\xab\x4c\x53\x63\x68\x71\x87\x86\x18\x19\x74\xb7\x99\x94\xaf\xe1\x56\x2e\x97\xd3\xe3\xf1\xea\x06\x1b\xff\x86\x98\xeb\x2b\xa4\x5c\xdd\x3c\x7b\xc7\x9f\x64\xd2\xbf\x0c\xe1\x2c\xff\x8e\x61\x93\x3a\xd6\x38\xae\x52\x90\x19\x67\xff\xad\xe8\xcf\x32\x97\x17\xef\x1a\xba\x6d\x52\x8f\xe0\x9f\x67\xe6\x07\xa3\x3a\x34\xf6\x52\xa4\x9b\x45\xe2\x93\xe4\x52\x35\xeb\x40\x58\x0f\x7d\x38\x40\x8e\xd4\x14\xde\x88\xba\x6e\x57\x81\x9b\x67\x38\x36\x88\xe4\x2c\x22\x71\x10\xb3\xb3\x88\x99\x45\x78\x61\x3b\xb1\x4f\x41\x2d\xbd\x16\xd4\x0e\xed\x0b\xa8\x64\x80\x1a\x29\x6c\xe9\x8d\xca\xed\x9a\x19\xda\x63\xec\xb1\xbe\x71\x26\xd7\xa9\x0c\xe7\xaa\x56\xe6\x8c\xb0\xcb\xd8\xe4\x14\x3b\xb2\xcd\xb9\x3a\xc5\xce\x2f\x63\xe7\x8d\x2b\xd9\x2a\xe8\xf2\x94\x91\xfc\x75\xa5\x64\x25\x8a\xe0\x72\xca\x1a\x7c\xf2\x1e\x3e\x19\xe2\x67\xef\xe1\x67\xbd\x41\x6d\x78\xc7\xf8\x2e\xd2\x08\x43\x3f\xbe\x23\x15\xaf\x6b\x99\xcd\x92\xbb\x2f\xe5\x4a\x91\x72\xbd\xaf\x0b\x34\x93\xc5\x1e\xb7\xd3\x01\x13\x8e\xd9\xae\xf4\xdb\x37\x58\x5f\xf5\xb3\xa4\xef\x38\xb6\xbb\x37\x4e\x0c\xe6\x04\x42\x06\x6d\xde\x01\x39\x74\x3b\x94\xb7\x34\x7b\x65\xa6\xe1\xdd\x48\x69\xd6\x4c\xac\x52\x20\xc2\x30\xc2\x19\xd1\xd4\x36\x09\xaf\x54\xd4\xf7\x70\x7e\xf8\x98\xe2\x02\x7b\xc5\x39\x33\xbb\xf1\x75\xd6\xca\xe1\x64\xba\x60\xe5\xe9\x90\xfa\xae\x95\x95\x91\xd6\x3c\x32\x8a\x25\xee\x06\xc8\x8a\x31\x0d\x0a\x9a\x4b\x45\x0c\x93\x22\x05\x21\x05\x75\x44\x1a\x45\x84\x66\xf5\x15\xe1\x1c\xa2\x70\xa6\xf1\x7a\x23\xff\xba\x74\x77\x9e\x8c\x56\xa4\xf5\x72\x73\xce\x96\x7a\xef\xb1\x73\xcc\x0b\x2d\x41\x57\x99\xe1\xd4\x01\x33\xb1\xa6\x8a\x99\x73\x46\x57\xa2\xa0\x0a\x83\x32\xe2\x6f\x55\xc2\x79\x47\xb1\x02\x4b\x38\xc0\x86\xa8\x15\x13\x41\x26\x8d\x91\x1b\xbb\x2c\x28\xba\xb1\x25\xcf\xd7\x11\x1c\x60\x90\x88\xc8\x29\xdc\x6f\x86\x71\x66\x18\xd5\x75\xdd\x86\x99\x54\x05\x55\xc3\x4d\x14\xdf\x41\x4b\x3e\xdf\x1b\xba\x07\x79\xca\x3d\x7f\x97\xb7\xed\x15\x7a\x4d\x0a\xb9\xc5\x70\x65\x72\x17\xd4\xa7\x14\x22\x88\xcb\x1d\xdc\x97\x3b\x88\xce\x68\xb5\x6b\xf3\xdd\x6d\x9b\x6b\x87\x11\x33\xac\x5d\x02\x7a\x67\xd5\x94\x4c\xb4\x31\x9d\x10\xc1\x36\x4d\x34\x91\x0e\x51\xb8\xd0\x40\xf1\x19\x31\x11\xc8\xca\x40\x6c\x6d\xfb\xe5\x95\xee\x97\x8a\x6c\xa8\xae\x61\x07\x6f\xb2\x88\xfe\x86\x39\xc1\x4a\xc1\xc9\x96\x82\x92\x86\x18\x7a\x1d\x3f\x46\x05\x5d\xdd\x80\xce\x09\xa7\xd7\x71\x98\xdc\xc0\xd1\x9b\xc4\xd1\xc7\xe0\x08\xee\x32\xf3\x77\x4e\x37\x54\x98\x26\x31\x77\xb7\xf0\x85\x69\xa3\xed\x81\xb3\x2e\xe5\x29\xc4\x98\xe9\x36\xa5\xf0\x6b\x65\x8c\x14\xba\x49\xa6\x11\x75\x40\x6d\x02\xda\x1d\xb0\x8e\x63\x7d\xc2\x00\x0e\xda\x9f\x37\x29\xea\xb5\x19\xeb\xd5\x56\x4d\xc6\x65\xfe\x7a\x61\x60\x97\xa4\x28\xec\x4b\x7d\x2c\x77\x10\xdf\xd7\xcb\xeb\xff\xe3\xe5\xd5\xae\x4c\xc6\x33\x31\x33\xc2\xb6\xf8\x8f\x4d\x80\x0e\xde\x3d\xab\x0f\x8c\x19\x0c\xeb\xbf\x15\x85\x3b\xf8\xad\xda\x64\xf0\x09\xdb\xa3\x16\xac\x2c\xa9\xb1\x61\x2e\x95\x7d\xe5\xbd\xa4\x3e\x96\xef\x44\xbe\xbe\x4e\x6d\x8d\x6b\xc9\x59\x71\xee\x75\x9c\x34\x9b\x76\x5e\x7a\x93\xb3\x6f\xbe\xa7\x1b\x59\x3a\xc4\x2e\x53\xb1\x3d\xd7\x5e\x8d\x5c\x09\xdb\xc3\xc1\x9b\x60\x80\x96\x5c\x6e\x83\x5d\xdb\x88\x01\xdc\x07\xc5\x84\xa6\x06\x82\xd8\xbe\xcd\x79\xb9\xab\xff\x58\x8d\x7c\xfb\x3f\x8c\x17\x6e\x63\xff\x61\x3e\x2c\x92\x1f\x64\x3a\xf6\xe6\xfb\xbd\x23\x61\xc9\xc5\xe0\x58\x0d\x8e\xb2\x1c\x1c\x73\x2e\x47\xdd\xbd\x0b\xb8\xa3\x00\x08\x1c\x2e\x37\x64\x47\x9e\xdc\x8c\xe4\x0d\x56\xaa\xe1\xce\xcb\xec\xd6\x3c\xd4\x14\x72\x36\xf0\x47\x9b\xf1\x00\x6a\x7f\xcc\x39\x35\xd2\xe5\x7b\xdd\x16\xee\xd1\x90\x95\x7b\x5c\x56\x62\x24\xb5\xdf\x5e\x87\x06\x11\x33\x88\x99\xd9\x97\xe7\x63\x36\x62\x2c\x68\xee\x32\xbe\x11\xe5\x0f\xc4\xbe\x8d\xe4\x34\xbf\x0b\x35\x82\xb6\x8a\x58\x55\x5b\xa9\x8a\x00\x0f\x29\x64\x8a\x92\xd7\x00\x09\x16\x72\x38\x50\x51\x1c\x8f\xde\xff\x02\x00\x00\xff\xff\x31\x76\xd8\x1c\x8e\x14\x00\x00"),
		},
		"/ui": &vfsgen۰DirInfo{
			name:    "ui",
			modTime: time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
		},
		"/ui/app.html": &vfsgen۰CompressedFileInfo{
			name:             "app.html",
			modTime:          time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
			uncompressedSize: 2476,

			compressedContent: []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x94\x56\xcd\x73\xdb\xb6\x12\x3f\x8b\x7f\xc5\x86\x17\x4b\x79\x22\xe9\xbc\xd3\x1b\x4b\xd4\x6b\x9d\x64\xfa\x31\x49\x0f\xb5\x7b\xe8\x64\x72\x58\x01\x2b\x12\x16\x09\x30\xc0\x4a\xb2\x86\xe1\xff\xde\x01\x60\x59\x52\xaa\xa4\xcd\xc5\x22\x16\xbf\xfd\xfa\xed\x07\x3c\x7f\x21\x8d\xe0\x7d\x47\x50\x73\xdb\x2c\x92\xb9\xff\x81\x06\x75\x55\xa6\xa4\x53\x10\x0d\x3a\x57\xa6\x4b\x74\x94\x2e\x92\x64\x5e\x13\xca\x45\x32\xea\xfb\xfc\x17\xfd\x40\x82\x49\xe6\x3f\x13\xca\x7b\xd3\x0d\x43\x32\x9a\xb7\xc4\x08\xa2\x46\xeb\x88\xcb\x74\xc3\xab\xec\x7f\xe9\xe2\x20\xaf\x99\xbb\x8c\x3e\x6d\xd4\xb6\x4c\x1f\xb3\x0d\x66\xc2\xb4\x1d\xb2\x5a\x36\x94\x82\x30\x9a\x49\x73\x99\x2a\x2a\x49\x56\x74\x54\xd3\xd8\x52\x99\x56\xc6\x54\x67\x38\x6d\xd8\xa2\x76\x0d\x32\x5d\x76\xf1\x3a\x22\xb3\x77\xa8\xab\x0d\x56\xa7\xba\xa4\xd3\x45\x02\x00\x70\xea\x61\xab\x68\xd7\x19\xcb\x27\xb8\x9d\x92\x5c\x97\x92\xb6\x4a\x50\x16\x0e\x53\x38\xc0\xb2\x95\xe2\x52\x98\x2d\xd9\x14\x8a\x40\xc9\x4e\x71\x0d\xf9\x7b\x62\x94\xc8\x78\xe4\x23\x98\xbf\x92\xe4\x84\x55\x1d\x2b\xa3\xaf\x8e\x1e\xfa\x3e\x7f\x73\xbc\x18\x86\x68\xeb\x54\x8f\x77\x8a\x99\xec\x0d\x2b\x6e\xe8\x5c\xf3\xde\x8b\xbe\xa5\x23\xd0\xca\xa3\xca\x95\xdb\xb4\x2d\xda\xfd\xd5\x57\xf1\xdf\x1f\x63\x67\x4d\x47\x96\xf7\x65\x6a\xaa\x1b\xdf\x47\xa7\xec\xd1\xd2\x29\xa6\x8b\xf0\x2b\x0f\xff\x57\x19\x9d\xa9\x7c\x4f\x80\x7d\x4f\x5a\x86\x2a\x04\x47\x8b\xa3\xf9\x79\x11\x25\xc9\x68\xde\x28\xbd\x06\x4b\x4d\x99\x3a\xde\x37\xe4\x6a\x22\x4e\xa1\xb6\xb4\x0a\x66\x7f\x74\x8e\xf8\x8f\xdf\xdf\x0d\x43\x11\xef\xe3\x4f\xbe\xdc\x68\xd9\x50\x2e\x9c\xfb\x7f\xdf\x6f\xc9\x3a\x65\x34\xa4\x5f\x81\xa4\xc3\x90\x3e\xfb\x52\xb2\xbc\x72\x66\x63\x05\x55\x16\xbb\x3a\x13\xb5\x35\x2d\x65\x9e\x2b\x36\x96\x32\xc5\xd4\x5e\xc5\x88\x2e\x5d\x1d\x62\xf3\x8d\xee\x6e\x8a\x22\x62\xf2\x38\x1d\xb9\x30\x6d\x71\x80\x17\x92\x18\x55\x53\xc8\xea\xa1\x5e\x99\xf6\x41\x11\xa2\xec\x4c\xf3\xd0\x68\x25\xdb\x65\xb5\x96\xab\x55\x87\x62\x9d\x7e\x41\x03\xa1\x15\xf5\xc1\x4d\x61\x3a\xd2\x51\x94\x3f\xb6\x4d\x0a\xbe\xc4\x65\x8a\x5d\xd7\x28\x81\x9e\xed\x13\xc4\x49\x71\xfe\x13\xc1\x9e\xe5\x32\xbd\x3b\xa6\x0b\x77\xd1\x7c\x28\x8f\x5a\xc1\xd3\x16\x39\x41\xdc\x5b\x14\x6b\xb2\xa1\x6e\xd1\x1c\xa8\x4a\xfb\xf4\x85\xeb\x16\xc9\x68\x34\x83\xf1\x6a\xa3\x85\xf7\x02\x63\x9e\x82\x9d\x02\x4e\x41\x4c\x61\x3d\x05\x9a\x82\x9c\x40\x0f\x6a\x05\xe3\x17\xfc\x61\xfd\xd1\x1f\x38\xff\xa9\x31\x4b\x6c\xee\xa9\x69\x54\x45\x9a\x7f\xc3\x96\x5c\x87\x82\xa0\xfc\xd6\xe5\xe7\xcf\xf0\xe1\xe3\xec\x1b\x88\xbc\xdb\xb8\x7a\xbc\x9e\xcc\xc0\xbb\x82\x12\x8e\x71\x79\xbf\x63\x2f\xcd\x3f\x79\x27\xf1\x23\x18\x9c\x44\x2d\xb4\xd5\xa6\x25\xcd\x6e\x02\xc3\x0c\x2e\x22\x67\xe0\x03\xb4\xb9\xb0\x84\x4c\x6f\x1b\xf2\xf8\x31\x4e\x66\x20\x83\xbc\x22\x7e\x12\xba\xdb\xfd\x3d\x56\x3e\xae\x31\x4e\x3e\x5c\x7b\xcd\x1c\xdd\x5e\x0b\x28\xe1\x95\x3f\x38\xeb\x3f\xc5\x0c\x64\xde\xa1\xf5\x39\x18\x49\xb9\xd2\x8e\x2c\xdf\xd2\xca\x58\x1a\x47\xea\x06\x18\xc6\x3b\xa5\xa5\xd9\x4d\x41\x1a\x11\x42\x9c\x42\x1a\x0b\x91\x4e\xe1\xb9\xef\x7c\x8b\x61\x75\x68\x3c\xec\x94\x0b\xcd\xc7\x07\x92\x32\xb4\xac\x56\x28\xd8\x15\x1c\x2b\xea\xef\xf2\x07\xe7\x8d\x3c\xa3\xd2\xc9\x64\x96\x8c\xe6\x45\xb4\x7f\x36\xb3\x17\x6b\x1f\x43\xcb\xc3\xe0\x3f\x32\x94\xd0\xf7\xf9\xeb\xa7\x83\xd7\x3a\x00\x3a\xac\xe8\xad\xb5\xc6\x46\x48\xfc\x0c\x66\x4f\x5d\x9d\x3f\x61\xb7\x86\xd9\xb4\xc3\x90\xcc\x8b\xf8\xc8\x25\xf3\xa5\x91\xfb\x2f\x90\xb7\x46\xee\x0f\x8f\x9d\x54\x5b\x3f\xce\xa9\x35\x86\xd3\xc5\xbc\x90\x6a\xeb\x67\x49\x9b\x27\x1f\x7f\x9a\x0d\x68\x22\x09\x6c\x80\x34\x2e\x1b\x82\x5f\x71\x8b\x77\x31\x33\x36\x60\x37\x1a\xb8\x56\x0e\xb0\xeb\xf2\x79\xf1\xac\x78\x4c\xdf\x59\xf1\xb7\x3d\x14\x6e\x5c\xe1\x75\x9e\x76\xcc\xc3\xf9\x16\xba\x04\x08\x3b\xe8\x2b\xd9\xfb\x9c\x4e\xb2\x8f\x59\x27\x7d\x18\xa3\x03\xbd\xf9\xc9\x88\xbe\x31\xfc\xda\xb4\xef\x8d\x24\xcf\xe9\x3c\xac\xbb\xf3\x42\x15\x2f\xe1\x6e\xd3\xf9\x57\x12\x76\xb4\x5c\x2b\x06\xdf\x6b\x0c\xf1\x25\x75\xf0\xb2\x48\x46\x3f\xb8\x88\x70\x30\xee\x50\x4a\xa5\xab\x1b\x68\xf1\x71\x7c\xdd\x3d\x4e\x26\xd0\x27\xc9\x68\x94\x57\x61\xf2\x32\x8d\xdb\x25\xda\xa9\x97\xb4\xa8\x74\xe6\xcb\x0b\xb9\x35\xbb\x20\x8a\xcb\x27\xb3\x66\x07\x7d\x32\x1a\x8d\x9e\xac\x65\x0d\xad\x38\x9a\x7c\xf5\xdf\xee\x71\x0a\xa4\xb7\x63\x87\x2b\xca\xd0\x12\x66\x21\xa0\x80\x09\x3d\x78\x54\xb3\xaa\xaa\xff\x51\x2f\x80\xa2\xe2\x10\x22\x6d\x8d\xc4\x26\xf3\xbb\x30\x04\x16\xff\x5c\x08\xe7\xfa\x92\xaf\xeb\x83\x1d\x4f\x7f\xa0\x73\xe1\xe9\x27\x2d\x3d\xc1\xbe\x21\xc3\xff\x66\x7f\x05\x00\x00\xff\xff\x0c\xed\x08\x9e\xac\x09\x00\x00"),
		},
		"/ui/error.html": &vfsgen۰CompressedFileInfo{
			name:             "error.html",
			modTime:          time.Date(2018, 1, 1, 1, 1, 1, 1, time.UTC),
			uncompressedSize: 1214,

			compressedContent: []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x74\x54\xc1\x6e\xe3\x46\x0c\x3d\x4b\x5f\xc1\xf5\xa5\x6d\xe0\x48\xe8\xad\x08\x26\x46\x17\xbb\x3d\xf8\xd0\xed\xa2\xf1\xa5\x47\x4a\x43\x59\x83\x8c\x66\x54\x92\xb2\x23\x18\xfa\xf7\x62\x46\x59\x3b\x1b\x74\x4f\xc6\x50\x7c\x7c\x8f\xef\x11\xbe\x5c\xea\xbb\xb2\xd8\xff\xf9\xf5\xaf\xbf\x0f\x1f\xbf\x1c\x3e\xc0\xa1\x27\x26\x40\x26\xd0\x73\x04\x62\x8e\x0c\x4a\xc3\xe8\x51\x49\x1e\xca\xb2\xb8\x5f\x8b\x55\xaf\x83\x4f\xaf\x0e\x43\x3b\xbf\x29\x95\xc5\xed\x01\x4e\xc0\x92\xb8\x63\x20\x0b\x1a\xa1\x21\x40\x81\xc1\x05\x37\xa0\x07\x0c\x16\x98\xbc\xc3\xc6\xe7\xfa\x18\x45\x5c\xe3\xa9\x82\xbd\x82\x8d\x24\x65\x11\xa2\x82\x0b\xad\x9f\x2c\xc1\xc7\x2f\xff\x40\xd4\x9e\xde\xe8\xd9\x26\x98\xf6\x34\xc3\x80\x33\x30\xfd\x3b\x39\x26\xb0\xa8\x08\xda\xa3\x26\xfa\x10\xb5\x2c\x46\x26\xa1\xa0\xf0\x33\x55\xc7\x0a\xee\x3e\xc5\x61\x88\x21\xf7\x6d\xe1\xdc\xbb\xb6\xcf\xf8\x44\x46\x2f\x4e\x14\x5c\x07\xb9\xf3\xe8\x54\x88\x4f\xc4\x10\x79\xe5\x2e\x8b\xa4\x98\x2c\x4c\x63\x0c\xb0\x7e\x94\x6c\x97\xa5\x23\xa3\x25\xfb\x4b\x05\x87\xde\xc9\x55\x24\x48\x1f\x27\x9f\x36\x0d\x96\x18\xe8\x44\x01\xce\x3d\x85\xb2\xd0\x9e\x40\x66\x51\x1a\x92\x50\xa1\x13\x31\xf9\x19\x1a\x8e\xcf\x14\xaa\xb2\x2c\xde\x59\xfb\x3f\x6e\x0e\x91\x09\x26\x21\x86\x8e\x1d\x05\xeb\xe7\x6d\xb6\xf5\x4c\x80\xfe\x8c\xb3\x80\xf2\x0c\x1a\x93\xec\x4c\xef\x14\x3a\xc7\xa2\xd9\xe3\x49\x48\x56\xb7\x3a\x8e\xc3\xd5\x17\xd2\x76\x9d\xd2\x62\xb8\xba\xff\xba\xfd\xd5\x7a\xf0\xee\x99\xa0\x41\xa1\xac\xad\x82\x7d\x48\x49\xe4\xfd\xf4\xd5\x7d\x85\x0e\x9d\x97\xa4\x75\xa5\xdf\x7e\x77\x3b\x2e\x67\x07\x1d\x7a\xdf\x60\xfb\x7c\x8d\x6c\x12\xb2\x55\x79\x57\x2f\x4b\x69\x3e\xd8\xd8\xea\x3c\x12\x24\xc8\xae\x34\xd9\x07\x8f\xe1\xf8\xb8\xa1\xb0\x49\x05\x42\xbb\x2b\x0b\x33\x90\x22\xb4\x3d\xb2\x90\x3e\x6e\x26\xed\xee\x7f\xdb\xa4\xba\x3a\xf5\xb4\xbb\x5c\xaa\x27\x45\x9d\xe4\x53\xb4\xb4\x2c\x70\x7d\x1f\xe8\x45\x97\x05\xee\xe1\x29\x4e\xdc\xa6\x08\xc7\xde\xd4\x2b\xa8\x34\xf5\x3a\xdd\x34\xd1\xce\x69\x98\x75\x27\x68\x3d\x8a\x3c\x6e\xf2\x22\x89\xa1\x30\xfd\xaf\xe9\xa7\x30\x32\x62\xf8\xf6\x59\xf2\xf4\xfb\x36\x5a\xda\xbc\x67\x37\x75\xea\xfc\x21\x46\xe9\x45\xdf\x60\x56\x85\x37\x8c\xa9\x57\xbe\xcb\xc5\x75\x50\xfd\x91\x64\x2c\x4b\x9e\x35\x32\x7d\xa7\xee\x36\xe9\xb5\xcb\xd4\x23\xd3\x8a\xa5\x60\x33\xca\xf4\x0c\x75\x1e\x3b\xee\x9e\x22\xf3\xbc\x4d\x99\x30\xfd\x24\xd0\x10\x05\x40\x18\x39\x36\x9e\x86\x0a\xbe\x7a\x42\x21\x30\x08\x3d\x53\xf7\xb8\x19\xd0\x79\x8d\x0f\x32\x8d\x63\x64\xfd\x5d\x6e\x06\x56\x6d\x1c\x36\xbb\x36\x06\xc5\x36\x5d\x99\xa9\x71\x97\x0f\xea\xdb\x31\xe5\x43\xc9\x7f\x2d\xfb\xcf\x0f\x60\x44\x39\x86\xe3\x55\xe8\xfe\x73\xde\x77\x2d\x9a\x7a\x4c\xce\xd7\xd6\x9d\x52\x20\x6b\x12\xa6\x5e\xcf\xe1\xbf\x00\x00\x00\xff\xff\x84\x89\xd8\xf0\xbe\x04\x00\x00"),
		},
	}
	fs["/"].(*vfsgen۰DirInfo).entries = []os.FileInfo{
		fs["/styles.css"].(os.FileInfo),
		fs["/ui"].(os.FileInfo),
	}
	fs["/ui"].(*vfsgen۰DirInfo).entries = []os.FileInfo{
		fs["/ui/app.html"].(os.FileInfo),
		fs["/ui/error.html"].(os.FileInfo),
	}

	return fs
}()

type vfsgen۰FS map[string]interface{}

func (fs vfsgen۰FS) Open(path string) (http.File, error) {
	path = pathpkg.Clean("/" + path)
	f, ok := fs[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}

	switch f := f.(type) {
	case *vfsgen۰CompressedFileInfo:
		gr, err := gzip.NewReader(bytes.NewReader(f.compressedContent))
		if err != nil {
			// This should never happen because we generate the gzip bytes such that they are always valid.
			panic("unexpected error reading own gzip compressed bytes: " + err.Error())
		}
		return &vfsgen۰CompressedFile{
			vfsgen۰CompressedFileInfo: f,
			gr:                        gr,
		}, nil
	case *vfsgen۰DirInfo:
		return &vfsgen۰Dir{
			vfsgen۰DirInfo: f,
		}, nil
	default:
		// This should never happen because we generate only the above types.
		panic(fmt.Sprintf("unexpected type %T", f))
	}
}

// vfsgen۰CompressedFileInfo is a static definition of a gzip compressed file.
type vfsgen۰CompressedFileInfo struct {
	name              string
	modTime           time.Time
	compressedContent []byte
	uncompressedSize  int64
}

func (f *vfsgen۰CompressedFileInfo) Readdir(count int) ([]os.FileInfo, error) {
	return nil, fmt.Errorf("cannot Readdir from file %s", f.name)
}
func (f *vfsgen۰CompressedFileInfo) Stat() (os.FileInfo, error) { return f, nil }

func (f *vfsgen۰CompressedFileInfo) GzipBytes() []byte {
	return f.compressedContent
}

func (f *vfsgen۰CompressedFileInfo) Name() string       { return f.name }
func (f *vfsgen۰CompressedFileInfo) Size() int64        { return f.uncompressedSize }
func (f *vfsgen۰CompressedFileInfo) Mode() os.FileMode  { return 0444 }
func (f *vfsgen۰CompressedFileInfo) ModTime() time.Time { return f.modTime }
func (f *vfsgen۰CompressedFileInfo) IsDir() bool        { return false }
func (f *vfsgen۰CompressedFileInfo) Sys() interface{}   { return nil }

// vfsgen۰CompressedFile is an opened compressedFile instance.
type vfsgen۰CompressedFile struct {
	*vfsgen۰CompressedFileInfo
	gr      *gzip.Reader
	grPos   int64 // Actual gr uncompressed position.
	seekPos int64 // Seek uncompressed position.
}

func (f *vfsgen۰CompressedFile) Read(p []byte) (n int, err error) {
	if f.grPos > f.seekPos {
		// Rewind to beginning.
		err = f.gr.Reset(bytes.NewReader(f.compressedContent))
		if err != nil {
			return 0, err
		}
		f.grPos = 0
	}
	if f.grPos < f.seekPos {
		// Fast-forward.
		_, err = io.CopyN(ioutil.Discard, f.gr, f.seekPos-f.grPos)
		if err != nil {
			return 0, err
		}
		f.grPos = f.seekPos
	}
	n, err = f.gr.Read(p)
	f.grPos += int64(n)
	f.seekPos = f.grPos
	return n, err
}
func (f *vfsgen۰CompressedFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.seekPos = 0 + offset
	case io.SeekCurrent:
		f.seekPos += offset
	case io.SeekEnd:
		f.seekPos = f.uncompressedSize + offset
	default:
		panic(fmt.Errorf("invalid whence value: %v", whence))
	}
	return f.seekPos, nil
}
func (f *vfsgen۰CompressedFile) Close() error {
	return f.gr.Close()
}

// vfsgen۰DirInfo is a static definition of a directory.
type vfsgen۰DirInfo struct {
	name    string
	modTime time.Time
	entries []os.FileInfo
}

func (d *vfsgen۰DirInfo) Read([]byte) (int, error) {
	return 0, fmt.Errorf("cannot Read from directory %s", d.name)
}
func (d *vfsgen۰DirInfo) Close() error               { return nil }
func (d *vfsgen۰DirInfo) Stat() (os.FileInfo, error) { return d, nil }

func (d *vfsgen۰DirInfo) Name() string       { return d.name }
func (d *vfsgen۰DirInfo) Size() int64        { return 0 }
func (d *vfsgen۰DirInfo) Mode() os.FileMode  { return 0755 | os.ModeDir }
func (d *vfsgen۰DirInfo) ModTime() time.Time { return d.modTime }
func (d *vfsgen۰DirInfo) IsDir() bool        { return true }
func (d *vfsgen۰DirInfo) Sys() interface{}   { return nil }

// vfsgen۰Dir is an opened dir instance.
type vfsgen۰Dir struct {
	*vfsgen۰DirInfo
	pos int // Position within entries for Seek and Readdir.
}

func (d *vfsgen۰Dir) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		d.pos = 0
		return 0, nil
	}
	return 0, fmt.Errorf("unsupported Seek in directory %s", d.name)
}

func (d *vfsgen۰Dir) Readdir(count int) ([]os.FileInfo, error) {
	if d.pos >= len(d.entries) && count > 0 {
		return nil, io.EOF
	}
	if count <= 0 || count > len(d.entries)-d.pos {
		count = len(d.entries) - d.pos
	}
	e := d.entries[d.pos : d.pos+count]
	d.pos += count
	return e, nil
}
