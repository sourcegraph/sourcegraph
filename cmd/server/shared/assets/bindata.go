// Code generated by go-bindata. DO NOT EDIT.
// sources:
// nginx.conf (1.718kB)
// redis-cache.conf.tmpl (283B)
// redis-store.conf.tmpl (341B)
// nginx/sourcegraph_backend.conf (193B)
// nginx/sourcegraph_http.conf (212B)
// nginx/sourcegraph_main.conf (174B)
// nginx/sourcegraph_server.conf (176B)

package assets

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("read %q: %w", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes  []byte
	info   os.FileInfo
	digest [sha256.Size]byte
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _nginxConf = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xc4\x54\xc1\x6e\xdb\x30\x0c\xbd\xeb\x2b\x1e\x90\x5e\x17\x77\x05\xb6\x15\xcd\x69\xc0\x30\xf4\xd0\x01\xc5\x9a\xc3\x76\x32\x14\x89\x89\xb5\xc6\xa2\x41\xca\x69\xbb\x22\xff\x3e\xc8\x76\x5a\xaf\x4d\x36\x74\x3d\x4c\x40\x00\x85\x7a\x7c\xe4\x23\x69\x4e\x30\xaf\x82\xc2\x71\x5c\x86\x15\x6e\xac\x62\x45\x91\xc4\x26\xf2\x58\xdc\xe1\x8a\x5b\x71\xb4\x12\xdb\x54\x53\x33\xc1\x77\x6e\xe1\x6c\x84\xf5\x3f\x5a\x4d\x48\x15\x0d\x9e\xad\xd8\x14\x38\x22\x31\xac\xf7\xf9\x17\xf2\x7f\xbb\xc6\xfc\xe2\x0a\x2c\x38\x9f\xcf\x2f\xb1\x24\x9b\x5a\x21\xcd\x54\x5f\xc9\x7a\xd4\x2c\x04\x9b\x50\xa5\xd4\xe8\x59\x51\x78\x76\x3a\xd5\x51\x4c\xc7\x75\x61\x7d\x1d\x62\x11\x57\x21\xde\x1a\x43\x22\x2c\xe5\x9a\x57\xd0\xe4\x49\x64\x66\x9a\xe0\x51\x6c\xac\x14\xd2\x0e\xa8\x69\x13\xfc\xcc\x98\x09\x3e\x31\x22\x27\x08\xd5\xbc\xa1\x29\xe6\x7d\xba\x89\x62\x52\xf0\x12\xa3\x40\x65\x6d\x43\x9c\x66\x2d\x9d\x3e\x57\xd9\xb8\x22\x2c\x28\xdd\x10\x45\x33\xc1\x86\x44\x03\x47\x85\x8d\x1e\xb5\xbd\x43\x88\x6e\xdd\x7a\x42\xa8\x1b\xe1\x0d\xd5\x1d\x67\xe2\xe7\x25\x99\x9a\x1d\xb4\xcb\xad\xd8\x1b\x74\x66\x0c\x6d\x3a\x8a\x7b\xb3\x35\x26\x97\x03\xf7\x06\x00\x94\x64\x43\x52\x26\xbe\xa6\x98\x93\xce\xd0\x6c\x9f\xe0\xea\xe3\x97\x0b\x08\xf9\x20\xe4\xb2\x46\x6d\x38\x2a\xa1\x22\xeb\x49\x14\x56\x08\xca\x35\xa5\x50\x93\x62\x6d\x65\x45\x9d\x63\x23\x7c\x7b\x57\x2e\xda\xe5\x92\xa4\xd4\xf0\x93\xb2\x11\x6f\x4f\x4e\xaf\x67\xcf\xde\x15\xbb\x73\x8a\x93\x77\xef\x9f\x20\xf4\x01\xf6\xc0\xd3\x83\x86\x04\x5f\x50\xfd\x2c\xf8\x60\xf5\x7b\xb6\xd7\x74\x20\x33\x1c\xee\xc2\x43\xf0\x21\x73\xeb\x1c\xa9\x76\x33\xd6\xd5\x3b\xdb\xda\x46\x93\x90\xad\xb1\xb0\xee\x9a\xa2\x1f\xba\xf3\x62\xa1\x83\xfb\x53\xad\x23\xb2\x41\xf5\xab\xf5\xfe\x59\xf3\x38\x8f\x5e\xe1\xd6\x8c\xe6\xed\x5f\xe5\xf5\xde\xff\x5d\xdd\x28\x8d\xa1\xa7\xf9\xac\x83\x26\x8a\xf8\x70\x7c\x7a\x3c\x7b\x34\xb2\xeb\x37\x57\x31\xd2\xfc\x38\xe5\x8d\x55\xed\xb6\xd3\x59\x51\x0c\x35\x9b\xed\x81\x29\xa5\xb2\xff\xf0\x70\xce\x9a\x70\x94\x5d\xca\x8a\x35\xfd\x05\xfd\xed\xcd\x67\x96\x1b\x2b\x9e\x7c\xbe\xe1\xa8\x47\x58\xef\xcb\xdb\x72\xb9\x7b\xca\xb7\x17\x10\x5d\x0a\x27\xc6\x91\xba\x8a\x6a\x1a\x15\xe0\x55\x7b\xe3\xf7\xc8\x87\xf7\xc7\x3e\xdc\xfe\x3d\xf2\x14\x79\x70\x9f\xec\x80\xdb\x61\x50\xb7\xe6\x57\x00\x00\x00\xff\xff\xf9\x24\x5a\xd3\xb6\x06\x00\x00")

func nginxConfBytes() ([]byte, error) {
	return bindataRead(
		_nginxConf,
		"nginx.conf",
	)
}

func nginxConf() (*asset, error) {
	bytes, err := nginxConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "nginx.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xd, 0xc1, 0xef, 0x69, 0xf8, 0xb6, 0x3c, 0x4d, 0x24, 0xd0, 0xcf, 0x89, 0x8a, 0x4e, 0x64, 0xfa, 0xa5, 0x38, 0x51, 0x89, 0x6e, 0x7f, 0x45, 0x89, 0x67, 0xcf, 0x3c, 0xe0, 0x82, 0xc7, 0x3b, 0x65}}
	return a, nil
}

var _redisCacheConfTmpl = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x44\x8f\x41\x6e\xeb\x30\x0c\x44\xf7\x3a\xc5\x00\x7f\xfb\x5d\x34\x9b\x9e\xa0\x07\xe8\x15\x14\x9b\x95\x89\x48\xa4\x40\xd2\x76\x8d\x20\x77\x2f\x94\x16\xe8\x8e\x1c\xcc\x60\xe6\x75\xb5\xc0\xfd\x8e\x97\x8f\x71\x3c\x1e\x29\xfd\x43\xae\x55\x0f\xe4\x79\x26\x77\x7c\x9a\xb6\xa1\x80\xc5\x23\xcb\x4c\x9e\xba\x69\xd0\x1c\xb4\x4c\x4d\x17\x82\xe8\x08\x55\x6e\x1c\x68\xd4\xd4\x4e\x6c\x9e\x0b\xfd\x87\x51\x6c\x26\x20\x33\x35\x1c\x2b\x09\x56\x8e\x60\x29\x3f\xee\xd4\xf2\xd7\x6f\xe0\x52\xae\x7f\xdf\xd4\xb5\xf2\x7c\x8e\xd6\x1b\x9d\x3e\x55\xdb\x46\x83\x4b\xee\xbe\x6a\x38\x54\xb0\xb0\xdf\x40\x3b\xd9\x89\xc6\xb2\x05\xa5\x85\xed\x09\xf2\xce\x36\x38\x72\xef\x24\x8b\x4a\x3d\xc7\x40\xcf\x3b\xe1\xed\x15\x97\xe7\x54\xca\x1e\xd8\xc9\xae\xea\x84\xaa\xa5\xb0\x94\x54\xb5\x54\xda\xa9\xe2\xc8\x26\x43\xf8\x0e\x00\x00\xff\xff\xcf\xa1\x0d\x7b\x1b\x01\x00\x00")

func redisCacheConfTmplBytes() ([]byte, error) {
	return bindataRead(
		_redisCacheConfTmpl,
		"redis-cache.conf.tmpl",
	)
}

func redisCacheConfTmpl() (*asset, error) {
	bytes, err := redisCacheConfTmplBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "redis-cache.conf.tmpl", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xb, 0x8e, 0x40, 0x1b, 0x22, 0xd3, 0xee, 0xd, 0xc4, 0xbb, 0x45, 0xf7, 0xa4, 0xe4, 0x80, 0x8b, 0x72, 0x6b, 0x47, 0xf9, 0x9, 0x4, 0x46, 0x79, 0xac, 0x2f, 0xf8, 0xbf, 0x9e, 0x5e, 0x67, 0xeb}}
	return a, nil
}

