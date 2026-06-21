package types

import (
	"encoding/json"
	"fmt"
)

type SwitchState string

const (
	SwitchStateClosed SwitchState = "closed"
	SwitchStateOpen   SwitchState = "open"
)

type SwitchCommand string

const (
	SwitchCommandClose SwitchCommand = "close"
	SwitchCommandOpen  SwitchCommand = "open"
)

type AlarmState string

const (
	AlarmStateNormal AlarmState = "normal"
	AlarmStateAlert  AlarmState = "alert"
	AlarmStateOff    AlarmState = "off"
)

type LockState string

const (
	LockStateLocked   LockState = "locked"
	LockStateUnlocked LockState = "unlocked"
)

type PowerSupply string

const (
	PowerSupplyBattery   PowerSupply = "battery"
	PowerSupplyUsb       PowerSupply = "usb"
	PowerSupplyPowerLine PowerSupply = "PowerLine"
)

type ThermostatMode string

const (
	ThermostatModeCool ThermostatMode = "cool"
	ThermostatModeHeat ThermostatMode = "heat"
	ThermostatModeAuto ThermostatMode = "auto"
	ThermostatModeOff  ThermostatMode = "off"
)

type FanMode string

const (
	FanModeOn   FanMode = "on"
	FanModeAuto FanMode = "auto"
)

type LatchSide string

const (
	LatchSideLeft  LatchSide = "left"
	LatchSideRight LatchSide = "right"
)

type DoorState string

const (
	DoorStateClosed DoorState = "closed"
	DoorStateOpen   DoorState = "open"
	DoorStateError  DoorState = "error"
)

type ScheduleType string

const (
	ScheduleTypeDisable ScheduleType = "disable"
	ScheduleTypeWeekly  ScheduleType = "weekly"
	ScheduleTypeMonthly ScheduleType = "monthly"
)

type SprinklerMode string

const (
	SprinklerModeAuto   SprinklerMode = "auto"
	SprinklerModeManual SprinklerMode = "manual"
	SprinklerModeOff    SprinklerMode = "off"
)

type SprinklerCommand string

const (
	SprinklerCommandStart SprinklerCommand = "start"
	SprinklerCommandStop  SprinklerCommand = "stop"
)

type WaterMode string

const (
	WaterModeManual   WaterMode = "manual"
	WaterModeSchedule WaterMode = "schedule"
)

type SprinklerWaterType string

const (
	SprinklerWaterTypeDuration SprinklerWaterType = "duration"
	SprinklerWaterTypeAmount   SprinklerWaterType = "amount"
)

type ThermostatRunning string

const (
	ThermostatRunningCool ThermostatRunning = "cool"
	ThermostatRunningHeat ThermostatRunning = "heat"
	ThermostatRunningIdle ThermostatRunning = "idle"
)

type ThermostatSchedule string

const (
	ThermostatScheduleRun  ThermostatSchedule = "run"
	ThermostatScheduleHold ThermostatSchedule = "hold"
)

type ThermostatMaster string

const (
	ThermostatMasterLocal   ThermostatMaster = "local"
	ThermostatMasterSensor1 ThermostatMaster = "sensor1"
	ThermostatMasterSensor2 ThermostatMaster = "sensor2"
)

type LeakSensorMode string

const (
	LeakSensorModeWaterPeak LeakSensorMode = "WaterPeak"
	LeakSensorModeWaterLeak LeakSensorMode = "WaterLeak"
)

type LeakSensorSensitivity string

const (
	LeakSensorSensitivityLow  LeakSensorSensitivity = "low"
	LeakSensorSensitivityHigh LeakSensorSensitivity = "high"
)

type WaterLeakPlan string

const (
	WaterLeakPlanManual   WaterLeakPlan = "Manual"
	WaterLeakPlanAuto     WaterLeakPlan = "Auto"
	WaterLeakPlanAwayAuto WaterLeakPlan = "AwayAuto"
)

type WaterMeterLeakPlan string

const (
	WaterMeterLeakPlanOn       WaterMeterLeakPlan = "on"
	WaterMeterLeakPlanOff      WaterMeterLeakPlan = "off"
	WaterMeterLeakPlanSchedule WaterMeterLeakPlan = "schedule"
)

