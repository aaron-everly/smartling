package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/99designs/api-sdk-go"
)

var cachePath = findCachePath()

func findCachePath() string {
	var cachePath string
	usr, err := user.Current()
	if err == nil && usr.HomeDir != "" {
		cachePath = filepath.Join(usr.HomeDir, ".smartling", "cache")
	} else {
		log.Panicln("Can't locate a cache directory")
	}

	_ = os.MkdirAll(cachePath, 0755)

	return cachePath
}

func projectFileHash(projectFilepath string) string {
	localpath := localRelativeFilePath(projectFilepath)

	file, err := os.Open(localpath)
	logAndQuitIfError(err)
	defer file.Close()

	hash := sha1.New()
	_, err = io.Copy(hash, file)
	logAndQuitIfError(err)

	_, err = hash.Write([]byte(fmt.Sprintf("%#v%#v", filetypeForProjectFile(projectFilepath), ProjectConfig.ParserConfig)))
	logAndQuitIfError(err)

	b := []byte{}
	h := hex.EncodeToString(hash.Sum(b))

	return h[:7] // truncate to 7 chars
}

func translateProjectFile(projectFilepath, locale, prefix string) (hit bool, b []byte, err error) {

	hash := projectFileHash(projectFilepath)

	cacheFilePath := filepath.Join(cachePath, fmt.Sprintf("%s.%s", hash, locale))

	// check cache
	hit, b = getCachedTranslations(cacheFilePath)
	if hit {
		return hit, b, nil
	}

	// translate
	b, err = translateViaSmartling(projectFilepath, prefix, locale)
	if err != nil {
		return
	}

	// write to cache
	err = ioutil.WriteFile(cacheFilePath, b, 0644)
	if err != nil {
		return
	}

	return
}

func getCachedTranslations(cacheFilePath string) (hit bool, b []byte) {
	if cacheFile, err := os.Open(cacheFilePath); err == nil {
		if cfStat, err := cacheFile.Stat(); err == nil {
			if time.Now().Sub(cfStat.ModTime()) < ProjectConfig.cacheMaxAge() {
				if b, err = ioutil.ReadFile(cacheFilePath); err == nil {
					return true, b // return the cached data
				}
			}
		}
	}

	return
}

func findIdenticalRemoteFileOrPush(projectFilepath, prefix string) string {
	remoteFile := projectFileRemoteName(projectFilepath, prefix)
	allRemoteFiles := getRemoteFileList()

	if allRemoteFiles.contains(remoteFile) {
		// exact file already exists remotely
		return remoteFile
	}

	for _, f := range allRemoteFiles {
		if strings.Contains(f, fmt.Sprintf("/%s/", projectFileHash(projectFilepath))) {
			// if file with the same hash exists remotely
			return f
		}
	}

	return pushProjectFile(projectFilepath, prefix)
}

func translateViaSmartling(projectFilepath, prefix, locale string) (b []byte, err error) {
	remotePath := findIdenticalRemoteFileOrPush(projectFilepath, prefix)

	b, err = client.DownloadTranslation(locale, smartling.FileDownloadRequest{
		FileURIRequest: smartling.FileURIRequest{FileURI: remotePath},
	})

	fmt.Println("Downloaded", remotePath)

	return
}