var _redisStoreConfTmpl = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x44\x4f\x5b\x8a\xe3\x40\x0c\xfc\xf7\x29\x0a\xf6\x37\x5e\x12\x96\xbd\xc1\x1e\x60\xaf\x20\xbb\x95\x8e\x98\x6e\xa9\x91\xda\xce\x98\x90\xbb\x0f\x9d\x0c\xcc\x9f\x54\x0f\xaa\xaa\x99\x77\x3c\x1e\xf8\xfd\x7f\x1c\xcf\xe7\x34\xfd\x02\x95\x62\x77\xd0\xba\x72\x04\xae\x6e\x75\x20\x10\x8d\x4e\xba\x72\x4c\xcd\xad\xf3\xda\x39\xcd\xd5\x12\x43\x6d\x98\x8a\x54\xe9\xa8\x5c\xcd\x0f\x6c\x41\x99\x4f\x70\xee\x9b\x2b\xd8\xdd\x1c\xf7\x1b\x2b\x6e\xd2\xbb\x68\x7e\xab\xa7\x4a\x9f\xdf\x86\x4b\x5e\x7e\xbe\xb9\x59\x91\xf5\x80\x1a\xef\xb2\x76\x31\x7d\x07\xec\x8c\xd5\xea\x88\x29\x96\xd1\x0d\x49\xe2\xe3\x04\x4a\x49\x86\x88\x4a\x39\x10\x4a\x2d\x6e\xd6\xc1\x3b\xfb\x81\xbf\xa8\xa2\x5b\xe7\x98\x92\xf8\x6b\xe7\x3f\xf1\x31\x93\x5a\x63\x4d\xa6\xe5\xc0\xc1\x31\x91\x5d\xe7\x2d\x78\xf6\xb4\xcc\xcd\x99\xea\x52\xf8\x45\x04\xed\x8c\x3f\xe7\x33\x2e\xaf\x0e\x4c\xd1\xb1\xb3\x2f\x16\x3c\x5a\x64\xd1\x3c\x15\xcb\x85\x77\x2e\xb8\x93\xeb\x00\xbe\x02\x00\x00\xff\xff\xb0\x20\x1a\x16\x55\x01\x00\x00")