type RecordMode string

const (
	RecordModeFullTime RecordMode = "full-time"
	RecordModeAlarm    RecordMode = "alarm"
	RecordModeOff      RecordMode = "off"
)

type NightVision string

const (
	NightVisionOn   NightVision = "on"
	NightVisionOff  NightVision = "off"
	NightVisionAuto NightVision = "auto"
)

type SpeakerTone string

const (
	SpeakerToneEmergency SpeakerTone = "Emergency"
	SpeakerToneAlert     SpeakerTone = "Alert"
	SpeakerToneWarn      SpeakerTone = "Warn"
	SpeakerToneTip       SpeakerTone = "Tip"
)

type LockCredentialType string

const (
	LockCredentialTypeOneTime    LockCredentialType = "OneTime"
	LockCredentialTypeRangeTime  LockCredentialType = "RangeTime"
	LockCredentialTypePeriodTime LockCredentialType = "PeriodTime"
)

type LockCipherType string

const (
	LockCipherTypeFingerprint LockCipherType = "Fingerprint"
	LockCipherTypeCard        LockCipherType = "Card"
	LockCipherTypeFob         LockCipherType = "Fob"
	LockCipherTypeCode        LockCipherType = "Code"
)

type CellularRegState string

const (
	CellularRegStateNone        CellularRegState = "none"
	CellularRegStateRegistered  CellularRegState = "registered"
	CellularRegStateRegistering CellularRegState = "registering"
	CellularRegStateDenied      CellularRegState = "denied"
	CellularRegStateRoaming     CellularRegState = "roaming"
	CellularRegStateError       CellularRegState = "error"
)

type SmartRemoterEventType string

const (
	SmartRemoterEventTypePress     SmartRemoterEventType = "Press"
	SmartRemoterEventTypeLongPress SmartRemoterEventType = "LongPress"
)

type BatteryLevel string

const (
	BatteryLevelCritical BatteryLevel = "critical"
	BatteryLevelLow      BatteryLevel = "low"
	BatteryLevelMedium   BatteryLevel = "medium"
	BatteryLevelHigh     BatteryLevel = "high"
	BatteryLevelFull     BatteryLevel = "full"
)

var batteryLevelValues = []BatteryLevel{
	BatteryLevelCritical,
	BatteryLevelLow,
	BatteryLevelMedium,
	BatteryLevelHigh,
	BatteryLevelFull,
}

func (v *BatteryLevel) UnmarshalJSON(data []byte) error {
	var i int
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	if i < 0 || i >= len(batteryLevelValues) {
		return fmt.Errorf("BatteryLevel ordinal %d out of range", i)
	}
	*v = batteryLevelValues[i]
	return nil
}

func (v BatteryLevel) MarshalJSON() ([]byte, error) {
	for i, val := range batteryLevelValues {
		if val == v {
			return json.Marshal(i)
		}
	}
	return nil, fmt.Errorf("unknown BatteryLevel: %q", string(v))
}

type BatteryState string

const (
	BatteryStateNone        BatteryState = "none"
	BatteryStatePowering    BatteryState = "powering"
	BatteryStateCharging    BatteryState = "charging"
	BatteryStateStandby     BatteryState = "standby"
	BatteryStateMaintenance BatteryState = "maintenance"
)

var batteryStateValues = []BatteryState{
	BatteryStateNone,
	BatteryStatePowering,
	BatteryStateCharging,
	BatteryStateStandby,
	BatteryStateMaintenance,
}

func (v *BatteryState) UnmarshalJSON(data []byte) error {
	var i int
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	if i < 0 || i >= len(batteryStateValues) {
		return fmt.Errorf("BatteryState ordinal %d out of range", i)
	}
	*v = batteryStateValues[i]
	return nil
}

func (v BatteryState) MarshalJSON() ([]byte, error) {
	for i, val := range batteryStateValues {
		if val == v {
			return json.Marshal(i)
		}
	}
	return nil, fmt.Errorf("unknown BatteryState: %q", string(v))
}

type LockSoundLevel string

