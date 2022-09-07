package main

import (
	"os"
	"time"
)

//fileExists tells is file existing
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func TimeInFinland(t time.Time) (time.Time, error) {
	loc, errloc := time.LoadLocation("Europe/Helsinki") //Hard coded. Vattenfall runs in finnish time
	if errloc != nil {
		return t, errloc
	}
	return t.In(loc), nil
}

func FinnishWeekDayName(t time.Time) string {
	return map[int]string{1: "Ma", 2: "Ti", 3: "Ke", 4: "To", 5: "Pe", 6: "La", 7: "Su"}[int(t.Weekday())]
}