func redisStoreConfTmplBytes() ([]byte, error) {
	return bindataRead(
		_redisStoreConfTmpl,
		"redis-store.conf.tmpl",
	)
}

func redisStoreConfTmpl() (*asset, error) {
	bytes, err := redisStoreConfTmplBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "redis-store.conf.tmpl", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xc3, 0xbf, 0x2b, 0xf2, 0x81, 0x6, 0x5f, 0xdd, 0x6e, 0x3f, 0x25, 0xfb, 0xf6, 0x39, 0x5d, 0x73, 0x24, 0xaa, 0xab, 0xc9, 0xc7, 0x18, 0x71, 0xf1, 0x9a, 0x88, 0x8d, 0x21, 0x55, 0x8d, 0xe0, 0xc9}}
	return a, nil
}

var _nginxSourcegraph_backendConf = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x5c\xcd\x41\x4e\x80\x30\x10\x46\xe1\x7d\x4f\xf1\x27\x6c\x4d\x53\xdc\x48\x74\xeb\x0d\xf4\x02\xa5\x1d\xca\x44\x98\x92\xe9\x54\xe0\xf6\x86\xad\xfb\x97\xef\x0d\xf8\x5e\xb9\x21\x55\x59\xb8\x80\x1b\x0a\x09\x69\x34\xca\x98\x6f\x7c\xd5\xae\x89\x8a\xc6\x63\xf5\xf8\xac\x90\x6a\xd8\x6b\xe6\xe5\x7e\x01\x1b\x4e\xde\x36\xcc\x04\xa5\x53\xd9\x8c\xc4\x0d\xa8\x82\x66\x51\xad\x1f\xfe\x3f\xcd\x92\xb6\x9e\x29\x83\x05\xb6\x12\xfa\xd1\x4c\x29\xee\x4f\x62\x74\x19\x96\xaa\x98\x63\xfa\x21\x79\xee\x6e\x80\x14\x96\xcb\x3f\x82\x73\x8d\xf4\x97\x14\xe3\xeb\x9b\x0f\x3e\xf8\xf1\x7d\x0a\x53\xf8\x70\x7f\x01\x00\x00\xff\xff\x05\x45\x7e\x49\xc1\x00\x00\x00")

