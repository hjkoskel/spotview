package main

import (
	"fmt"
	"math"
	"time"

	"github.com/hjkoskel/gomonochromebitmap"
)

type Epd0213 struct {
	hw LowLevelInterfacing
}

//TODO constant? or are there EPD2IN9 variants?
const (
	EPD_WIDTH  = 104
	EPD_HEIGHT = 212
)

// EPD2IN9 commands
const (
	DRIVER_OUTPUT_CONTROL                byte = 0x01
	BOOSTER_SOFT_START_CONTROL           byte = 0x0C
	GATE_SCAN_START_POSITION             byte = 0x0F
	DEEP_SLEEP_MODE                      byte = 0x10
	DATA_ENTRY_MODE_SETTING              byte = 0x11
	SW_RESET                             byte = 0x12
	TEMPERATURE_SENSOR_CONTROL           byte = 0x1A
	TEMPERATURE_SENSOR_CONTROL_ANOTHER   byte = 0x18 //Not defined at original code
	MASTER_ACTIVATION                    byte = 0x20
	DISPLAY_UPDATE_CONTROL_1             byte = 0x21
	DISPLAY_UPDATE_CONTROL_2             byte = 0x22
	WRITE_RAM                            byte = 0x24
	WRITE_RAM_RED                        byte = 0x26
	WRITE_VCOM_REGISTER                  byte = 0x2C
	WRITE_LUT_REGISTER                   byte = 0x32
	SET_DUMMY_LINE_PERIOD                byte = 0x3A
	SET_GATE_TIME                        byte = 0x3B
	BORDER_WAVEFORM_CONTROL              byte = 0x3C
	SET_RAM_X_ADDRESS_START_END_POSITION byte = 0x44
	SET_RAM_Y_ADDRESS_START_END_POSITION byte = 0x45
	SET_RAM_X_ADDRESS_COUNTER            byte = 0x4E
	SET_RAM_Y_ADDRESS_COUNTER            byte = 0x4F
)

func CreateEPD0213(hw LowLevelInterfacing) (Epd0213, error) {
	return Epd0213{hw: hw}, nil
}

func (p *Epd0213) Init() error {

	err := p.hw.Reset()
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	err = p.hw.Send(DRIVER_OUTPUT_CONTROL, 0xD3, 0, 0) // Driver Output control
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	// Data Entry mode setting
	err = p.hw.Send(DATA_ENTRY_MODE_SETTING, 0x03)
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	// RAM x address start at 0; end at 0Ch(12+1)*8->104
	err = p.hw.Send(SET_RAM_X_ADDRESS_START_END_POSITION, 0, 0x0C)
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}
	// RAM y address start at 0D3h;RAM y address end at 00h;
	err = p.hw.Send(SET_RAM_Y_ADDRESS_START_END_POSITION, 0xD3, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	// Border Waveform Control
	err = p.hw.Send(BORDER_WAVEFORM_CONTROL, 0x05) // HIZ
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	err = p.hw.Send(DISPLAY_UPDATE_CONTROL_1, 0x80, 0x80) //Inverse RED RAM content
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	err = p.hw.Send(TEMPERATURE_SENSOR_CONTROL_ANOTHER, 0x80) // Temperature Sensor Control; Internal temperature sensor
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	//Load Temperature and waveform setting.
	err = p.hw.Send(DISPLAY_UPDATE_CONTROL_2, 0xB1)
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	err = p.hw.Send(MASTER_ACTIVATION)
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}

	err = p.waitIdle()
	if err != nil {
		return fmt.Errorf("Init err %v", err.Error())
	}
	return nil
}

