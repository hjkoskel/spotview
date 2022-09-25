/*
Downloads pricing info from vattenfall api, if not available at cache
*/
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"
)

/**
https://www.vattenfall.fi/api/price/spot/2022-08-24/2022-08-24?lang=fi

{
    "timeStamp": "2022-08-24T08:00:00",
    "timeStampDay": "2022-08-24",
    "timeStampHour": "08:00",
    "value": 47.13,
    "priceArea": "",
    "unit": "snt/kWh"
  },
*/

const VATTENFALLEXPECTED_UNIT = "snt/kWh"

type VattenfallItem struct { //JSON struct. so much redudant data
	TimeStamp     string  `json:"timeStamp"`
	TimeStampDay  string  `json:"timeStampDay"`
	TimeStampHour string  `json:"timeStampHour"`
	Value         float64 `json:"value"`
	PriceArea     string  `json:"priceArea"`
	Unit          string  `json:"unit"` //VATTENFALLEXPECTED_UNIT
}

type VattenfallData []VattenfallItem

func GetVattenfallData(t time.Time, cachedir string) (VattenfallData, error) {
	result := VattenfallData{}

	cachefilenameonly, errName := vattenfallCacheFileName(t)
	if errName != nil {
		return VattenfallData{}, fmt.Errorf("timerr %v", errName.Error())
	}

	cachefilename := path.Join(cachedir, cachefilenameonly)

	fmt.Printf("Getting cached data from file %s\n", cachefilename)
	if fileExists(cachefilename) { //Good, get that
		content, readErr := ioutil.ReadFile(cachefilename)
		if readErr == nil {
			errUnmarshal := json.Unmarshal(content, &result)
			if errUnmarshal == nil {
				contentErr := result.CheckErr(t)
				if contentErr == nil {
					return result, nil //Got valid data from cache
				}
				fmt.Printf("Content error %v\n", contentErr.Error())
			} else {
				fmt.Printf("Unmarshall err from cache %v\n", errUnmarshal.Error())
			}
		} else {
			fmt.Printf("Read error %v\n", readErr.Error())
		}
	}
	fmt.Printf("Downloading fresh from vattenfall\n")
	content, dlErr := downloadVattenfall(t)
	if dlErr != nil {
		return result, dlErr
	}
	errUnmarshal := json.Unmarshal(content, &result)
	if errUnmarshal != nil {
		return result, errUnmarshal
	}

	err := result.CheckErr(t)
	if err != nil {
		return result, nil
	}
	//Ok, save to cache
	createErr := os.MkdirAll(cachedir, 0777)
	if createErr != nil {
		return result, fmt.Errorf("error creating cache %v fail %v", cachedir, createErr.Error())
	}
	errWriteCache := os.WriteFile(cachefilename, content, 0666)
	if errWriteCache != nil {
		return result, errWriteCache
	}

	return result, nil
}

func vattenfallCacheFileName(t time.Time) (string, error) {
	lt, ltErr := TimeInFinland(t)
	if ltErr != nil {
		return "", ltErr
	}
	return fmt.Sprintf("%v-%02d-%02d.json", lt.Year(), lt.Month(), lt.Day()), nil
}

func vattenfallUrl(t time.Time) (string, error) {
	lt, ltErr := TimeInFinland(t)
	if ltErr != nil {
		return "", ltErr
	}
	return fmt.Sprintf("http://www.vattenfall.fi/api/price/spot/%v-%02d-%02d/%v-%02d-%02d?lang=fi",
		lt.Year(), lt.Month(), lt.Day(),
		lt.Year(), lt.Month(), lt.Day(),
	), nil
}

