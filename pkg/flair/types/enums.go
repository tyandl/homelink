package types

// SystemMode controls how a Structure's HVAC and vents operate.
type SystemMode string

const (
	SystemModeAuto     SystemMode = "auto"
	SystemModeManual   SystemMode = "manual"
	SystemModeSchedule SystemMode = "schedule"
)

// StructureMode is the presence mode of a Structure.
type StructureMode string

const (
	StructureModeHome  StructureMode = "Home"
	StructureModeAway  StructureMode = "Away"
	StructureModeSleep StructureMode = "Sleep"
)

// TemperatureScale specifies Celsius or Fahrenheit display preference.
type TemperatureScale string

const (
	TemperatureScaleCelsius    TemperatureScale = "C"
	TemperatureScaleFahrenheit TemperatureScale = "F"
)

// SetPointMode controls which temperature set point the HVAC system targets.
type SetPointMode string

const (
	SetPointModeFloat SetPointMode = "Float" // system chooses based on HVAC mode
	SetPointModeCool  SetPointMode = "Cool"
	SetPointModeHeat  SetPointMode = "Heat"
	SetPointModeAuto  SetPointMode = "Auto"
)

// HvacMode is the operating mode of an HVAC unit.
type HvacMode string

const (
	HvacModeCool HvacMode = "cool"
	HvacModeHeat HvacMode = "heat"
	HvacModeAuto HvacMode = "auto"
	HvacModeFan  HvacMode = "fan"
	HvacModeDry  HvacMode = "dry"
	HvacModeOff  HvacMode = "off"
)

// PuckColorSelection is the LED color displayed on a Puck.
type PuckColorSelection string

const (
	PuckColorRed    PuckColorSelection = "red"
	PuckColorGreen  PuckColorSelection = "green"
	PuckColorBlue   PuckColorSelection = "blue"
	PuckColorYellow PuckColorSelection = "yellow"
	PuckColorOff    PuckColorSelection = "off"
)
