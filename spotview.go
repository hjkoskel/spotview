/*
Electricity spot price display for raspberry pi + epd0213 display
This software works by default on read-only filesystem.

check help/options with -h command line flag

One way to deploy this to use raspberry distribution like https://gokrazy.org/
gokr-packer -overwrite=/dev/sdb github.com/hjkoskel/spotview

*/
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sort"
	"time"

	_ "time/tzdata" //With embedded zoneinfo, no zoneinfo required on operational system

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
		return listed[0]
	}
	if n < 1 {
		return listed[0] - 1 //at least less than most min
	}

	return listed[len(listed)-n]
}

const (
	PRICEINCREMENT     float64 = 50
	EXPENSIVEHOURCOUNT int     = 6 //How many are "red"

	SMALLTICKPRICESTEP float64 = 10
	TICKPRICESTEP      float64 = 50

	SMALLTICKLEN int = 1
	TICKLEN      int = 4
)

type PriceView struct {
	FirstName string
	FirstData [24]float64

	LastName string
	LastData [24]float64
}

//Cents per kWh
func (p *PriceView) CreateBlackRedView(expensiveHourCount int) (gomonochromebitmap.MonoBitmap, gomonochromebitmap.MonoBitmap, error) {
	redPic := gomonochromebitmap.NewMonoBitmap(DISP_WIDTH, DISP_HEIGHT, false)
	blackPic := gomonochromebitmap.NewMonoBitmap(DISP_WIDTH, DISP_HEIGHT, false)

	//X scale
	barWidth := DISP_WIDTH / 48
	barMargin := (DISP_WIDTH % 48) / 2
	//Y scale
	_, max1 := maxArr(p.FirstData[:])
	_, max2 := maxArr(p.LastData[:])
	maxprice := math.Max(max1, max2)

	//Round to increments
	plotMax := PRICEINCREMENT * math.Ceil(maxprice/PRICEINCREMENT)

	//Title+plot+Xaxis text
	plotHeight := DISP_HEIGHT - TITLE_HEIGHT - XAXIS_HEIGHT
	yConv := float64(plotHeight) / float64(plotMax)

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
	firstExpensive := maxNvaluesOnThreshold(p.FirstData[:], expensiveHourCount)
	for h, price := range p.FirstData {
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

	lastExpensive := maxNvaluesOnThreshold(p.LastData[:], expensiveHourCount)
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

	//Yscale, small ticks
	for v := float64(0); v < plotMax; v += SMALLTICKPRICESTEP {
		tickpos := DISP_HEIGHT - XAXIS_HEIGHT - int(v*yConv)
		blackPic.Hline(0, SMALLTICKLEN, tickpos, true)
		if 0 < v {
			blackPic.Print(fmt.Sprintf("%.0f", v), tickFont, 0, 0, image.Rect(2, tickpos-2, DISP_WIDTH, DISP_HEIGHT), true, false, false, false)
		}
	}
	//Yscale, large ticks
	for v := float64(0); v < plotMax; v += TICKPRICESTEP {
		blackPic.Hline(0, TICKLEN, DISP_HEIGHT-XAXIS_HEIGHT-int(v*yConv), true)
	}

	return blackPic, redPic, nil
}

func createPngOutput(filename string, blackPic *gomonochromebitmap.MonoBitmap, redPic *gomonochromebitmap.MonoBitmap) error {
	if len(filename) == 0 {
		return nil
	}
	planar, errPlanar := gomonochromebitmap.CreatePlanarColorImage([]gomonochromebitmap.MonoBitmap{*blackPic, *redPic}, []color.Color{
		color.White, color.Black, color.RGBA{R: 255, A: 255}, color.RGBA{R: 255, A: 255}})
	if errPlanar != nil {
		return fmt.Errorf("planar color image err%v", errPlanar.Error())
	}

	out, errCreateOut := os.Create(filename)
	if errCreateOut != nil {
		return fmt.Errorf("err creating %v debugfile %v", filename, errCreateOut.Error())
	}
	errEncode := png.Encode(out, planar)
	if errEncode != nil {
		return fmt.Errorf("error png-encode debugfile %v err=%v", filename, errEncode.Error())
	}
	closeErr := out.Close()
	if closeErr != nil {
		return fmt.Errorf("closing %v error %v", filename, closeErr.Error())
	}
	return nil
}

func UpdateAndShutdownEpaper(lowLevel EPD0213LowLevel, blackPic *gomonochromebitmap.MonoBitmap, redPic *gomonochromebitmap.MonoBitmap) error {
	paper, paperCreateErr := CreateEPD0213(&lowLevel)
	if paperCreateErr != nil {
		return fmt.Errorf("error creating %v", paperCreateErr.Error())
	}

	initErr := paper.Init()
	if initErr != nil {
		return fmt.Errorf("init failed %v", initErr.Error())
	}

	defer paper.DeepSleep() //Safety if fail

	blackData, convBlackErr := paper.ToRamFormat(*blackPic)
	if convBlackErr != nil {
		return fmt.Errorf("error converting black %v", convBlackErr.Error())
	}
	redData, convRedErr := paper.ToRamFormat(*redPic)
	if convRedErr != nil {
		return fmt.Errorf("Error converting red %v", convRedErr.Error())
	}

	drawErr := paper.Draw(blackData, redData)
	if drawErr != nil {
		return fmt.Errorf("Drawing failed %v", drawErr.Error())
	}
	return nil
}

const (
	VALIDEPOCHLIMIT = 1663890653963
)

func waitClock() {
	for time.Now().UnixMilli() < VALIDEPOCHLIMIT { //Arbiratry time... not less
		fmt.Printf("waiting wall clock sync")
		time.Sleep(time.Second * 5)
	}
}

func main() {
	pOutputFileName := flag.String("o", "/tmp/spotview.png", "outputfilename (in .png) what spotview renders on screen")
	pNohw := flag.Bool("nohw", false, "e-paper is not available")
	pCacheDirName := flag.String("cache", "/tmp/vattenfallcache", "download cache dirname for downloaded price data. (prefer non-volatile location if possible)")

	pSpiName := flag.String("spi", "/dev/spidev0.0", "spi device file name")
	pReadyPinName := flag.String("pinbusy", "GPIO24", "busy pin name (pin8 BUSY on display)")
	pResetPin := flag.String("pinreset", "GPIO17", "reset pin name (pin7 RESET on display)")
	pDataModePinName := flag.String("pindc", "GPIO25", " data mode pin name (pin6 D/C on display)")

	pNumberOfExpensiveHours := flag.Int("e", 6, "number of expensive hours per 24h highlighted in red")

	flag.Parse()

	//Waiting clock. Needed in case of appliance
	waitClock()

	pw, errGet := GetPriceViewVattenfall(time.Now(), *pCacheDirName)
	if errGet != nil {
		fmt.Printf("Error getting data %v\n", errGet.Error())
		os.Exit(-1)
	}

	testBlack, testRed, genErr := pw.CreateBlackRedView(*pNumberOfExpensiveHours)
	if genErr != nil {
		fmt.Printf("Error generating view %v\n", genErr.Error())
		os.Exit(-1)
	}

	//Debug output
	errPng := createPngOutput(*pOutputFileName, &testBlack, &testRed)
	if errPng != nil {
		fmt.Printf("%v\n", errPng.Error())
		os.Exit(-1)
	}
	fmt.Printf("wrote output %v\n", *pOutputFileName)

	if !*pNohw {
		lowLevel, errLowLevel := InitEPD0213LowLevel(*pSpiName, *pReadyPinName, *pResetPin, *pDataModePinName)

		if errLowLevel != nil {
			fmt.Printf("low level init error %v\n", errLowLevel.Error())
			os.Exit(-1)
		}

		errUpdate := UpdateAndShutdownEpaper(lowLevel, &testBlack, &testRed)
		if errUpdate != nil {
			fmt.Printf("Hardware error %v\n", errUpdate.Error())
			os.Exit(-1)
		}
	}
}
