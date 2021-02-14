package main

import (
	"encoding/json"
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Config configurations for generating image file
type Config struct {
	ImgWidth                   int     `json:"imgWidth"`
	ImgHeight                  int     `json:"imgHeight"`
	LogoWidthMagnification     float64 `json:"logoWidthMagnification"`
	ElementWidthMagnification  float64 `json:"elementWidthMagnification"`
	ElementMarginMagnification float64 `json:"elementMarginMagnification"`
	FontSize                   float64 `json:"fontSize"`
	BgAlphaValue               float64 `json:"bgAlphaValue"`
	OutFormat                  string  `json:"outFormat"`
	BgImgPath                  string  `json:"bgImgPath"`
	LogoImgPath                string  `json:"logoImgPath"`
	FontBinPath                string  `json:"fontBinPath"`
	SrcDir                     string  `json:"srcDir"`
	DestDir                    string  `json:"destDir"`
}

var configFile = "config.json"
var config Config

func chkError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func loadConfig() {
	f, err := os.Open(configFile)
	chkError(err)
	defer f.Close()
	err = json.NewDecoder(f).Decode(&config)
	chkError(err)
}

func chkDestDir() {
	if _, err := os.Stat(config.DestDir); os.IsNotExist(err) {
		os.Mkdir(config.DestDir, 0777)
	}
}

func destFilePath(fileName string, fileExt string) string {
	path := (filepath.Join(config.DestDir, fileName) + "." + fileExt)
	return path
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
		case "jpeg":
			webp.Encode(&outFile, img, &webp.Options{Lossless: false, Quality: 80})
		default:
			webp.Encode(&outFile, img, &webp.Options{Lossless: true})
		}
	default:
	}
}

// Refer to https://github.com/noelyahan/mergi/blob/master/watermark.go
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

func textToImg(text string, fontSize float64, offsetY int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, config.ImgWidth, config.ImgHeight))
	ftBin, err := ioutil.ReadFile(filepath.Join(config.SrcDir, config.FontBinPath))
	chkError(err)
	ft, err := truetype.Parse(ftBin)
	chkError(err)

	d := &font.Drawer{
		Dst: img,
		Src: image.NewUniform(color.White),
		Face: truetype.NewFace(ft, &truetype.Options{
			Size: fontSize,
		}),
		Dot: fixed.Point26_6{},
	}
	d.Dot.X = fixed.Int26_6(center(int(fixed.I(img.Bounds().Dx())), int(d.MeasureString(text))))
	d.Dot.Y = fixed.Int26_6(fixed.I(offsetY + center(config.ImgHeight, offsetY)))
	d.DrawString(text)
	return img
}

func center(val0 int, val1 int) int {
	return (val0 - val1) / 2
}

func init() {
	loadConfig()
}

func main() {
	elements := flag.String("e", "", "Insert Elements")
	insText := flag.String("t", "", "Insert text")
	textPt := flag.Float64("p", config.FontSize, "Font Size")
	fileName := flag.String("o", "", "Output file name")
	flag.Parse()

	// Load background image
	bgImgFile, err := os.Open(filepath.Join(config.SrcDir, config.BgImgPath))
	chkError(err)
	defer bgImgFile.Close()

	bgImg, bgImgType, err := image.Decode(bgImgFile)
	chkError(err)
	bgImg = resize.Resize(uint(config.ImgWidth), 0, bgImg, resize.Lanczos3)
	bgImg = opacity(bgImg, config.BgAlphaValue)
	bgImgHeight := bgImg.Bounds().Dy()
	bgImg, err = cutter.Crop(bgImg, cutter.Config{
		Width:   config.ImgWidth,
		Height:  config.ImgHeight,
		Mode:    cutter.Centered,
		Options: cutter.Ratio,
	})
	chkError(err)
	bgOffsetY := center(bgImgHeight, bgImg.Bounds().Dy())

	// New image
	originPoint := image.Point{0, 0}
	imgRectangle := image.Rectangle{originPoint, bgImg.Bounds().Size()}
	outImg := image.NewRGBA(imgRectangle)
	draw.Draw(outImg, imgRectangle, bgImg, image.Point{0, bgOffsetY}, draw.Src)

	// Load logo image
	logoImgFile, err := os.Open(filepath.Join(config.SrcDir, config.LogoImgPath))
	chkError(err)
	defer logoImgFile.Close()
	logoImg, _, err := image.Decode(logoImgFile)
	chkError(err)
	logoImg = resize.Resize(uint(float64(config.ImgWidth)*config.LogoWidthMagnification), 0, logoImg, resize.Bicubic)

	// Load/Insert element images
	elementList := strings.Split(*elements, ",")
	for i, f := range elementList {
		elementList[i] = filepath.Join(config.SrcDir, f)
	}

	elementImgSize := uint(float64(config.ImgWidth) * config.ElementWidthMagnification)
	elementMargin := int(float64(config.ImgWidth) * config.ElementMarginMagnification)
	elementPointX := center(config.ImgWidth, int(elementImgSize)*len(elementList)+elementMargin)
	elementPointY := center(config.ImgHeight, int(elementImgSize))

	for i, element := range elementList {
		elementImgFile, err := os.Open(element)
		chkError(err)
		elementImg, _, _ := image.Decode(elementImgFile)
		elementWidth := elementImg.Bounds().Dx()
		elementHeight := elementImg.Bounds().Dy()
		if elementWidth >= elementHeight {
			elementImg = resize.Resize(elementImgSize, 0, elementImg, resize.Bilinear)
		} else {
			elementImg = resize.Resize(0, elementImgSize, elementImg, resize.Bilinear)
		}
		elementWidth = elementImg.Bounds().Dx()
		elementHeight = elementImg.Bounds().Dy()

		elementOffsetY := center(int(elementImgSize), elementHeight)
		elementPoint := image.Point{elementPointX, elementPointY + elementOffsetY}
		elementRectangle := image.Rectangle{elementPoint, bgImg.Bounds().Size()}
		draw.Draw(outImg, elementRectangle, elementImg, originPoint, draw.Over)
		if i != len(elementList) {
			elementPointX = elementPointX + int(elementImgSize) + elementMargin
		}
	}

	// Insert logo
	logoPoint := image.Point{center(config.ImgWidth, logoImg.Bounds().Dx()), center(elementPointY, logoImg.Bounds().Dy())}
	logoRectangle := image.Rectangle{logoPoint, bgImg.Bounds().Size()}
	draw.Draw(outImg, logoRectangle, logoImg, originPoint, draw.Over)

	// Insert text
	if len(*insText) > 0 {
		textPointY := elementPointY + int(elementImgSize)
		textImg := textToImg(*insText, *textPt, textPointY)
		textRectangle := image.Rectangle{originPoint, textImg.Bounds().Size()}
		draw.Draw(outImg, textRectangle, textImg, originPoint, draw.Over)
	}

	// Save
	if len(*fileName) == 0 {
		*fileName = "ogp"
	}
	chkDestDir()
	formatList := strings.Split(config.OutFormat, ",")
	for _, format := range formatList {
		filePath := destFilePath(*fileName, format)
		outFile, err := os.Create(filePath)
		chkError(err)
		encode(bgImgType, format, outImg, *outFile)
	}
}
