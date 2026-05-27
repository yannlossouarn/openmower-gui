package api

import (
	"bufio"
	"fmt"
	"github.com/cedbossneo/openmower-gui/pkg/types"
	"github.com/gin-gonic/gin"
	"io"
)

// defaultGPSProtocol and defaultGPSPort are the fallback values used when the
// mower_config.sh file does not (yet) contain OM_GPS_PROTOCOL / OM_GPS_PORT.
const defaultGPSProtocol = "UBX"
const defaultGPSPort = "/dev/gps"

func SetupRoutes(r *gin.RouterGroup, provider types.IFirmwareProvider, ubloxProvider types.IGpsProvider, dbProvider types.IDBProvider) {
	group := r.Group("/setup")
	FlashBoard(group, provider)
	FlashGPS(group, ubloxProvider, dbProvider)
	ConfigureGPS(group, ubloxProvider, dbProvider)
}

// FlashGPS uploads the full GPS configuration file and then switches the GPS
// output protocol to match OM_GPS_PROTOCOL from mower_config.sh.
//
// @Summary flash the GPS configuration and set the output protocol
// @Description flash the GPS configuration file then configure UBX or NMEA output to match OM_GPS_PROTOCOL
// @Tags setup
// @Accept  json
// @Produce  text/event-stream
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /setup/flashGPS [post]
func FlashGPS(group *gin.RouterGroup, provider types.IGpsProvider, dbProvider types.IDBProvider) gin.IRoutes {
	return group.POST("/flashGPS", func(context *gin.Context) {
		// Read the current GPS protocol and port from mower_config.sh.
		// Fall back to sensible defaults if the file doesn't exist yet.
		configValues, err := readMowerConfigValues(dbProvider)
		protocol := defaultGPSProtocol
		device := defaultGPSPort
		if err == nil {
			if p, ok := configValues["OM_GPS_PROTOCOL"]; ok && p != "" {
				protocol = p
			}
			if d, ok := configValues["OM_GPS_PORT"]; ok && d != "" {
				device = d
			}
		}

		reader, writer := io.Pipe()
		rd := bufio.NewReader(reader)
		go func() {
			// Step 1: upload the full GPS config file (ubxload always sets UBX mode).
			if flashErr := provider.FlashGPS(writer); flashErr != nil {
				writer.CloseWithError(flashErr)
				return
			}

			// Step 2: set the protocol the user has chosen in Settings.
			// Even for UBX we run this to ensure all protocol-related keys are
			// consistent (ubxload may leave some keys at firmware defaults).
			_, _ = writer.Write([]byte(fmt.Sprintf("------> Applying protocol from settings: %s\n", protocol)))
			if cfgErr := provider.ConfigureGPSProtocol(writer, protocol, device); cfgErr != nil {
				writer.CloseWithError(cfgErr)
				return
			}

			writer.Close()
		}()
		context.Stream(func(w io.Writer) bool {
			line, _, err2 := rd.ReadLine()
			if err2 != nil {
				if err2 == io.EOF {
					context.SSEvent("end", "end")
					return false
				}
				context.SSEvent("error", err2.Error())
				return false
			}
			context.SSEvent("message", string(line))
			return true
		})
	})
}

// ConfigureGPS reads OM_GPS_PROTOCOL and OM_GPS_PORT from mower_config.sh and
// pushes the chosen protocol to the GPS hardware.  This is used by the Settings
// page to apply a protocol change without re-flashing the full config file.
//
// @Summary configure GPS hardware output protocol
// @Description switch the GPS output between UBX binary and NMEA ASCII to match OM_GPS_PROTOCOL
// @Tags setup
// @Accept  json
// @Produce  text/event-stream
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /setup/configureGPS [post]
func ConfigureGPS(group *gin.RouterGroup, provider types.IGpsProvider, dbProvider types.IDBProvider) gin.IRoutes {
	return group.POST("/configureGPS", func(context *gin.Context) {
		configValues, err := readMowerConfigValues(dbProvider)
		if err != nil {
			context.JSON(500, ErrorResponse{Error: "cannot read mower config: " + err.Error()})
			return
		}
		protocol := defaultGPSProtocol
		device := defaultGPSPort
		if p, ok := configValues["OM_GPS_PROTOCOL"]; ok && p != "" {
			protocol = p
		}
		if d, ok := configValues["OM_GPS_PORT"]; ok && d != "" {
			device = d
		}

		reader, writer := io.Pipe()
		rd := bufio.NewReader(reader)
		go func() {
			err := provider.ConfigureGPSProtocol(writer, protocol, device)
			if err != nil {
				writer.CloseWithError(err)
			} else {
				writer.Close()
			}
		}()
		context.Stream(func(w io.Writer) bool {
			line, _, err2 := rd.ReadLine()
			if err2 != nil {
				if err2 == io.EOF {
					context.SSEvent("end", "end")
					return false
				}
				context.SSEvent("error", err2.Error())
				return false
			}
			context.SSEvent("message", string(line))
			return true
		})
	})
}

// FlashBoard flash the mower board with the given config
//
// @Summary flash the mower board with the given config
// @Description flash the mower board with the given config
// @Tags setup
// @Accept  json
// @Produce  text/event-stream
// @Param settings body types.FirmwareConfig true "config"
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /setup/flashBoard [post]
func FlashBoard(r *gin.RouterGroup, provider types.IFirmwareProvider) gin.IRoutes {
	return r.POST("/flashBoard", func(c *gin.Context) {
		var config types.FirmwareConfig
		var err error
		err = c.BindJSON(&config)
		if err != nil {
			c.JSON(500, ErrorResponse{
				Error: err.Error(),
			})
			return
		}
		reader, writer := io.Pipe()
		rd := bufio.NewReader(reader)
		go func() {
			err = provider.FlashFirmware(writer, config)
			if err != nil {
				writer.CloseWithError(err)
			} else {
				writer.Close()
			}
		}()
		c.Stream(func(w io.Writer) bool {
			line, _, err2 := rd.ReadLine()
			if err2 != nil {
				if err2 == io.EOF {
					c.SSEvent("end", "end")
					return false
				}
				c.SSEvent("error", err2.Error())
				return false
			}
			c.SSEvent("message", string(line))
			return true
		})
	})
}
