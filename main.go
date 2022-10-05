// Shields.io endpoint provider for app meta data from play store.
package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/bitly/go-simplejson"
	"github.com/cvzi/playshields/lru"
	"github.com/gin-gonic/gin"
)

const (
	playstoreAppURL = "https://play.google.com/store/apps/details?hl=en_US&id="
	escapeDollar    = "\xf0\x9f\x92\xb2\xf0\x9f\x92\xb2"
	appIDPattern    = "[a-zA-Z0-9_]+\\.[a-zA-Z0-9_]+(\\.[a-zA-Z0-9_]+)*"
)

// htmlCache holds the complete body of a downloaded website or an error string.
var htmlCache = lru.New(500)

// jsonCache holds the json code for a badge.
var jsonCache = lru.New(10000)

// jsCache holds the json object from the store website
var jsCache = lru.New(500)

type htmlCacheEntry struct {
	content  string
	ok       bool
	errorStr string
}

type (
	placeHolderGetter func(string, []string) (string, error)
	placeHolder       struct {
		placeHolderGetter placeHolderGetter
		description       string
	}
)

var playStorePlaceHolders map[string]placeHolder

var playStoreDescriptions map[string]string

var regExpAppID = regexp.MustCompile(appIDPattern)

func init() {
	playStorePlaceHolders = map[string]placeHolder{
		"$version":       {playStoreGetVersion, "App version"},
		"$installs":      {playStoreGetInstalls, "Installs"},
		"$size":          {playStoreGetSize, "*Defunct, returns empty string"},
		"$updated":       {playStoreGetLastUpdate, "Last update"},
		"$android":       {playStoreGetMinAndroid, "Required min. Android version"},
		"$minsdk":        {playStoreGetMinSdk, "Required min. SDK"},
		"$targetsdk":     {playStoreGetTargetSdk, "Target SDK"},
		"$targetandroid": {playStoreGetTargetAndroid, "Target Android version"},
		"$rating":        {playStoreGetRating, "Rating"},
		"$floatrating":   {playStoreGetPreciseRating, "Precise rating"},
		"$name":          {playStoreGetName, "Name"},
		"$friendly":      {playStoreGetContentRating, "Content Rating"},
		"$published":     {playStoreGetFirstPublished, "First published"},
	}

	// Holds the descriptions for the website.
	playStoreDescriptions = make(map[string]string)
	for key, value := range playStorePlaceHolders {
		playStoreDescriptions[key] = value.description
	}
}

// playStoreTable cuts out a single value from the table.
func playStoreTable(content string, key string) string {
	slices := strings.Split(strings.Split(strings.Split(content, ">"+key+"</")[1], "</span>")[0], ">")
	return slices[len(slices)-1]
}

// playStoreGetRating downloads the play store app website and cuts out the rating number.
func playStoreGetRating(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	rating := js.GetIndex(1).GetIndex(2).GetIndex(51).GetIndex(0).GetIndex(0).MustString()
	return rating, nil
}

// playStoreGetPreciseRating downloads the play store app website and cuts out the precice rating number.
func playStoreGetPreciseRating(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	ratingPrecise := js.GetIndex(1).GetIndex(2).GetIndex(51).GetIndex(0).GetIndex(1).MustFloat64()
	return fmt.Sprint(ratingPrecise), nil
}

// playStoreGetName downloads the play store app website and cuts out the app name.
func playStoreGetName(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	name := js.GetIndex(1).GetIndex(2).GetIndex(0).GetIndex(0).MustString()
	return name, nil
}

// playStoreGetInstalls downloads the play store app website and cuts out the number of installs.
func playStoreGetInstalls(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	installs := js.GetIndex(1).GetIndex(2).GetIndex(13).GetIndex(3).MustString()
	return installs, nil
}

// playStoreGetVersion downloads the play store app website and cuts out the current app version.
func playStoreGetVersion(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	version := js.GetIndex(1).GetIndex(2).GetIndex(140).GetIndex(0).GetIndex(0).GetIndex(0).MustString()
	if strings.TrimSpace(version) == "" {
		return "Varies with device", nil
	}
	return version, nil
}

// playStoreGetLastUpdate downloads the play store app website and cuts out the date of the last update
func playStoreGetLastUpdate(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	lastUpdate := js.GetIndex(1).GetIndex(2).GetIndex(145).GetIndex(0).GetIndex(0).MustString()
	return lastUpdate, nil
}