func (p *Epd0213) setWindows(xStart uint16, yStart uint16, xEnd uint16, yEnd uint16) error {

	err := p.hw.Send(SET_RAM_X_ADDRESS_START_END_POSITION,
		byte((xStart>>3)&0xFF),
		byte((xEnd>>3)&0xFF))
	if err != nil {
		return fmt.Errorf("SetWindows err %v", err.Error())
	}

	err = p.hw.Send(SET_RAM_Y_ADDRESS_START_END_POSITION,
		byte(yStart&0xFF),
		byte((yStart>>8)&0xFF),
		byte(yEnd&0xFF),
		byte((yEnd>>8)&0xFF))
	if err != nil {
		return fmt.Errorf("SetWindows err %v", err.Error())
	}
	return nil
}

func (p *Epd0213) setCursor(xStart uint16, yStart uint16) error {
	err := p.hw.Send(SET_RAM_X_ADDRESS_COUNTER,
		byte((xStart>>3)&0xFF))
	if err != nil {
		return fmt.Errorf("SetCursor err %v", err.Error())
	}

	err = p.hw.Send(SET_RAM_Y_ADDRESS_COUNTER,
		byte(yStart&0xFF),
		byte((yStart>>8)&0xFF))
	if err != nil {
		return fmt.Errorf("SetCursor err %v", err.Error())
	}
	return nil
}

func (p *Epd0213) turnOn() error { //TODO turnOff
	err := p.hw.Send(DISPLAY_UPDATE_CONTROL_2, 0xC4)
	if err != nil {
		return fmt.Errorf("TurnOn err %v", err.Error())
	}

	err = p.hw.Send(MASTER_ACTIVATION)
	if err != nil {
		return fmt.Errorf("TurnOn err %v", err.Error())
	}

	err = p.waitIdle()
	if err != nil {
		return fmt.Errorf("TurnOn err %v", err.Error())
	}
	return nil
}

func (p *Epd0213) setLut() error { //Not understood where needed?
	//70 bytes
	err := p.hw.Send(WRITE_LUT_REGISTER, []byte{
		0xAA, 0x99, 0x10, 0x00, 0x00, 0x00, 0x00, 0x55, 0x99, 0x80, 0x00, 0x00, 0x00, 0x00, 0x8A, 0xA8,
		0x9B, 0x00, 0x00, 0x00, 0x00, 0x8A, 0xA8, 0x9B, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x0F, 0x0F, 0x0F, 0x0F, 0x02, 0x14, 0x14, 0x14, 0x14, 0x06, 0x14, 0x14, 0x0C,
		0x82, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}...)
	if err != nil {
		return fmt.Errorf("SetLut err %v", err.Error())
	}
	return nil
}

func (p *Epd0213) DeepSleep() error {
	err := p.hw.Send(DEEP_SLEEP_MODE, 0x01)
	if err != nil {
		return fmt.Errorf("DeepSleep err %v", err.Error())
	}
	return nil
}

//Clear...trivial? Just draw empty pics?
func (p *Epd0213) clear() error {

	bytesWidth := EPD_WIDTH / 8
	if EPD_WIDTH&8 != 0 {
		bytesWidth++
	}

	clearContent := make([]byte, bytesWidth)
	for i := range clearContent {
		clearContent[i] = 0xFF
	}

	err := p.setWindows(0, 0, EPD_WIDTH, EPD_HEIGHT)
	if err != nil {
		return fmt.Errorf("Clear err %v", err.Error())
	}

	//Black pixels
	for row := uint16(0); row < EPD_HEIGHT; row++ {
		err = p.setCursor(0, row)
		if err != nil {
			return fmt.Errorf("Clear err %v", err.Error())
		}
		err = p.hw.Send(WRITE_RAM, clearContent...)
		if err != nil {
			return fmt.Errorf("Clear err %v", err.Error())
		}
	}

	//Red pixels
	for row := uint16(0); row < EPD_HEIGHT; row++ {
		err = p.setCursor(0, row)
		if err != nil {
			return fmt.Errorf("Clear err %v", err.Error())
		}
		err = p.hw.Send(WRITE_RAM_RED, clearContent...)
		if err != nil {
			return fmt.Errorf("Clear err %v", err.Error())
		}
	}

	err = p.turnOn()
	if err != nil {
		return fmt.Errorf("Clear err %v", err.Error())
	}
	return nil
}

