package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/chai2010/webp"
	"github.com/nfnt/resize"
)

var configFile = "config.json"
var profilesFile = "profiles.json"
var tmpDirPath = "./tmp/"

type Config struct {
	LogoWidthMagnification  float64 `json:"logoWidthMagnification"`
	LogoMarginMagnification float64 `json:"logoMarginMagnification"`
	LogoAlphaValue          float64 `json:"logoAlphaValue"`
	UseTmpDir               bool    `json:"useTmpDir"`
	LogoImgPath             string  `json:"logoImgPath"`
}

type Profile struct {
	Size   int    `json:"size"`
	Format string `json:"format"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

func loadConfig() (*Config, error) {
	f, err := os.Open(configFile)
	if err != nil {
		fmt.Println("load config:", err)
		return nil, err
	}
	defer f.Close()

	var cfg Config
	err = json.NewDecoder(f).Decode(&cfg)
	return &cfg, err
}

func outFileName(baseName string, profile Profile, useTmpDir bool) string {
	fileName := (profile.Prefix + baseName + profile.Suffix + "." + profile.Format)
	if useTmpDir {
		return tmpDirPath + fileName
	}
	return fileName
}

func chkTmpDir() {
	if _, err := os.Stat(tmpDirPath); os.IsNotExist(err) {
		os.Mkdir(tmpDirPath, 0777)
	}
}

func encode(imgType string, outFormat string, img image.Image, outFile os.File) {
	switch outFormat {
	case "jpeg", "jpg":
		jpeg.Encode(&outFile, img, &jpeg.Options{Quality: 85})
	case "png":
		png.Encode(&outFile, img)
	case "gif":
		gif.Encode(&outFile, img, nil)
	case "webp":
		switch imgType {
		case "jpeg", "":
			webp.Encode(&outFile, img, &webp.Options{Lossless: false, Quality: 80})
		default:
			webp.Encode(&outFile, img, &webp.Options{Lossless: true})
		}
	default:
	}
}

func opacity(img image.Image, alpha float64) image.Image {
	mapValues := func(value, start1, stop1, start2, stop2 float64) int {
		return int(start2 + (stop2-start2)*((value-start1)/(stop1-start1)))
	}

	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}

	bounds := img.Bounds()
	mask := image.NewAlpha(bounds)
	for x := 0; x < bounds.Dx(); x++ {
		for y := 0; y < bounds.Dy(); y++ {
			r := mapValues(alpha, 1, 0, 0, 255)
			mask.SetAlpha(x, y, color.Alpha{uint8(255 - r)})
		}
	}

	maskImg := image.NewRGBA(bounds)
	draw.DrawMask(maskImg, bounds, img, image.ZP, mask, image.ZP, draw.Over)
	return maskImg
}

func main() {
	config, err := loadConfig()
	logoWidthMagnification := config.LogoWidthMagnification
	logoMarginMagnification := config.LogoMarginMagnification
	logoAlphaValue := config.LogoAlphaValue
	useTmpDir := config.UseTmpDir
	logoImgPath := config.LogoImgPath

	flag.Parse()
	args := flag.Args()
	if flag.NArg() < 1 {
		fmt.Print("Invalid args.")
		return
	}

	// Load convert profiles
	profiles, err := ioutil.ReadFile(profilesFile)
	profileList := make([]*Profile, 0)
	err = json.Unmarshal(profiles, &profileList)
	if err != nil {
		fmt.Println("load profiles:", err)
		return
	}

	// Original image
	originFilePath := args[0]
	fileBase := filepath.Base(originFilePath[:len(originFilePath)-len(filepath.Ext(originFilePath))])

	originFile, err := os.Open(originFilePath)
	if err != nil {
		fmt.Println("Open image:", err)
		return
	}
	defer originFile.Close()

	originImg, imgType, err := image.Decode(originFile)
	if err != nil {
		fmt.Println("Decode image:", err)
		return
	}

	// Logo image
	logoFile, err := os.Open(logoImgPath)
	if err != nil {
		fmt.Println("Open image:", err)
		return
	}
	defer logoFile.Close()

	logoImg, _, err := image.Decode(logoFile)
	if err != nil {
		fmt.Println("Decode image:", err)
		return
	}

	// Insert logo
	originPoint := image.Point{0, 0}
	originRectangle := image.Rectangle{originPoint, originImg.Bounds().Size()}
	originWidth := originRectangle.Dx()
	originHeight := originRectangle.Dy()
	var logoWidth float64
	var logoMargin float64
	if originWidth >= originHeight {
		logoWidth = float64(originWidth) * logoWidthMagnification
		logoMargin = float64(originWidth) * logoMarginMagnification
	} else {
		logoWidth = float64(originHeight) * logoWidthMagnification
		logoMargin = float64(originHeight) * logoMarginMagnification
	}
	logoImg = resize.Resize(uint(logoWidth), 0, logoImg, resize.Lanczos3)
	logoImg = opacity(logoImg, logoAlphaValue)
	logoHeight := logoImg.Bounds().Dy()
	logoPoint := image.Point{(originWidth - (int(logoWidth) + int(logoMargin))), (originHeight - (int(logoHeight) + int(logoMargin)))}
	logoRectangle := image.Rectangle{logoPoint, logoPoint.Add(logoImg.Bounds().Size())}

	// New image
	img := image.NewRGBA(originRectangle)
	draw.Draw(img, originRectangle, originImg, originPoint, draw.Src)
	draw.Draw(img, logoRectangle, logoImg, originPoint, draw.Over)

	// Prepare temp dir
	if useTmpDir {
		chkTmpDir()
	}

	for _, profile := range profileList {
		filePath := outFileName(fileBase, *profile, useTmpDir)
		outFile, err := os.Create(filePath)
		if err != nil {
			fmt.Println("create:", err)
			return
		}
		defer outFile.Close()

		var outImg image.Image
		if originWidth >= originHeight {
			outImg = resize.Resize(uint(profile.Size), 0, img, resize.Lanczos3)
		} else {
			outImg = resize.Resize(0, uint(profile.Size), img, resize.Lanczos3)
		}
		encode(imgType, profile.Format, outImg, *outFile)
	}
}
