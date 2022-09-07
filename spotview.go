/*
Electricity spot price display for raspberry pi + epd0213 display
*/
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sort"
	"time"

	"github.com/hjkoskel/gomonochromebitmap"
)

//TODO constant? or are there EPD2IN9 variants?
const (
	DISP_WIDTH   = 212
	DISP_HEIGHT  = 104
	TITLE_HEIGHT = 8
	XAXIS_HEIGHT = 7
	BARGAP       = 1
)

func maxArr(arr []float64) (int, float64) {
	result := float64(0)
	resultIndex := 0
	for i, v := range arr {
		if result < v {
			result = v
			resultIndex = i
		}
	}
	return resultIndex, result
}

func maxNvaluesOnThreshold(arr []float64, n int) float64 {
	if len(arr) == 0 {
		return 0
	}
	listed := make([]float64, len(arr))
	copy(listed, arr)
	sort.Float64s(listed)

	if len(listed) < n {
		return listed[len(listed)-1]
	}
	if n < 1 {
		return arr[0]
	}

	return listed[len(listed)-n]
}

/*
func maxNvalues(arr []float64, n int) []int {
	list := make([]float64, len(arr))
	result := make([]int, n)

	for target:=range(result){
		result[target]
	}

	for i, v := range arr {
		if len(result) < n {
			result = append(result, i)
		} else {
			for j := range result {
			}
		}
	}
	return result
}*/

const (
	PRICEINCREMENT     float64 = 50
	EXPENSIVEHOURCOUNT int     = 6 //How many are "red"

	SMALLTICKPRICESTEP float64 = 10
	TICKPRICESTEP      float64 = 50

	SMALLTICKLEN int = 2
	TICKLEN      int = 4
)

/*
func createBarPlot(values []float64, yConv float64, width int, height int) gomonochromebitmap.MonoBitmap {

}*/

type PriceView struct {
	FirstName string
	FirstData [24]float64

	LastName string
	LastData [24]float64
}

//Cents per kWh
func (p *PriceView) CreateBlackRedView() (gomonochromebitmap.MonoBitmap, gomonochromebitmap.MonoBitmap, error) {
	redPic := gomonochromebitmap.NewMonoBitmap(DISP_WIDTH, DISP_HEIGHT, false)
	blackPic := gomonochromebitmap.NewMonoBitmap(DISP_WIDTH, DISP_HEIGHT, false)

	//X scale
	barWidth := DISP_WIDTH / 48
	barMargin := (DISP_WIDTH % 48) / 2
	fmt.Printf("BarWidth %v, margin %v\n", barWidth, barMargin)
	//Y scale
	_, max1 := maxArr(p.FirstData[:])
	_, max2 := maxArr(p.LastData[:])
	maxprice := math.Max(max1, max2)

	//Round to increments
	plotMax := PRICEINCREMENT * math.Ceil(maxprice/PRICEINCREMENT)

	//Title+plot+Xaxis text
	plotHeight := DISP_HEIGHT - TITLE_HEIGHT - XAXIS_HEIGHT
	yConv := float64(plotHeight) / float64(plotMax)

	fmt.Printf("plot max %.2f, yConv=%f\n", plotMax, yConv)

	tickFont := gomonochromebitmap.GetFont_4x5()
	for n := 0; n < 48; n += 4 {
		fontOff := -1 //looks better
		if 9 < n%24 {
			fontOff = -3
		}
		blackPic.Print(fmt.Sprintf("%v", n%24), tickFont, 0, 0, image.Rect(barMargin+n*barWidth+fontOff, DISP_HEIGHT-5, DISP_WIDTH, DISP_HEIGHT), true, false, false, false)
	}

	firstDayText := fmt.Sprintf("%s %.1f c/kWh", p.FirstName, max1)
	lastDayText := fmt.Sprintf("%s %.1f c/kWh", p.LastName, max2)

	titleFont := gomonochromebitmap.GetFont_5x7()
	titleFontWidth := 5 + 1
	blackPic.Print(firstDayText, titleFont, 0, 1, image.Rect(
		DISP_WIDTH/4-titleFontWidth*len(firstDayText)/2, 0, DISP_WIDTH/2, DISP_HEIGHT), true, false, false, false)
	blackPic.Print(lastDayText, titleFont, 0, 1, image.Rect(
		DISP_WIDTH/2+DISP_WIDTH/4-titleFontWidth*len(lastDayText)/2, 0, DISP_WIDTH, DISP_HEIGHT), true, false, false, false)

	//First day
	firstExpensive := maxNvaluesOnThreshold(p.FirstData[:], 6)
	lastExpensive := maxNvaluesOnThreshold(p.LastData[:], 6)

	for h, price := range p.FirstData {
		//bar := image.Rect(barMargin+h*barWidth, int(yConv*(plotMax-price)), barMargin+(h+1)*barWidth, DISP_HEIGHT-5)
		barHeight := int(price * yConv)

		bar := image.Rect(
			barMargin+h*barWidth,
			TITLE_HEIGHT+plotHeight-barHeight,
			barMargin+(h+1)*barWidth-1-BARGAP,
			TITLE_HEIGHT+plotHeight)

		blackPic.Fill(bar, true)
		if firstExpensive <= price {
			redPic.Fill(bar, true)
		}
	}

	for h, price := range p.LastData { //BAD copy paste. TODO refactor
		barHeight := int(price * yConv)

		bar := image.Rect(
			barMargin+h*barWidth,
			TITLE_HEIGHT+plotHeight-barHeight,
			barMargin+(h+1)*barWidth-1-BARGAP,
			TITLE_HEIGHT+plotHeight)

		bar.Min.X += barWidth * 24
		bar.Max.X += barWidth * 24

		blackPic.Fill(bar, true)
		if lastExpensive <= price {
			redPic.Fill(bar, true)
		}
	}

	//Yscale
	for v := float64(0); v < plotMax; v += SMALLTICKPRICESTEP {
		blackPic.Hline(0, SMALLTICKLEN, DISP_HEIGHT-XAXIS_HEIGHT-int(v*yConv), true)
	}
	for v := float64(0); v < plotMax; v += TICKPRICESTEP {
		blackPic.Hline(0, TICKLEN, DISP_HEIGHT-XAXIS_HEIGHT-int(v*yConv), true)
	}

	return blackPic, redPic, nil
}

