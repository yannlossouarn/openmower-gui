package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/bluenviron/goroslib/v2/pkg/msgs/geometry_msgs"
	"github.com/cedbossneo/openmower-gui/pkg/msgs/dynamic_reconfigure"
	"github.com/cedbossneo/openmower-gui/pkg/msgs/mower_map"
	"github.com/cedbossneo/openmower-gui/pkg/msgs/mower_msgs"
	"github.com/cedbossneo/openmower-gui/pkg/types"
	"github.com/docker/distribution/uuid"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// rawAreaOverride is used to detect which per-area override fields the frontend
// explicitly sent (pointer = nil when the JSON key is absent or null).
type rawAreaOverride struct {
	Angle               *float64 `json:"angle"`
	OutlineCount        *int32   `json:"outline_count"`
	OutlineOverlapCount *int32   `json:"outline_overlap_count"`
	OutlineOffset       *float64 `json:"outline_offset"`
}
type rawReplaceMapBody struct {
	Areas []struct {
		Area rawAreaOverride `json:"area"`
	} `json:"areas"`
}

// MowingAreaDetails is the response shape for GET /openmower/map/areas.
// Pointer fields are nil when no per-area override is set (global param applies).
type MowingAreaDetails struct {
	Index               int      `json:"index"`
	Name                string   `json:"name"`
	Active              bool     `json:"active"`
	Angle               *float64 `json:"angle"`
	OutlineCount        *int32   `json:"outline_count"`
	OutlineOverlapCount *int32   `json:"outline_overlap_count"`
	OutlineOffset       *float64 `json:"outline_offset"`
}

func OpenMowerRoutes(r *gin.RouterGroup, provider types.IRosProvider) {
	group := r.Group("/openmower")
	ServiceRoute(group, provider)
	AddMapAreaRoute(group, provider)
	SetDockingPointRoute(group, provider)
	ClearMapRoute(group, provider)
	ReplaceMapRoute(group, provider)
	AreaDetailsRoute(group, provider)
	SubscriberRoute(group, provider)
	PublisherRoute(group, provider)
}

// AddMapAreaRoute add a map area
//
// @Summary add a map area
// @Description add a map area
// @Tags openmower
// @Accept  json
// @Produce  json
// @Param CallReq body mower_map.AddMowingAreaSrvReq true "request body"
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /openmower/map/area/add [post]
func AddMapAreaRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.POST("/map/area/add", func(c *gin.Context) {
		var CallReq mower_map.AddMowingAreaSrvReq
		err := unmarshalROSMessage[*mower_map.AddMowingAreaSrvReq](c.Request.Body, &CallReq)
		if err != nil {
			return
		}
		err = provider.CallService(c.Request.Context(), "/mower_map_service/add_mowing_area", &mower_map.AddMowingAreaSrv{}, &CallReq, &mower_map.AddMowingAreaSrvRes{})
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(200, OkResponse{})
		}
	})
}

// ClearMapRoute delete a map area
//
// @Summary clear the map
// @Description clear the map
// @Tags openmower
// @Accept  json
// @Produce  json
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /openmower/map [delete]
func ClearMapRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.DELETE("/map", func(c *gin.Context) {
		err := provider.CallService(c.Request.Context(), "/mower_map_service/clear_map", &mower_map.ClearMapSrv{}, &mower_map.ClearMapSrvReq{}, &mower_map.ClearMapSrvRes{})
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(200, OkResponse{})
		}
	})
}

// ReplaceMapRoute delete a map area
//
// @Summary clear the map and insert areas
// @Description clear the map and insert areas
// @Tags openmower
// @Accept  json
// @Produce  json
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /openmower/map [put]
func ReplaceMapRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.PUT("/map", func(c *gin.Context) {
		// Read the body once so we can parse it twice
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
			return
		}

		// Parse per-area override fields using pointer types so we can detect
		// absent/null JSON keys (nil pointer = not set = use global parameter).
		var rawBody rawReplaceMapBody
		_ = json.Unmarshal(bodyBytes, &rawBody) // best-effort; errors mean no overrides

		// Clear the existing map
		err = provider.CallService(c.Request.Context(), "/mower_map_service/clear_map", &mower_map.ClearMapSrv{}, &mower_map.ClearMapSrvReq{}, &mower_map.ClearMapSrvRes{})
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
			return
		}

		// Unmarshal geometry via mapstructure (existing path)
		var CallReq mower_map.ReplaceMowingAreaSrvReq
		err = unmarshalROSMessage[*mower_map.ReplaceMowingAreaSrvReq](io.NopCloser(bytes.NewReader(bodyBytes)), &CallReq)
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
			return
		}

		for i, element := range CallReq.Areas {
			element.Area.Active = true
			// Default all override fields to sentinel values (= use global parameter)
			element.Area.Angle = math.NaN()
			element.Area.OutlineCount = -1
			element.Area.OutlineOverlapCount = -1
			element.Area.OutlineOffset = math.NaN()
			// Apply explicit overrides only for fields the frontend actually sent
			if i < len(rawBody.Areas) {
				ro := rawBody.Areas[i].Area
				if ro.Angle != nil {
					element.Area.Angle = *ro.Angle
				}
				if ro.OutlineCount != nil {
					element.Area.OutlineCount = *ro.OutlineCount
				}
				if ro.OutlineOverlapCount != nil {
					element.Area.OutlineOverlapCount = *ro.OutlineOverlapCount
				}
				if ro.OutlineOffset != nil {
					element.Area.OutlineOffset = *ro.OutlineOffset
				}
			}
			err = provider.CallService(c.Request.Context(), "/mower_map_service/add_mowing_area", &mower_map.AddMowingAreaSrv{}, &element, &mower_map.AddMowingAreaSrvRes{})
			if err != nil {
				c.JSON(500, ErrorResponse{Error: err.Error()})
				return
			}
		}

		c.JSON(200, OkResponse{})
	})
}