func downloadVattenfall(t time.Time) ([]byte, error) {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	// Put content on file
	url, urlErr := vattenfallUrl(t)
	if urlErr != nil {
		return nil, urlErr
	}

	fmt.Printf("DL url is %s\n", url)
	resp, errGet := client.Get(url)
	if errGet != nil {
		return nil, errGet
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

//Check error, so it matches
func (p *VattenfallData) CheckErr(t time.Time) error {
	if len(*p) != 24 {
		return fmt.Errorf("not 24 items in hour data")
	}
	first := (*p)[0]

	lt, ltErr := TimeInFinland(t)
	if ltErr != nil {
		return ltErr
	}
	wantedDay := fmt.Sprintf("%v-%02d-%02d", lt.Year(), lt.Month(), lt.Day())
	if first.TimeStampDay != wantedDay {
		return fmt.Errorf("invalid day %s wanted %s\n", first.TimeStampDay, wantedDay)
	}

	for i, itm := range *p {
		if itm.Unit != VATTENFALLEXPECTED_UNIT {
			return fmt.Errorf("unexpected unit %s", itm.Unit)
		}
		if itm.PriceArea != "" {
			return fmt.Errorf("unexcepted price area %s", itm.PriceArea)
		}
		if itm.TimeStampHour != fmt.Sprintf("%02d:00", i) {
			return fmt.Errorf("invalid TimeStampHour %s at %02d:00", itm.TimeStamp, i)
		}
		if first.TimeStampDay != itm.TimeStampDay {
			return fmt.Errorf("timeStampDay changes from %s to %s", first.TimeStampDay, itm.TimeStampDay)
		}
		wantedTimestamp := fmt.Sprintf("%sT%s:00", itm.TimeStampDay, itm.TimeStampHour)
		if itm.TimeStamp != wantedTimestamp {
			return fmt.Errorf("invalid TimeStamp=%v wantedTimestamp %v", itm.TimeStamp, wantedTimestamp)
		}
	}
	return nil
}

func (p *VattenfallData) GetHourPrices(t time.Time) ([24]float64, error) {
	var result [24]float64
	if len(*p) != 24 {
		return result, fmt.Errorf("not 24 items")
	}
	for i, itm := range *p {
		result[i] = itm.Value
	}
	return result, nil
}

/*
Main routine for downloading from vattenfall net or cache
*/

func GetPriceViewVattenfall(tNow time.Time, cachedir string) (PriceView, error) {
	result := PriceView{}
	dataNow, errDataNow := GetVattenfallData(tNow, cachedir)
	if errDataNow != nil {
		return PriceView{}, fmt.Errorf("Todays data fail %v", errDataNow)
	}

	nowPrices, errNowPrices := dataNow.GetHourPrices(tNow)
	if errNowPrices != nil {
		return result, errNowPrices
	}

	tTomorrow := tNow.Add(time.Hour * 24)

	dataTomorrow, errDataTomorrow := GetVattenfallData(tTomorrow, cachedir)
	if errDataTomorrow == nil { //Good, today is first, then tomorrow
		tomorrowPrices, errTomorrowPrices := dataTomorrow.GetHourPrices(tTomorrow)
		if errTomorrowPrices == nil {
			return PriceView{
				FirstName: FinnishWeekDayName(tNow),
				FirstData: nowPrices,
				LastName:  FinnishWeekDayName(tTomorrow),
				LastData:  tomorrowPrices}, nil
		}
		fmt.Printf("tomorrow data err %v, get yesterday\n", errTomorrowPrices.Error())
	}
	//tomorrow prices are not available yet. use yesterday and today
	fmt.Printf("tomorrow prices not available yet\n")
	tYesteday := tNow.Add(-time.Hour * 24)
	dataYesterday, errDataYesterday := GetVattenfallData(tYesteday, cachedir)
	if errDataYesterday != nil {
		return result, errDataYesterday
	}

	yesterdayPrices, errYesterdayPrices := dataYesterday.GetHourPrices(tYesteday)
	if errYesterdayPrices != nil {
		return result, errYesterdayPrices
	}
	return PriceView{
		FirstName: FinnishWeekDayName(tYesteday),
		FirstData: yesterdayPrices,
		LastName:  FinnishWeekDayName(tNow),
		LastData:  nowPrices}, nil

}
