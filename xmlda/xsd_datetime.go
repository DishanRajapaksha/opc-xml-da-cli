package xmlda

import (
	"encoding/xml"
	"strings"
	"time"

	"github.com/hooklift/gowsdl/soap"
)

type XSDDateTime struct {
	soap.XSDDateTime
}

func (xdt *XSDDateTime) UnmarshalXMLAttr(attr xml.Attr) error {
	parsed, hasTz, err := parseXsdDateTime(attr.Value)
	if err != nil {
		return err
	}
	xdt.XSDDateTime = soap.CreateXsdDateTime(parsed, hasTz)
	return nil
}

func (xdt XSDDateTime) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	t := xdt.ToGoTime()
	if t.IsZero() {
		return xml.Attr{}, nil
	}
	return xml.Attr{Name: name, Value: t.Format(time.RFC3339Nano)}, nil
}

func parseXsdDateTime(value string) (time.Time, bool, error) {
	if value == "" {
		return time.Time{}, true, nil
	}

	hasTz := false
	if strings.Contains(value, "T") {
		dateAndTime := strings.SplitN(value, "T", 2)
		if len(dateAndTime) > 1 {
			if strings.Contains(dateAndTime[1], "Z") ||
				strings.Contains(dateAndTime[1], "+") ||
				strings.Contains(dateAndTime[1], "-") {
				hasTz = true
			}
		}
		if !hasTz {
			value += "Z"
		}
		if value == "0001-01-01T00:00:00Z" {
			return time.Time{}, true, nil
		}
	} else {
		if strings.Contains(value, "Z") || strings.Contains(value, ":") {
			hasTz = true
		}
		if !hasTz {
			value += "Z"
		}
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	return parsed, hasTz, err
}
