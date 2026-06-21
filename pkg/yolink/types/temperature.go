package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TemperatureUnit string

const (
	Celsius    TemperatureUnit = "C"
	Fahrenheit TemperatureUnit = "F"
)

func (u *TemperatureUnit) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = TemperatureUnit(strings.ToUpper(s))
	return nil
}

type Temperature struct {
	value float64
	unit  TemperatureUnit
}

func NewTemperature(value float64, unit TemperatureUnit) Temperature {
	return Temperature{value: value, unit: unit}
}

func (t *Temperature) UnmarshalJSON(data []byte) error {
	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	t.value = value
	t.unit = Celsius
	return nil
}

func (t Temperature) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value)
}

func (t Temperature) ToC() Temperature {
	if t.unit == Celsius {
		return t
	}
	return Temperature{value: (t.value - 32) * 5 / 9, unit: Celsius}
}

func (t Temperature) ToF() Temperature {
	if t.unit == Fahrenheit {
		return t
	}
	return Temperature{value: t.value*9/5 + 32, unit: Fahrenheit}
}

func (t Temperature) Value() float64 {
	return t.value
}

func (t Temperature) Unit() TemperatureUnit {
	return t.unit
}

func (t Temperature) String() string {
	return fmt.Sprintf("%.1f°%s", t.value, t.unit)
}
