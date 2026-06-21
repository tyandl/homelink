package types

import "fmt"

type StatusCode string

const (
	StatusSuccess StatusCode = "000000"

	// Hub errors
	StatusHubConnectionFailed StatusCode = "000101"
	StatusHubNoResponse       StatusCode = "000102"
	StatusTokenInvalid        StatusCode = "000103"
	StatusHubTokenInvalid     StatusCode = "000104"
	StatusRedirectURINull     StatusCode = "000105"
	StatusClientIDInvalid     StatusCode = "000106"

	// Device errors
	StatusDeviceConnectionFailed StatusCode = "000201"
	StatusDeviceNoResponse       StatusCode = "000202"
	StatusDeviceConnectionLost   StatusCode = "000203"

	// Service errors
	StatusServiceUnavailable            StatusCode = "010000"
	StatusInternalConnectionUnavailable StatusCode = "010001"

	// Request validation errors
	StatusCSIDInvalid          StatusCode = "010101"
	StatusSecKeyInvalid        StatusCode = "010102"
	StatusAuthorizationInvalid StatusCode = "010103"
	StatusTokenExpired         StatusCode = "010104"

	// Data packet errors
	StatusParamsInvalid      StatusCode = "010200"
	StatusTimeMissing        StatusCode = "010201"
	StatusMethodMissing      StatusCode = "010202"
	StatusMethodNotSupported StatusCode = "010203"
	StatusInvalidDataPacket  StatusCode = "010204"

	// Access errors
	StatusInterfaceRestricted StatusCode = "010300"
	StatusRateLimitExceeded   StatusCode = "010301"

	// Device management errors
	StatusDeviceAlreadyBound    StatusCode = "020100"
	StatusDeviceNotFound        StatusCode = "020101"
	StatusDeviceMaskError       StatusCode = "020102"
	StatusDeviceNotSupported    StatusCode = "020103"
	StatusDeviceBusy            StatusCode = "020104"
	StatusDeviceRetrievalFailed StatusCode = "020105"
	StatusNoDevicesFound        StatusCode = "020201"

	// Data errors
	StatusNoDataFound StatusCode = "030101"

	StatusUnknown StatusCode = "999999"
)

var statusDescriptions = map[StatusCode]string{
	StatusSuccess:                       "Success",
	StatusHubConnectionFailed:           "Can't connect to Hub",
	StatusHubNoResponse:                 "The hub cannot respond to this command",
	StatusTokenInvalid:                  "Token is invalid",
	StatusHubTokenInvalid:               "Hub token is invalid",
	StatusRedirectURINull:               "redirect_uri can't be null",
	StatusClientIDInvalid:               "client_id is invalid",
	StatusDeviceConnectionFailed:        "Cannot connect to the device",
	StatusDeviceNoResponse:              "The device cannot respond to this command",
	StatusDeviceConnectionLost:          "Cannot connect to the device",
	StatusServiceUnavailable:            "Service is not available, try again later",
	StatusInternalConnectionUnavailable: "Internal connection is not available, try again later",
	StatusCSIDInvalid:                   "Invalid request: CSID is invalid!",
	StatusSecKeyInvalid:                 "Invalid request: SecKey is invalid!",
	StatusAuthorizationInvalid:          "Invalid request: Authorization is invalid!",
	StatusTokenExpired:                  "Invalid request: The token is expired",
	StatusParamsInvalid:                 "Invalid data packet: params is not valid",
	StatusTimeMissing:                   "Invalid data packet: time can not be null",
	StatusMethodMissing:                 "Invalid data packet: method can not be null",
	StatusMethodNotSupported:            "Invalid data packet: method is not supported",
	StatusInvalidDataPacket:             "Invalid data packet",
	StatusInterfaceRestricted:           "This interface is restricted to access",
	StatusRateLimitExceeded:             "Access denied due to limits reached, Please retry later",
	StatusDeviceAlreadyBound:            "The device is already bound by another user",
	StatusDeviceNotFound:                "The device does not exist",
	StatusDeviceMaskError:               "Device mask error",
	StatusDeviceNotSupported:            "The device is not supported",
	StatusDeviceBusy:                    "Device is busy, try again later.",
	StatusDeviceRetrievalFailed:         "Unable to retrieve device",
	StatusNoDevicesFound:                "No devices were searched",
	StatusNoDataFound:                   "No Data found",
	StatusUnknown:                       "UnKnown error, please report it to yaochi@yosmart.com",
}

// String returns the human-readable description for the status code.
func (code StatusCode) String() string {
	if description, ok := statusDescriptions[code]; ok {
		return description
	}
	return fmt.Sprintf("unknown status code: %s", string(code))
}

// IsSuccess reports whether the code represents a successful response.
func (code StatusCode) IsSuccess() bool {
	return code == StatusSuccess
}