const (
	LockSoundLevelMute   LockSoundLevel = "mute"
	LockSoundLevelLow    LockSoundLevel = "low"
	LockSoundLevelMedium LockSoundLevel = "medium"
	LockSoundLevelHigh   LockSoundLevel = "high"
)

var lockSoundLevelValues = []LockSoundLevel{
	LockSoundLevelMute,
	LockSoundLevelLow,
	LockSoundLevelMedium,
	LockSoundLevelHigh,
}

func (v *LockSoundLevel) UnmarshalJSON(data []byte) error {
	var i int
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	if i < 0 || i >= len(lockSoundLevelValues) {
		return fmt.Errorf("LockSoundLevel ordinal %d out of range", i)
	}
	*v = lockSoundLevelValues[i]
	return nil
}

func (v LockSoundLevel) MarshalJSON() ([]byte, error) {
	for i, val := range lockSoundLevelValues {
		if val == v {
			return json.Marshal(i)
		}
	}
	return nil, fmt.Errorf("unknown LockSoundLevel: %q", string(v))
}

type MeterUnit string

const (
	MeterUnitGAL MeterUnit = "GAL"
	MeterUnitCCF MeterUnit = "CCF"
	MeterUnitM3  MeterUnit = "M3"
	MeterUnitL   MeterUnit = "L"
)

var meterUnitValues = []MeterUnit{
	MeterUnitGAL,
	MeterUnitCCF,
	MeterUnitM3,
	MeterUnitL,
}

func (v *MeterUnit) UnmarshalJSON(data []byte) error {
	var i int
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	if i < 0 || i >= len(meterUnitValues) {
		return fmt.Errorf("MeterUnit ordinal %d out of range", i)
	}
	*v = meterUnitValues[i]
	return nil
}

func (v MeterUnit) MarshalJSON() ([]byte, error) {
	for i, val := range meterUnitValues {
		if val == v {
			return json.Marshal(i)
		}
	}
	return nil, fmt.Errorf("unknown MeterUnit: %q", string(v))
}

type WaterTemperatureLevel string

const (
	WaterTemperatureLevelCold WaterTemperatureLevel = "cold"
	WaterTemperatureLevelHot  WaterTemperatureLevel = "hot"
)

var waterTemperatureLevelValues = []WaterTemperatureLevel{
	WaterTemperatureLevelCold,
	WaterTemperatureLevelHot,
}

func (v *WaterTemperatureLevel) UnmarshalJSON(data []byte) error {
	var i int
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	if i < 0 || i >= len(waterTemperatureLevelValues) {
		return fmt.Errorf("WaterTemperatureLevel ordinal %d out of range", i)
	}
	*v = waterTemperatureLevelValues[i]
	return nil
}

func (v WaterTemperatureLevel) MarshalJSON() ([]byte, error) {
	for i, val := range waterTemperatureLevelValues {
		if val == v {
			return json.Marshal(i)
		}
	}
	return nil, fmt.Errorf("unknown WaterTemperatureLevel: %q", string(v))
}

type ThermostatSwitch string

const (
	ThermostatSwitchOn  ThermostatSwitch = "on"
	ThermostatSwitchOff ThermostatSwitch = "off"
)

type SprinklerDaysType string

const (
	SprinklerDaysTypeWeekly       SprinklerDaysType = "weekly"
	SprinklerDaysTypeEvenDays     SprinklerDaysType = "even_days"
	SprinklerDaysTypeOddDays      SprinklerDaysType = "odd_days"
	SprinklerDaysTypeEveryFewDays SprinklerDaysType = "every_few_days"
)

type WiFiEncryption string

const (
	WiFiEncryptionNone     WiFiEncryption = "none"
	WiFiEncryptionWep      WiFiEncryption = "wep"
	WiFiEncryptionPsk      WiFiEncryption = "psk"
	WiFiEncryptionPsk2     WiFiEncryption = "psk2"
	WiFiEncryptionPskMixed WiFiEncryption = "psk-mixed"
	WiFiEncryptionUnknown  WiFiEncryption = "unknown"
)

type AlertState string

const (
	AlertStateAlert  AlertState = "alert"
	AlertStateNormal AlertState = "normal"
)
