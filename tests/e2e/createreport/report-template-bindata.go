// Code generated by go-bindata.
// sources:
// report-template.html
// DO NOT EDIT!

package main

import (
	"bytes"
	"compress/gzip"
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
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
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

var _reportTemplateHtml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xa4\x57\xdd\x6f\xdb\x36\x10\x7f\x9e\xff\x8a\x9b\xd6\xa1\x0e\x5a\xc9\x1f\xeb\x86\x42\x96\xfc\xd0\xb4\x45\x1f\xba\xb5\x58\xbc\x87\x61\xdd\x03\x2d\x9e\x65\x26\x14\x29\x90\x27\xc7\x5e\xe0\xff\x7d\xa0\x3e\x6c\x59\x56\xd2\xb4\x11\x10\x05\xe4\xdd\xef\x77\x1f\xbc\x3b\xca\xd1\x8f\x6f\x3f\x5d\x2e\xfe\xfe\xfc\x0e\xd6\x94\xc9\xf9\x20\x72\xff\x40\x32\x95\xc6\x1e\x2a\xcf\x6d\x20\xe3\xf3\x01\x00\x40\x94\x21\x31\x48\xd6\xcc\x58\xa4\xd8\xfb\x6b\xf1\xde\x7f\xed\xd5\x22\x12\x24\x71\xbe\x70\xef\x68\x54\x2d\x2a\x81\xa5\x9d\x44\xa0\x5d\x8e\xb1\x47\xb8\xa5\x51\x62\x6d\x0d\x72\x4f\x60\xb4\x26\xb8\x3b\xac\xdd\xb3\x64\xc9\x4d\x6a\x74\xa1\xb8\x9f\x68\xa9\x4d\x08\x3f\x4d\x7f\x99\x8e\x5f\x4d\x66\x27\x6a\xb5\xec\x76\x2d\x08\x8f\x92\xfd\xe0\xc8\x6d\x8b\x24\x41\x6b\xdf\x1c\xf8\x2e\x1d\xe4\xab\xd6\x38\x33\x37\xa9\x41\x54\xfd\xac\x2b\x26\xe4\xf7\x50\x1a\xe4\xf7\xb8\x79\x23\xf2\xef\xf4\x71\xd7\xcf\x98\xb3\xe4\x86\xa5\x78\xc9\x0c\xff\xc8\x76\xba\xe8\x66\x38\x35\x82\xfb\x84\x59\x2e\x19\xa1\xa3\x2c\x32\x65\x43\x98\xac\x0c\xb0\x82\xf4\xf1\x35\x3b\x87\x55\xda\x7e\xca\xf2\x10\x5e\xe7\xdb\x53\x0d\x2e\x6c\x2e\xd9\x2e\x2c\x55\xfb\x7d\x23\xb4\xf4\x24\xc7\x1e\x65\xd1\x3d\xb7\x82\xd3\x3a\x84\xc9\x78\xfc\x73\x27\x8e\x5e\xdf\x97\xda\x70\x34\xbe\x61\x5c\x14\x36\x84\x57\x5d\x79\xc6\x4c\x2a\x94\xbf\xd4\x44\x3a\x0b\xe1\xd7\xae\x3c\x67\x9c\x0b\x95\x76\x90\xed\xd0\x13\x2d\x25\xcb\xad\x58\x4a\xec\xc4\x9d\x14\xc6\xba\x63\xcd\xb5\x50\x84\xe6\xab\xf0\x0f\xc8\x9c\xad\x2e\x4b\x6f\x47\x9c\xf8\x76\x16\xf5\xbd\x49\xaa\xd2\x11\x82\xd2\xaa\x43\xe6\xda\xd8\x67\x52\xa4\x2a\x04\x89\x2b\x3a\x95\xea\x82\xa4\x50\xd8\x07\x5c\x69\x45\xbe\x15\xff\x61\x08\x93\xb3\xf4\x3d\x25\xfd\x0f\x67\x29\x64\x2b\xc2\x6e\x4b\x25\x5a\x11\x2a\x0a\xe1\xf9\x97\xf1\x78\xfa\xe6\x79\x3f\x19\x4b\x48\x6c\xf0\x61\x02\xef\xcb\x74\x3a\x99\x7a\x8f\xf5\xe6\xb2\xc2\x75\xd8\x0e\x07\x34\x86\xc9\xd9\x19\x65\x6c\xeb\xaf\x51\xa4\x6b\x0a\x61\xdc\xc9\xf6\x06\xcd\x4a\xea\xdb\x10\xd6\x82\xf3\xf6\xc8\x2a\x4f\xca\x30\x65\x05\x09\xad\xc2\x16\x09\x8c\x83\xa9\x05\x64\x16\x7d\x5d\xd0\xfd\x5d\x7a\x45\x8c\xec\xa7\x0d\x9a\x8d\xc0\x5b\xb8\x1b\xfc\x00\x0f\x77\x68\xf3\xf7\xe8\x26\x3d\x1d\x59\xd6\x22\x5f\xa0\x25\xdb\x49\x4d\xab\x6a\xb6\xbe\x64\x26\xc5\xde\x7b\xe0\x2b\x13\xfb\x49\xdc\x9d\xd1\x7d\x20\x76\x93\x3b\x7f\xaa\xd7\xbb\x2e\x75\x34\x2a\x2f\xcd\xf9\x20\x1a\x55\x97\x6f\xb4\xd4\x7c\x07\x89\x64\xd6\xc6\x9e\xbb\x30\xdd\xbd\xcc\xc5\x06\x4a\xbd\xd8\x3b\xa4\x77\x25\x71\x3b\x2b\xdf\x3e\x17\x06\x93\xea\xe4\xab\x43\x9a\xd5\x1d\x14\xc2\xe4\xb7\x7c\x3b\x83\xa6\xa0\x26\xe3\xf1\x66\xdd\x5c\xe3\x2d\xd2\xf3\x00\xbc\xb9\x0b\x14\xfe\xc4\x5c\x1b\x8a\x46\x5c\x6c\x1e\x44\xb5\x31\x6f\x19\x61\x08\x77\x77\x81\x5b\xb9\xc5\x7e\xdf\x25\xa8\xc3\x3b\x2b\xbc\xd6\xc7\x42\x94\x37\x66\xea\x61\x40\x3a\x77\x1d\xe1\x35\xe0\x56\x11\x79\xf3\xcf\xe5\x02\x1c\xa1\x2d\x6d\x7f\x3e\x4a\x9d\xf9\xfc\x1b\x88\x5b\x15\xe4\xcd\xdf\x97\x8b\x16\xf1\xfb\xa3\xf4\x5b\x89\xdb\x15\xe4\xcd\xaf\xaa\x55\x8b\xfa\xaa\x25\xbf\x97\xbb\xa7\xd6\xa0\x63\x6f\xbe\xd0\xc4\x64\x49\x0c\x24\xb2\xfa\x30\xdc\x9e\xa3\x5e\x88\x0c\x5b\xec\xad\xa3\xb9\xbb\x33\x4c\xa5\x08\xcf\x84\xe2\xb8\x7d\x09\xcf\x50\x62\xe6\xc6\x57\x18\x43\xf0\x61\xf1\xfb\xc7\x77\xd5\xda\xee\xf7\xb5\x7e\xa3\x71\xd8\x40\xc5\xf7\xfb\x41\xcd\x19\x8d\x5c\x2d\xcf\x07\x91\x4d\x8c\xc8\xa9\x32\x32\x1a\xc1\xb5\x85\x6a\x07\x48\x43\x62\x90\x11\x02\x53\xd0\x1a\x9f\xa5\xe6\x86\x99\x72\x0f\x62\xe0\x3a\x29\x9c\x9d\x20\x45\x6a\x9c\x78\xb3\xbb\x74\x69\xfd\x83\x65\x38\xf4\x5a\x58\xef\xa2\xea\xb1\x95\x36\x30\x94\x48\x20\x20\x86\xf1\x0c\x04\x44\x25\x5d\x20\x51\xa5\xb4\x9e\x81\x78\xf1\xe2\xa2\xd5\xc8\x8d\xb9\xce\xad\x1b\x43\xa1\x38\xae\x84\x42\x7e\x9c\x32\x07\xee\xeb\x8a\xfb\xba\xe6\xfe\x47\xfc\x1b\x24\x6b\x21\xb9\x41\x75\xb0\x73\x7d\x6a\xc7\x3d\x0e\xda\x24\x37\x3e\x47\x0a\xc2\x6c\x78\x7d\x71\x02\x11\x2b\x18\xd6\x90\x20\x69\x02\x0f\x84\x4a\x64\xc1\xd1\x9e\x64\xa0\x76\xdd\xbb\xe8\x9a\xad\x67\xd1\x79\x88\x35\xf1\x99\xf2\xd2\x20\xbb\x39\xd9\xdd\xf7\x0d\xdd\x73\xce\x80\x71\xfe\x6e\x83\x8a\x3e\x0a\x4b\xa8\xd0\x0c\xbd\x44\x8a\xe4\xc6\x7b\x09\xab\x42\x95\xb3\x0a\x86\x5d\xf7\x68\x2d\x6c\x15\x9b\x43\x05\xa4\xd3\x54\xe2\xd0\xab\xee\x65\xef\x34\x1d\xd5\x69\x55\xb7\x6b\x5c\x21\x15\x6e\x9b\xe2\xb8\x12\x4b\x29\x54\x3a\x3b\xcb\x60\x0d\x09\xca\x66\x0a\x32\xb6\xfd\x50\x4e\xc6\xfe\x44\xf5\xaa\x42\x0c\xaa\x90\xf2\x94\x7a\x0f\x28\x6d\xf7\x3b\xef\x61\x92\x5b\xa1\xb8\xbe\x0d\x84\x52\x68\xea\xcd\x17\xe0\xe5\x5b\x6f\x76\x5f\xbe\xeb\xba\x76\xed\xd5\x34\x54\x34\xaa\x7e\xc9\xfd\x1f\x00\x00\xff\xff\x46\x11\x73\x73\xda\x0d\x00\x00")

func reportTemplateHtmlBytes() ([]byte, error) {
	return bindataRead(
		_reportTemplateHtml,
		"report-template.html",
	)
}

func reportTemplateHtml() (*asset, error) {
	bytes, err := reportTemplateHtmlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "report-template.html", size: 3546, mode: os.FileMode(420), modTime: time.Unix(1673562223, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
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

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
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
	"report-template.html": reportTemplateHtml,
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
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
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
	"report-template.html": &bintree{reportTemplateHtml, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
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
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
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
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

