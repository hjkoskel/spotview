/*
capsulates all SPI and gpio pin operations
*/

package main

import (
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

type LowLevelInterfacing interface {
	Send(command byte, pars ...byte) error //Command and data
	Reset() error
	Idle() bool //true if is idle
}

type EPD0213LowLevel struct {
	readyPin    gpio.PinIO //BCM24
	resetPin    gpio.PinIO //BCM17
	dataModePin gpio.PinIO //BCM25
	spiConn     spi.Conn   //*spi.Device
}

func InitEPD0213LowLevel(spiDeviceFileName string, readyPinName string, resetPinName string, dataModePinName string) (EPD0213LowLevel, error) {
	if !fileExists(spiDeviceFileName) {
		return EPD0213LowLevel{}, fmt.Errorf("SPI device file %v is missing", spiDeviceFileName)
	}

	_, err := host.Init()

	if err != nil {
		return EPD0213LowLevel{}, err
	}

	p, errSpi := spireg.Open(spiDeviceFileName)
	if errSpi != nil {
		return EPD0213LowLevel{}, fmt.Errorf("spi %v error %v", spiDeviceFileName, errSpi.Error())
	}
	c, errSpiConnect := p.Connect(physic.MegaHertz, spi.Mode0, 8)
	if errSpiConnect != nil {
		return EPD0213LowLevel{}, errSpiConnect

	}

	result := EPD0213LowLevel{
		spiConn:     c,
		readyPin:    gpioreg.ByName(readyPinName),
		resetPin:    gpioreg.ByName(resetPinName),
		dataModePin: gpioreg.ByName(dataModePinName),
	}

	if result.readyPin == nil {
		return result, fmt.Errorf("readyPin fail")
	}
	if result.resetPin == nil {
		return result, fmt.Errorf("resetPin fail")
	}
	if result.dataModePin == nil {
		return result, fmt.Errorf("dataPin fail")
	}

	return result, nil
}

func (p *EPD0213LowLevel) Send(command byte, pars ...byte) error { //Command and data
	onebyteresp := make([]byte, 1)
	dataModePinClearErr := p.dataModePin.Out(gpio.Low)
	if dataModePinClearErr != nil {
		return dataModePinClearErr
	}

	spiErr := p.spiConn.Tx([]byte{command}, onebyteresp)
	if spiErr != nil {
		return spiErr
	}

	if 0 < len(pars) {
		//DC=1
		dataModePinSetErr := p.dataModePin.Out(gpio.High)
		if dataModePinSetErr != nil {
			return dataModePinSetErr
		}
		//CS=0
		for _, data := range pars {
			spiErr := p.spiConn.Tx([]byte{data}, onebyteresp)
			if spiErr != nil {
				return spiErr
			}
		}
	}
	//CS=1  //SPI controls CS?
	return nil
}

func (p *EPD0213LowLevel) Reset() error {
	err := p.resetPin.Out(gpio.High)
	if err != nil {
		return err
	}

	time.Sleep(200 * time.Millisecond) //Needed if was low?
	err = p.resetPin.Out(gpio.Low)
	if err != nil {
		return err
	}

	time.Sleep(200 * time.Millisecond)
	err = p.resetPin.Out(gpio.High)
	if err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond) //Stupid? or time for recover
	return nil
}
func (p *EPD0213LowLevel) Idle() bool {
	return p.readyPin.Read() == gpio.Low
} //true if is idle
