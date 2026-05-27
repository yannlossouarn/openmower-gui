package types

import "io"

type IGpsProvider interface {
	FlashGPS(writer io.Writer) error
	// ConfigureGPSProtocol switches the GPS hardware output to the given protocol
	// (either "UBX" or "NMEA") on the specified serial device.
	// Progress and errors are written to writer so callers can stream them to the UI.
	ConfigureGPSProtocol(writer io.Writer, protocol, device string) error
}