// AreaDetailsRoute returns per-area override parameters.
//
// @Summary get area details
// @Description get per-area details including override parameters (nil = use global)
// @Tags openmower
// @Produce  json
// @Success 200 {array} MowingAreaDetails
// @Failure 500 {object} ErrorResponse
// @Router /openmower/map/areas [get]
func AreaDetailsRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.GET("/map/areas", func(c *gin.Context) {
		var details []MowingAreaDetails
		for i := 0; i < 64; i++ {
			req := mower_map.GetMowingAreaSrvReq{Index: uint32(i)}
			res := mower_map.GetMowingAreaSrvRes{}
			err := provider.CallService(c.Request.Context(), "/mower_map_service/get_mowing_area", &mower_map.GetMowingAreaSrv{}, &req, &res)
			if err != nil {
				// No area at this index — we've enumerated all areas
				break
			}
			area := res.Area
			d := MowingAreaDetails{
				Index:  i,
				Name:   area.Name,
				Active: area.Active,
			}
			// Sentinel values mean "no per-area override, use global parameter"
			if !math.IsNaN(area.Angle) {
				d.Angle = &area.Angle
			}
			if area.OutlineCount != -1 {
				d.OutlineCount = &area.OutlineCount
			}
			if area.OutlineOverlapCount != -1 {
				d.OutlineOverlapCount = &area.OutlineOverlapCount
			}
			if !math.IsNaN(area.OutlineOffset) {
				d.OutlineOffset = &area.OutlineOffset
			}
			details = append(details, d)
		}
		if details == nil {
			details = []MowingAreaDetails{}
		}
		c.JSON(200, details)
	})
}

// SetDockingPointRoute set the docking point
//
// @Summary set the docking point
// @Description set the docking point
// @Tags openmower
// @Accept  json
// @Produce  json
// @Param CallReq body mower_map.SetDockingPointSrvReq true "request body"
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /openmower/map/docking [post]
func SetDockingPointRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.POST("/map/docking", func(c *gin.Context) {
		var CallReq mower_map.SetDockingPointSrvReq
		err := unmarshalROSMessage[*mower_map.SetDockingPointSrvReq](c.Request.Body, &CallReq)
		if err != nil {
			return
		}
		err = provider.CallService(c.Request.Context(), "/mower_map_service/set_docking_point", &mower_map.SetDockingPointSrv{}, &CallReq, &mower_map.SetDockingPointSrvRes{})
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(200, OkResponse{})
		}
	})
}

// SubscriberRoute subscribe to a topic
//
// @Summary subscribe to a topic
// @Description subscribe to a topic
// @Tags openmower
// @Param topic path string true "topic to subscribe to, could be: diagnostics, status, gps, imu, ticks, highLevelStatus"
// @Router /openmower/subscribe/{topic} [get]
func SubscriberRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.GET("/subscribe/:topic", func(c *gin.Context) {
		// create a node and connect to the master
		var err error
		topic := c.Param("topic")
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		/*
		   this is where we handle the request context
		*/
		// create a subscriber
		var def func()
		switch topic {
		case "diagnostics":
			def, err = subscribe(provider, c, conn, "/diagnostics", -1)
		case "status":
			def, err = subscribe(provider, c, conn, "/ll/mower_status", -1)
		case "highLevelStatus":
			def, err = subscribe(provider, c, conn, "/mower_logic/current_state", -1)
		case "gps":
			def, err = subscribe(provider, c, conn, "/ll/position/gps", 100)
		case "pose":
			def, err = subscribe(provider, c, conn, "/xbot_positioning/xb_pose", 100)
		case "imu":
			def, err = subscribe(provider, c, conn, "/ll/imu/data_raw", 100)
		case "ticks":
			def, err = subscribe(provider, c, conn, "/xbot_positioning/wheel_ticks_in", 100)
		case "map":
			def, err = subscribe(provider, c, conn, "/xbot_monitoring/map", -1)
		case "path":
			def, err = subscribe(provider, c, conn, "/slic3r_coverage_planner/path_marker_array", -1)
		case "plan":
			def, err = subscribe(provider, c, conn, "/move_base_flex/FTCPlanner/global_plan", -1)
		case "mowingPath":
			def, err = subscribe(provider, c, conn, "/mowing_path", -1)
		case "power":
			def, err = subscribe(provider, c, conn, "/ll/power", -1)
		case "emergency":
			def, err = subscribe(provider, c, conn, "/ll/emergency", -1)
		case "dockingSensor":
			def, err = subscribe(provider, c, conn, "/mower/docking_sensor", -1)
		case "lidar":
			def, err = subscribe(provider, c, conn, "/scan", 100)
		}
		if err != nil {
			log.Println(err.Error())
			return
		}
		defer def()
		/*
		   send log lines to channel
		*/
		_, _, err = conn.ReadMessage()
		if err != nil {
			c.Error(err)
			return
		}
	})
}