func nginxSourcegraph_backendConfBytes() ([]byte, error) {
	return bindataRead(
		_nginxSourcegraph_backendConf,
		"nginx/sourcegraph_backend.conf",
	)
}

func nginxSourcegraph_backendConf() (*asset, error) {
	bytes, err := nginxSourcegraph_backendConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "nginx/sourcegraph_backend.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x7b, 0x2d, 0x35, 0xf4, 0x4f, 0xee, 0x2c, 0x1e, 0x2a, 0x41, 0xd4, 0xf4, 0xa0, 0x53, 0x8e, 0x46, 0xad, 0x74, 0xe9, 0x93, 0xaf, 0x8e, 0x6b, 0xa3, 0x7d, 0x96, 0x87, 0x8, 0x29, 0x90, 0x9c, 0x97}}
	return a, nil
}

var _nginxSourcegraph_httpConf = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x5c\x8d\xb1\x6a\xc4\x30\x10\x05\x7b\x7d\xc5\x03\xb7\xc1\x24\x45\xaa\xb4\x69\x53\x25\x90\xd2\xc8\xd2\x3b\x79\x41\xb7\x6b\xa4\x35\x27\xe7\xeb\x83\xdb\xab\x67\x98\x99\xf0\xb3\x49\x47\x32\xbd\x49\x81\x74\x14\x2a\x5b\x74\x66\xac\x27\xbe\xed\x68\x89\xa5\xc5\x7d\x9b\xf1\x69\x50\x73\xdc\x2d\xcb\xed\x7c\x81\x38\x1e\x52\x2b\x56\xa2\xf1\xd1\xc4\x9d\x1a\x26\x98\xa2\x7b\x6c\x7e\xec\xf3\x73\x5a\x34\xd5\x23\x33\x43\x14\xbe\x11\x9b\xfb\x7e\x61\xe7\xf0\xeb\xa6\x45\x74\xcc\x97\x1f\xc2\x84\x5f\x22\x45\xc5\xb1\x57\x8b\x19\x35\xb6\x42\x70\x38\xb5\x8b\x69\x0f\xa9\x0a\xd5\x97\x7b\x1c\xcb\x6a\xf9\x5c\xba\xfc\x11\x6f\xef\xaf\x5f\x1f\xe1\x3f\x00\x00\xff\xff\xfc\xc0\xaa\xd0\xd4\x00\x00\x00")