// playStoreGetMinAndroid downloads the play store app website and cuts out the minimal supported Android version
func playStoreGetMinAndroid(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	minAndroid := js.GetIndex(1).GetIndex(2).GetIndex(140).GetIndex(1).GetIndex(1).GetIndex(0).GetIndex(0).GetIndex(1).MustString()
	if strings.TrimSpace(minAndroid) == "" {
		return "Varies with device", nil
	}
	return minAndroid, nil
}

// playStoreGetMinSdk downloads the play store app website and cuts out the minimal supported Android SDK version
func playStoreGetMinSdk(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	minSdk := js.GetIndex(1).GetIndex(2).GetIndex(140).GetIndex(1).GetIndex(1).GetIndex(0).GetIndex(0).GetIndex(0).MustInt()
	if minSdk < 1 {
		return "Varies with device", nil
	}
	return fmt.Sprint(minSdk), nil
}

// playStoreGetTargetSdk downloads the play store app website and cuts out the targeted Android SDK version
func playStoreGetTargetSdk(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	targetSdk := js.GetIndex(1).GetIndex(2).GetIndex(140).GetIndex(1).GetIndex(0).GetIndex(0).GetIndex(0).MustInt()
	return fmt.Sprint(targetSdk), nil
}

// playStoreGetTargetAndroid downloads the play store app website and cuts out the targeted Android version
func playStoreGetTargetAndroid(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	targetSdk := js.GetIndex(1).GetIndex(2).GetIndex(140).GetIndex(1).GetIndex(0).GetIndex(0).GetIndex(1).MustString()
	return fmt.Sprint(targetSdk), nil
}

// playStoreGetContentRating downloads the play store app website and cuts out the content rating string
func playStoreGetContentRating(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	contentRating := js.GetIndex(1).GetIndex(2).GetIndex(9).GetIndex(0).MustString()
	return fmt.Sprint(contentRating), nil
}

// playStoreGetFirstPublished downloads the play store app website and cuts out the date of the first publication
func playStoreGetFirstPublished(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	var js *simplejson.Json
	if js, err = cachedGetJson(placeHolderGetterParams); err != nil {
		return "", fmt.Errorf("app unavailable: %s", err.Error())
	}

	firstPublished := js.GetIndex(1).GetIndex(2).GetIndex(10).GetIndex(0).MustString()
	return fmt.Sprint(firstPublished), nil
}

// playStoreGetSize defunct, returns empty string, used to return apk file size
func playStoreGetSize(placeHolderName string, placeHolderGetterParams []string) (content string, err error) {
	return "", nil
}

// cachedGetJson downloads a website and cuts out the json part, parses it and stores the result in cache.
func cachedGetJson(placeHolderGetterParams []string) (js *simplejson.Json, err error) {
	appid := placeHolderGetterParams[0]
	url := playstoreAppURL + url.QueryEscape(appid)

	if cacheEntry, ok := jsCache.Get(appid); ok {
		return cacheEntry.(*simplejson.Json), nil
	}

	content, err := cachedGetBody(url)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(content, "AF_initDataCallback({")[1:]
	var arrString string = ""
	for _, element := range parts {
		if strings.Contains(element, "[\""+appid+"\"],") {
			arrString = strings.TrimSpace(strings.Split(element, "</script>")[0])
			arrString = strings.TrimSpace(strings.Split(strings.SplitN(arrString, "data:", 2)[1], "sideChannel:")[0])
			arrString = arrString[0 : len(arrString)-1] // remove trailing comma
		}
	}

	js, err = simplejson.NewJson([]byte(arrString))
	if err != nil {
		return nil, err
	}
	jsCache.Set(appid, js)
	return js, nil
}

// cachedGetBody downloads a website and cuts out the body part and stores the result in cache.
func cachedGetBody(url string) (content string, err error) {
	if cacheEntry, ok := htmlCache.Get(url); ok {
		if cacheEntry.(htmlCacheEntry).ok {
			content = cacheEntry.(htmlCacheEntry).content
		} else {
			return "", errors.New(cacheEntry.(htmlCacheEntry).errorStr)
		}
	} else {
		resp, err := http.Get(url)
		if err != nil {
			htmlCache.Set(url, htmlCacheEntry{errorStr: err.Error(), ok: false})
			return "", err
		} else if resp.StatusCode != http.StatusOK {
			htmlCache.Set(url, htmlCacheEntry{errorStr: resp.Status, ok: false})
			return "", errors.New(resp.Status)
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			htmlCache.Set(url, htmlCacheEntry{errorStr: err.Error(), ok: false})
			return "", err
		}
		content = string(data)
		content = strings.SplitN(content, "</head>", 2)[1]
		htmlCache.Set(url, htmlCacheEntry{content: content, ok: true})
	}
	return content, nil
}