func main() {

	//Example data

	pw := PriceView{
		FirstName: "Ma",
		FirstData: [24]float64{30.14, 22.97, 22.98, 24.04, 26.33, 29.26, 44.77, 57.32, 63.10, 64.76, 59.67, 53.82, 48.40, 47.36, 42.90, 36.62, 47.36, 48.87, 60.68, 68.48, 68.48, 55.20, 51.14, 33.57},

		LastName: "Ti",
		LastData: [24]float64{18.64, 7.20, 7.67, 7.69, 7.56, 9.55, 21.36, 49.54, 65.39, 68.28, 58.20, 49.37, 38.05, 31.93, 33.41, 34.38, 35.75, 36.37, 35.89, 34.58, 34.86, 34.80, 37.02, 30.05},
	}

	pw, errGet := GetPriceViewVattenfall(time.Now())
	if errGet != nil {
		fmt.Printf("Error getting data %v", errGet.Error())
		os.Exit(-1)
	}

	testBlack, testRed, genErr := pw.CreateBlackRedView()
	if genErr != nil {
		fmt.Printf("Gen err %v", genErr.Error())
	}

	//Debug output
	planar, errPlanar := gomonochromebitmap.CreatePlanarColorImage([]gomonochromebitmap.MonoBitmap{testBlack, testRed}, []color.Color{
		color.White, color.Black, color.RGBA{R: 255, A: 255}, color.RGBA{R: 255, A: 255}})
	if errPlanar != nil {
		fmt.Printf("Planar err%v", errPlanar.Error())
		return
	}

	out, errCreateOut := os.Create("out.png")
	if errCreateOut != nil {
		fmt.Printf("\n\nerr %v\n", errCreateOut.Error())
		return
	}
	errEncode := png.Encode(out, planar)
	if errEncode != nil {
		fmt.Printf("\n\nError encode %v\n", errEncode.Error())
		return
	}
	closeErr := out.Close()
	if closeErr != nil {
		fmt.Printf("Closing error %v\n", closeErr.Error())
		return
	}

	fmt.Printf("\n\n----Testing with real hardware (fail on not pi) ----\n")

	lowLevel, errLowLevel := InitEPD0213LowLevel()

	if errLowLevel != nil {
		fmt.Printf("Low level init error %v", errLowLevel.Error())
		return
	}
	paper, paperCreateErr := CreateEPD0213(&lowLevel)
	if paperCreateErr != nil {
		fmt.Printf("Error creating %v", paperCreateErr.Error())
		return
	}
	fmt.Printf("Initializing...\n")
	initErr := paper.Init()
	if initErr != nil {
		fmt.Printf("Init failed %v", initErr.Error())
		return
	}

	defer paper.DeepSleep() //Safety if fail

	blackData, convBlackErr := paper.ToRamFormat(testBlack)
	if convBlackErr != nil {
		fmt.Printf("Error converting black %v", convBlackErr.Error())
		return
	}
	redData, convRedErr := paper.ToRamFormat(testRed)
	if convRedErr != nil {
		fmt.Printf("Error converting red %v", convRedErr.Error())
		return
	}
	fmt.Printf("Drawing...\n")
	drawErr := paper.Draw(blackData, redData)
	if drawErr != nil {
		fmt.Printf("Drawing failed %v", drawErr.Error())
		return
	}

}