func nginxSourcegraph_httpConfBytes() ([]byte, error) {
	return bindataRead(
		_nginxSourcegraph_httpConf,
		"nginx/sourcegraph_http.conf",
	)
}

func nginxSourcegraph_httpConf() (*asset, error) {
	bytes, err := nginxSourcegraph_httpConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "nginx/sourcegraph_http.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0xf, 0xd5, 0xb, 0x20, 0x74, 0xdb, 0xe1, 0xb2, 0x81, 0x38, 0x2c, 0xcc, 0x13, 0x1f, 0x9c, 0x78, 0xa4, 0xe3, 0x3d, 0xce, 0x37, 0xe4, 0x7e, 0x1b, 0x9a, 0x7e, 0x75, 0x8f, 0x92, 0x34, 0x9, 0x82}}
	return a, nil
}

var _nginxSourcegraph_mainConf = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x5c\x8d\x31\x0e\x83\x30\x10\x04\x7b\xbf\x62\x25\xda\x88\x57\xa4\x49\x9d\x7c\xc0\xe0\xc3\x9c\x62\xf6\xd0\x71\x08\xf8\x7d\x44\x9b\x7a\x46\x33\x1d\x3e\xb3\x6e\x18\x8d\x93\x56\xe8\x86\x2a\x14\xcf\x21\x05\xc3\x85\xb7\xed\x3e\x4a\xf5\xbc\xce\x3d\x9e\x06\x5a\x60\xb1\xa2\xd3\xf5\x80\x06\x0e\x6d\x0d\x83\xc0\xe5\x70\x8d\x10\xa6\x0e\x46\x6c\x91\x3d\xf6\xb5\xff\x4f\x2b\xc7\xb6\x17\x29\x50\x22\x66\xc1\x92\x95\x37\x0e\x39\xe3\xbe\xb1\x2a\xcf\xfe\xf6\x53\xea\xf0\x62\x08\x43\x8d\xb9\xb5\x0b\x43\xcb\xfc\xa6\x5f\x00\x00\x00\xff\xff\x7d\xc8\x2e\x34\xae\x00\x00\x00")

func nginxSourcegraph_mainConfBytes() ([]byte, error) {
	return bindataRead(
		_nginxSourcegraph_mainConf,
		"nginx/sourcegraph_main.conf",
	)
}

func nginxSourcegraph_mainConf() (*asset, error) {
	bytes, err := nginxSourcegraph_mainConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "nginx/sourcegraph_main.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x2b, 0xe0, 0x54, 0x8, 0x8b, 0xc7, 0xfa, 0x50, 0x47, 0x74, 0x8f, 0x86, 0xcf, 0xef, 0xde, 0xc5, 0x8b, 0x99, 0xb0, 0xc2, 0xf9, 0x95, 0x17, 0x94, 0xc0, 0x22, 0x48, 0x47, 0x6b, 0x1b, 0x8e, 0x3c}}
	return a, nil
}

var _nginxSourcegraph_serverConf = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x5c\x8d\x3d\x8e\x83\x30\x10\x85\x7b\x9f\xe2\x49\xb4\x2b\x4e\xb1\xcd\xd6\x9b\x0b\x18\x3c\x98\x51\x9c\x37\x68\x3c\x04\xb8\x7d\x44\x9b\xfe\xfb\x19\xf0\x58\xb5\x63\x36\x2e\x5a\xa1\x1d\x55\x28\x9e\x43\x0a\xa6\x0b\xff\xb6\xfb\x2c\xd5\xf3\xb6\x8e\xf8\x35\xd0\x02\x2f\x2b\xba\x5c\x3f\xd0\xc0\xa1\xad\x61\x12\xb8\x1c\xae\x11\xc2\x34\xc0\x88\x1e\xd9\x63\xdf\xc6\xef\xb4\x72\x6e\x7b\x91\x02\x25\x62\x15\x74\xf1\xb7\xf8\x0d\x84\x9c\x71\xff\x58\x95\xe7\x78\x1b\x29\x0d\xf8\x63\x08\x43\x8d\xb9\xb5\x0b\x53\xcb\x7c\xa6\x4f\x00\x00\x00\xff\xff\x9c\x8c\x8b\x56\xb0\x00\x00\x00")

