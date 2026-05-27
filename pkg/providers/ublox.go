package providers

import (
	"fmt"
	"golang.org/x/sys/execabs"
	"golang.org/x/xerrors"
	"io"
)

type UbloxProvider struct {
}

func NewUbloxProvider() *UbloxProvider {
	u := &UbloxProvider{}
	return u
}

// FlashGPS uploads the full GPS configuration file (Robot.txt.set.ubx) to the
// u-blox module via ubxload.  The config file hard-codes UBX binary output on
// USB; call ConfigureGPSProtocol afterwards if the user has selected NMEA.
func (fp *UbloxProvider) FlashGPS(writer io.Writer) error {
	_, _ = writer.Write([]byte("------> Uploading GPS configuration...\n"))
	cmd := execabs.Command("/bin/bash", "-c", "ubxload --port /dev/gps --baudrate 115200 --timeout 0.05 --infile Robot.txt.set.ubx --verbosity 3")
	cmd.Dir = "/app/setup"
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	if err != nil {
		_, _ = writer.Write([]byte("------> Error while uploading GPS configuration: " + err.Error() + "\n"))
		return xerrors.Errorf("error while flashing gps: %w", err)
	}
	_, _ = writer.Write([]byte("------> GPS configuration uploaded\n"))
	return nil
}

// ConfigureGPSProtocol switches the GPS hardware output between UBX binary and
// NMEA ASCII on the given USB/serial device.  It calls configure_gps_protocol.py
// which sends UBX-CFG-VALSET commands saved to Flash so the setting survives
// power cycles.
func (fp *UbloxProvider) ConfigureGPSProtocol(writer io.Writer, protocol, device string) error {
	_, _ = writer.Write([]byte(fmt.Sprintf("------> Configuring GPS for %s protocol on %s...\n", protocol, device)))
	cmd := execabs.Command("python3", "/app/setup/configure_gps_protocol.py",
		"--protocol", protocol,
		"--port", device,
	)
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	if err != nil {
		_, _ = writer.Write([]byte("------> Error configuring GPS protocol: " + err.Error() + "\n"))
		return xerrors.Errorf("error configuring GPS protocol: %w", err)
	}
	_, _ = writer.Write([]byte("------> GPS protocol configured\n"))
	return nil
}