// replacePlaceHolder replaces a single $field in s with the result of f.
func replacePlaceHolder(errorArr *[]error, s string, placeHolderName string, f placeHolderGetter, placeHolderGetterParams []string) string {
	if strings.Contains(s, placeHolderName) {
		value, err := f(placeHolderName, placeHolderGetterParams)
		if err != nil {
			*errorArr = append(*errorArr, err)
			return s
		}
		if len(value) > 1000 {
			value = value[0:1000]
		}
		s = strings.ReplaceAll(s, placeHolderName, value)
	}
	return s
}

// replacePlaceHolders replaces all placeholders like $field.
func replacePlaceHolders(s string, placeHolderNames map[string]placeHolder, placeHolderGetterParams []string) (string, error) {
	errorArr := make([]error, 0)
	s = strings.ReplaceAll(s, "$$", escapeDollar)

	for key, value := range placeHolderNames {
		s = replacePlaceHolder(&errorArr, s, key, value.placeHolderGetter, placeHolderGetterParams)
	}

	s = strings.ReplaceAll(s, escapeDollar, "$$")
	_, err := combineErrors(errorArr)
	return s, err
}

// combineErrors creates a single error from a slice of errors.
func combineErrors(errorArr []error) (hasError bool, err error) {
	errN := len(errorArr)
	if errN == 0 {
		return true, nil
	}
	errStrings := make([]string, 0, errN)
	for i := 0; i < errN; i++ {
		if errorArr[i] != nil {
			errStrings = append(errStrings, errorArr[i].Error())
		}
	}
	return false, errors.New(strings.Join(errStrings, ","))
}

// errorJSON sends badge that shows the error message.
func errorJSON(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{"schemaVersion": 1, "label": "error", "message": message, "isError": true})
}

func setupRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/static/favicon.ico")
	})
	router.GET("/", func(c *gin.Context) {
		templateValues := gin.H{
			"appid":        c.DefaultQuery("appid", ""),
			"label":        c.DefaultQuery("label", "Android"),
			"message":      c.DefaultQuery("message", "$version"),
			"placeholders": playStoreDescriptions,
		}
		c.HTML(http.StatusOK, "index.tmpl.html", templateValues)
	})
	router.GET("/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"schemaVersion": 1,
			"label":         "Status",
			"message":       fmt.Sprintf("%d apps with %d badges", htmlCache.Len(), jsonCache.Len()),
			"cacheSeconds":  60,
		})
	})
	router.GET("/play", func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=10000")
		cacheKey := c.Request.URL.Path + c.Request.URL.RawQuery
		if cacheEntry, ok := jsonCache.Get(cacheKey); ok {
			c.JSON(http.StatusOK, cacheEntry)
		} else {
			appid := c.DefaultQuery("i", c.DefaultQuery("id", ""))
			if appid == "" {
				errorJSON(c, "missing app id")
				return
			}
			indices := regExpAppID.FindStringIndex(appid)
			if indices == nil || indices[0] > 0 || indices[1] < len(appid) {
				errorJSON(c, "invalid app id format")
				return
			}

			label := c.DefaultQuery("l", c.DefaultQuery("label", "play"))
			message := c.DefaultQuery("m", c.DefaultQuery("message", "$version"))

			if len(label) > 1000 {
				label = label[0:1000]
			}
			if len(message) > 1000 {
				message = message[0:1000]
			}

			message, err := replacePlaceHolders(message, playStorePlaceHolders, []string{appid})
			if err != nil {
				errorJSON(c, err.Error())
				return
			}

			label, err = replacePlaceHolders(label, playStorePlaceHolders, []string{appid})
			if err != nil {
				errorJSON(c, err.Error())
				return
			}
			cacheEntry = gin.H{"schemaVersion": 1, "label": label, "message": message, "cacheSeconds": 3600}
			c.JSON(http.StatusOK, cacheEntry)
			jsonCache.Set(cacheKey, cacheEntry)
		}
	})
	return router
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}
	remoteAddr := os.Getenv("REMOTE_ADDR")
	var trustedProxies []string
	if strings.TrimSpace(remoteAddr) != "" {
		trustedProxies = []string{remoteAddr}
		log.Printf("SetTrustedProxies: %s", remoteAddr)
	}
	router := setupRouter()
	router.SetTrustedProxies(trustedProxies)
	router.Run(":" + port)
}