func nginxSourcegraph_serverConfBytes() ([]byte, error) {
	return bindataRead(
		_nginxSourcegraph_serverConf,
		"nginx/sourcegraph_server.conf",
	)
}

func nginxSourcegraph_serverConf() (*asset, error) {
	bytes, err := nginxSourcegraph_serverConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "nginx/sourcegraph_server.conf", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info, digest: [32]uint8{0x5, 0xe0, 0x16, 0xbe, 0xd, 0x61, 0x2d, 0xed, 0xc, 0x94, 0x64, 0xfa, 0x64, 0x3c, 0x78, 0x30, 0xb, 0x84, 0xdf, 0x75, 0x70, 0xfe, 0x5a, 0x9b, 0xd8, 0x70, 0xad, 0x81, 0xd9, 0x30, 0x73, 0x67}}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// AssetString returns the asset contents as a string (instead of a []byte).
func AssetString(name string) (string, error) {
	data, err := Asset(name)
	return string(data), err
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// MustAssetString is like AssetString but panics when Asset would return an
// error. It simplifies safe initialization of global variables.
func MustAssetString(name string) string {
	return string(MustAsset(name))
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetDigest returns the digest of the file with the given name. It returns an
// error if the asset could not be found or the digest could not be loaded.
func AssetDigest(name string) ([sha256.Size]byte, error) {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[canonicalName]; ok {
		a, err := f()
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s can't read by error: %v", name, err)
		}
		return a.digest, nil
	}
	return [sha256.Size]byte{}, fmt.Errorf("AssetDigest %s not found", name)
}

// Digests returns a map of all known files and their checksums.
func Digests() (map[string][sha256.Size]byte, error) {
	mp := make(map[string][sha256.Size]byte, len(_bindata))
	for name := range _bindata {
		a, err := _bindata[name]()
		if err != nil {
			return nil, err
		}
		mp[name] = a.digest
	}
	return mp, nil
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"nginx.conf":                     nginxConf,
	"redis-cache.conf.tmpl":          redisCacheConfTmpl,
	"redis-store.conf.tmpl":          redisStoreConfTmpl,
	"nginx/sourcegraph_backend.conf": nginxSourcegraph_backendConf,
	"nginx/sourcegraph_http.conf":    nginxSourcegraph_httpConf,
	"nginx/sourcegraph_main.conf":    nginxSourcegraph_mainConf,
	"nginx/sourcegraph_server.conf":  nginxSourcegraph_serverConf,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"},
// AssetDir("data/img") would return []string{"a.png", "b.png"},
// AssetDir("foo.txt") and AssetDir("notexist") would return an error, and
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		canonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(canonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"nginx": {nil, map[string]*bintree{
		"sourcegraph_backend.conf": {nginxSourcegraph_backendConf, map[string]*bintree{}},
		"sourcegraph_http.conf":    {nginxSourcegraph_httpConf, map[string]*bintree{}},
		"sourcegraph_main.conf":    {nginxSourcegraph_mainConf, map[string]*bintree{}},
		"sourcegraph_server.conf":  {nginxSourcegraph_serverConf, map[string]*bintree{}},
	}},
	"nginx.conf":            {nginxConf, map[string]*bintree{}},
	"redis-cache.conf.tmpl": {redisCacheConfTmpl, map[string]*bintree{}},
	"redis-store.conf.tmpl": {redisStoreConfTmpl, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory.
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	return os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
}

// RestoreAssets restores an asset under the given directory recursively.
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	canonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(canonicalName, "/")...)...)
}