// PublisherRoute publish to a topic
//
// @Summary publish to a topic
// @Description publish to a topic
// @Tags openmower
// @Param topic path string true "topic to publish to, could be: joy"
// @Router /openmower/publish/{topic} [get]
func PublisherRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	group.GET("/publish/:topic", func(c *gin.Context) {
		// create a node and connect to the master
		var err error
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Read messages from the websocket connection and publish them to ROS
		publisher, err := provider.Publisher("/joy_vel", &geometry_msgs.Twist{})
		if err != nil {
			c.Error(err)
			return
		}
		defer publisher.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				c.Error(err)
				break
			}
			var msgObj geometry_msgs.Twist
			err = json.Unmarshal(msg, &msgObj)
			if err != nil {
				c.Error(err)
				break
			}
			publisher.Write(&msgObj)
		}
	})
}

func subscribe(provider types.IRosProvider, c *gin.Context, conn *websocket.Conn, topic string, interval int) (func(), error) {
	id := uuid.Generate()
	uidString := id.String()
	err := provider.Subscribe(topic, uidString, func(msg []byte) {
		if interval > 0 {
			time.Sleep(time.Duration(interval) * time.Millisecond)
		}
		writer, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			c.Error(err)
			return
		}
		_, err = writer.Write([]byte(base64.StdEncoding.EncodeToString(msg)))
		if err != nil {
			c.Error(err)
			return
		}
		err = writer.Close()
		if err != nil {
			c.Error(err)
			return
		}
	},
	)
	if err != nil {
		return nil, err
	}
	return func() {
		provider.UnSubscribe(topic, uidString)
	}, nil
}

// ServiceRoute call a service
//
// @Summary call a service
// @Description call a service
// @Tags openmower
// @Accept  json
// @Produce  json
// @Param command path string true "command to call, could be: mower_start, mower_home, mower_s1, mower_s2, emergency, mow"
// @Param CallReq body map[string]interface{} true "request body"
// @Success 200 {object} OkResponse
// @Failure 500 {object} ErrorResponse
// @Router /openmower/call/{command} [post]
func ServiceRoute(group *gin.RouterGroup, provider types.IRosProvider) {
	// create a node and connect to the master
	group.POST("/call/:command", func(c *gin.Context) {
		// create a node and connect to the master
		command := c.Param("command")
		var err error
		switch command {
		case "high_level_control":
			var CallReq mower_msgs.HighLevelControlSrvReq
			err = c.BindJSON(&CallReq)
			if err != nil {
				return
			}
			err = provider.CallService(c.Request.Context(), "/mower_service/high_level_control", &mower_msgs.HighLevelControlSrv{}, &CallReq, &mower_msgs.HighLevelControlSrvRes{})
		case "emergency":
			var CallReq mower_msgs.EmergencyStopSrvReq
			err = c.BindJSON(&CallReq)
			if err != nil {
				return
			}
			err = provider.CallService(c.Request.Context(), "/ll/_service/emergency", &mower_msgs.EmergencyStopSrv{}, &CallReq, &mower_msgs.EmergencyStopSrvRes{})
		case "mower_logic":
			var CallReq dynamic_reconfigure.ReconfigureReq
			err = c.BindJSON(&CallReq)
			if err != nil {
				return
			}
			err = provider.CallService(c.Request.Context(), "/mower_logic/set_parameters", &dynamic_reconfigure.Reconfigure{}, &CallReq, &dynamic_reconfigure.ReconfigureRes{})
		case "mow_enabled":
			var CallReq mower_msgs.MowerControlSrvReq
			err = c.BindJSON(&CallReq)
			if err != nil {
				return
			}
			err = provider.CallService(c.Request.Context(), "/ll/_service/mow_enabled", &mower_msgs.MowerControlSrv{}, &CallReq, &mower_msgs.MowerControlSrvRes{})
		case "start_in_area":
			var CallReq mower_msgs.StartInAreaSrvReq
			err = c.BindJSON(&CallReq)
			if err != nil {
				return
			}
			err = provider.CallService(c.Request.Context(), "/mower_service/start_in_area", &mower_msgs.StartInAreaSrv{}, &CallReq, &mower_msgs.StartInAreaSrvRes{})
		default:
			err = errors.New("unknown command")
		}
		if err != nil {
			c.JSON(500, ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(200, OkResponse{})
		}
	})
}
