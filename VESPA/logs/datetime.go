/*
	Copyright 2019 Nokia
	Copyright (c) 2020 Mavenir

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/
package logs

import (
	"fmt"
	"time"
)

// Get function is responsible for taking date format, time format and location
// as args for time and return time in string format
// and return error if any else nil.
func Get(df, tf, location string) (string, error) {
	// Get current time
	cd := time.Now()
	// Loading location
	// If no location specified, UTC will be returned
	tz, err := time.LoadLocation(location)
	if err != nil {
		fmt.Println("Error in load location", err.Error())
		return "", err
	}
	switch df + tf {
	case "YY-MM-DD" + "SS:MM:HH":
		return cd.In(tz).Format("2006-01-02 03:04:05PM"), nil
	case "DD-MM-YY" + "HH:MM:SS":
		return cd.In(tz).Format("02-01-2006 03:04:05PM"), nil
	case "DD/MM/YY" + "MM:HH:SS":
		return cd.In(tz).Format("02/01/2006 04:03:05PM"), nil
	case "YY/MM/DD" + "HH:MM:SS":
		return cd.In(tz).Format("2006/01/02 15:04:05"), nil
	case "YY/DD/MM" + "HH:SS:MM":
		return cd.In(tz).Format("2006/02/01 15:04:05"), nil
	case "DD/MM/YYYY" + "":
		return cd.In(tz).Format("02/01/2006"), nil
	case "MM/DD/YYYY" + "":
		return cd.In(tz).Format("01/02/2006"), nil
	case "YY/MM/DD" + "":
		return cd.In(tz).Format("2006/01/02"), nil
	case "MM/DD/YY" + "":
		return cd.In(tz).Format("01/02/2006"), nil
	case "YYYY-MM-DD" + "":
		return cd.In(tz).Format("2006-01-02"), nil
	case "YYYYMMDD" + "HHMMSS":
		return cd.In(tz).Format("20060102150405"), nil
	case "YYYYMMDD" + "":
		return cd.In(tz).Format("20060102"), nil
	case "CYYMMDD" + "":
		return "1" + cd.In(tz).Format("060102"), nil
	case "DDMMYYYY" + "":
		return cd.In(tz).Format("02012006"), nil
	case "MMDDYY" + "HHMMSS":
		return cd.In(tz).Format("010206150405"), nil
	case "YY-MM-DD" + "SS-MM-HH":
		return cd.In(tz).Format("2006-01-02T03-04-05"), nil
	case "YY-MM-DD" + "SS-MM-HH-MIC":
		return cd.In(tz).Format("2006-01-02T15:04:05.000000000"), nil
	default:
		return cd.In(tz).Format("2006-01-02T15:04:05"), nil
	}
	// More switch cases can be added as per requirements
	// Maximum formats are covered in this only
}