const IDLEWAITTIMEOUT_MS = 60000

func (p *Epd0213) waitIdle() error {
	//TODO do at lower level? use GPIO features?
	timeoutDur := time.Millisecond * time.Duration(IDLEWAITTIMEOUT_MS)
	tStart := time.Now()
	for time.Since(tStart) < timeoutDur {
		if p.hw.Idle() {
			return nil
		}
		time.Sleep(time.Millisecond * 100)
	}
	return fmt.Errorf("waitIdle timeout after %v", time.Since(tStart))
}

func (p *Epd0213) Draw(blackData []byte, redData []byte) error {
	if len(blackData) == 0 && len(redData) == 0 {
		return p.clear()
	}

	bytesWidth := int(math.Ceil(float64(EPD_WIDTH) / 8))
	requiredBytes := bytesWidth * EPD_HEIGHT
	if len(blackData) != 0 && len(blackData) != requiredBytes {
		return fmt.Errorf("Invalid size black data %v, required %v", len(blackData), requiredBytes)
	}
	if len(redData) != 0 && len(redData) != requiredBytes {
		return fmt.Errorf("Invalid size red data %v, required %v", len(redData), requiredBytes)
	}

	err := p.setWindows(0, 0, EPD_WIDTH, EPD_HEIGHT)
	if err != nil {
		return fmt.Errorf("Draw error %v", err.Error())
	}

	if 0 < len(blackData) {
		for y := 0; y < EPD_HEIGHT; y++ {
			err = p.setCursor(0, uint16(y))
			if err != nil {
				return fmt.Errorf("Draw error %v", err.Error())
			}
			rowdata := blackData[y*bytesWidth : (y+1)*bytesWidth]
			p.hw.Send(WRITE_RAM, rowdata...)
		}
	}

	if 0 < len(redData) {
		for y := 0; y < EPD_HEIGHT; y++ {
			err = p.setCursor(0, uint16(y))
			if err != nil {
				return fmt.Errorf("Draw error %v", err.Error())
			}
			rowdata := redData[y*bytesWidth : (y+1)*bytesWidth]
			p.hw.Send(WRITE_RAM_RED, rowdata...)
		}
	}

	err = p.turnOn()
	if err != nil {
		return err
	}

	return p.DeepSleep() //Safe way? command here
}

//ToRamFormat, converts binary bitmap to same format as display ram
func (p *Epd0213) ToRamFormat(bm gomonochromebitmap.MonoBitmap) ([]byte, error) {
	//Is valid size check
	sameSize := (bm.W == EPD_WIDTH && bm.H == EPD_HEIGHT)
	rotatedSize := (bm.W == EPD_HEIGHT && bm.H == EPD_WIDTH)
	if !sameSize && !rotatedSize {
		return nil, fmt.Errorf("Bitmap is %vx%v pixels do not match %vx%v epd0213", bm.W, bm.H, EPD_WIDTH, EPD_HEIGHT)
	}

	bytesWidth := int(math.Ceil(float64(EPD_WIDTH) / 8))
	result := make([]byte, bytesWidth*EPD_HEIGHT)

	if rotatedSize {
		//Naive solution
		for y := 0; y < bm.W; y++ {
			for x := 0; x < bm.H; x++ {
				if !bm.GetPix(bm.W-y, x) {
					result[y*bytesWidth+x/8] |= 1 << (7 - x%8)
				}
			}
		}
		return result, nil
	}

	//Naive solution
	for y := 0; y < bm.H; y++ {
		for x := 0; x < bm.W; x++ {
			if !bm.GetPix(x, y) {
				result[y*bytesWidth+x/8] |= 1 << (7 - x%8)
			}
		}
	}

	return result, nil
}
