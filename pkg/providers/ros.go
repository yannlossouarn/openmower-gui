package providers

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"

	"github.com/bluenviron/goroslib/v2"
	"github.com/bluenviron/goroslib/v2/pkg/msgs/geometry_msgs"
	"github.com/bluenviron/goroslib/v2/pkg/msgs/nav_msgs"
	"github.com/bluenviron/goroslib/v2/pkg/msgs/sensor_msgs"
	"github.com/bluenviron/goroslib/v2/pkg/msgs/visualization_msgs"
	"github.com/cedbossneo/openmower-gui/pkg/msgs/mower_msgs"
	"github.com/cedbossneo/openmower-gui/pkg/msgs/std_msgs"
	"github.com/cedbossneo/openmower-gui/pkg/msgs/xbot_msgs"
	types2 "github.com/cedbossneo/openmower-gui/pkg/types"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/simplify"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

type RosSubscriber struct {
	Topic       string
	Id          string
	mtx         *sync.Mutex
	cb          func(msg []byte)
	nextMessage []byte
	close       chan bool
}

func NewRosSubscriber(topic, id string, cb func(msg []byte)) *RosSubscriber {
	r := &RosSubscriber{
		cb:    cb,
		Topic: topic,
		Id:    id,
		mtx:   &sync.Mutex{},
		close: make(chan bool),
	}
	go r.Run()
	return r
}

func (r *RosSubscriber) Publish(msg []byte) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.nextMessage = msg
}

func (r *RosSubscriber) Close() {
	r.close <- true
}

func (r *RosSubscriber) Run() {
	for {
		select {
		case <-r.close:
			return
		default:
			r.mtx.Lock()
			messageToProcess := r.nextMessage
			r.nextMessage = nil
			r.mtx.Unlock()
			r.processMessage(messageToProcess)
		}
	}
}

func (r *RosSubscriber) processMessage(messageToProcess []byte) {
	if messageToProcess != nil {
		r.cb(messageToProcess)
	} else {
		time.Sleep(100 * time.Millisecond)
	}
}

// RosProvider holds a ROS node and manages topic subscriptions on behalf of
// browser WebSocket clients. goroslib subscribers are created lazily: a ROS
// subscription is opened only when the first browser client subscribes to a
// topic and closed when the last browser client unsubscribes.
//
// The sole exception is /xbot_positioning/xb_pose, which is kept permanently
// active by the internal mowing-path tracker (id "gui").
type RosProvider struct {
	node           *goroslib.Node
	mtx            sync.Mutex
	rosSubscribers map[string]*goroslib.Subscriber  // topic -> active goroslib subscriber
	subscribers    map[string]map[string]*RosSubscriber
	lastMessage    map[string][]byte
	mowingPaths    []*nav_msgs.Path
	mowingPath     *nav_msgs.Path
	mowingPathOrigin orb.LineString
	dbProvider     types2.IDBProvider
}

func (p *RosProvider) getNode() (*goroslib.Node, error) {
	var err error
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if p.node != nil {
		return p.node, err
	}

	nodeName, err := p.dbProvider.Get("system.ros.nodeName")
	if err != nil {
		return nil, err
	}
	masterUri, err := p.dbProvider.Get("system.ros.masterUri")
	if err != nil {
		return nil, err
	}
	nodeHost, err := p.dbProvider.Get("system.ros.nodeHost")
	if err != nil {
		return nil, err
	}
	p.node, err = goroslib.NewNode(goroslib.NodeConf{
		Name:          string(nodeName),
		MasterAddress: string(masterUri),
		Host:          string(nodeHost),
		ReadTimeout:   time.Minute,
		WriteTimeout:  time.Minute,
	})
	return p.node, err
}

func NewRosProvider(dbProvider types2.IDBProvider) types2.IRosProvider {
	r := &RosProvider{
		dbProvider:     dbProvider,
		subscribers:    make(map[string]map[string]*RosSubscriber),
		lastMessage:    make(map[string][]byte),
		rosSubscribers: make(map[string]*goroslib.Subscriber),
	}

	// Start the internal mowing-path tracker. This is the only subscription
	// opened unconditionally at startup; all browser-facing subscriptions are
	// created on demand when the first client connects.
	if err := r.initMowingPathSubscriber(); err != nil {
		logrus.Error(err)
	}

	go func() {
		for range time.Tick(20 * time.Second) {
			node, err := r.getNode()
			if err != nil {
				logrus.Error(xerrors.Errorf("failed to get node: %w", err))
				continue
			}
			_, err = node.NodePing("rosout")
			if err != nil {
				logrus.Error(xerrors.Errorf("failed to ping node: %w, restarting node", err))
				r.resetSubscribers()
			} else {
				r.reconnectSubscribers()
			}
		}
	}()
	return r
}

// newRosSubscriber creates a type-specific goroslib.Subscriber for a known
// topic. Returns (nil, nil) for synthetic topics that have no direct ROS
// backing (e.g. /mowing_path and /xbot_monitoring/map which are populated
// internally).
func (p *RosProvider) newRosSubscriber(topic string, node *goroslib.Node) (*goroslib.Subscriber, error) {
	conf := goroslib.SubscriberConf{Node: node, Topic: topic, QueueSize: 1}
	switch topic {
	case "/ll/mower_status":
		conf.Callback = cbHandler[*mower_msgs.Status](p, topic)
	case "/mower_logic/current_state":
		conf.Callback = cbHandler[*mower_msgs.HighLevelStatus](p, topic)
	case "/ll/position/gps":
		conf.Callback = cbHandler[*xbot_msgs.AbsolutePose](p, topic)
	case "/xbot_positioning/xb_pose":
		conf.Callback = cbHandler[*xbot_msgs.AbsolutePose](p, topic)
	case "/ll/imu/data_raw":
		conf.Callback = cbHandler[*sensor_msgs.Imu](p, topic)
	case "/xbot_positioning/wheel_ticks_in":
		conf.Callback = cbHandler[*xbot_msgs.WheelTick](p, topic)
	case "/mower_map_service/json_map":
		conf.Callback = p.jsonMapHandler
	case "/slic3r_coverage_planner/path_marker_array":
		conf.Callback = cbHandler[*visualization_msgs.MarkerArray](p, topic)
	case "/move_base_flex/FTCPlanner/global_plan":
		conf.Callback = cbHandler[*nav_msgs.Path](p, topic)
	case "/ll/power":
		conf.Callback = cbHandler[*mower_msgs.Power](p, topic)
	case "/ll/emergency":
		conf.Callback = cbHandler[*mower_msgs.Emergency](p, topic)
	case "/mower/docking_sensor":
		conf.Callback = cbHandler[*mower_msgs.DockingSensor](p, topic)
	case "/scan":
		conf.Callback = cbHandler[*sensor_msgs.LaserScan](p, topic)
	case "/mowing_path", "/xbot_monitoring/map":
		return nil, nil // synthetic: populated internally, no goroslib subscriber
	default:
		logrus.Warnf("RosProvider: no goroslib factory for topic %q; messages will not flow", topic)
		return nil, nil
	}
	sub, err := goroslib.NewSubscriber(conf)
	if err != nil {
		return nil, err
	}
	logrus.Infof("Subscribed to %s", topic)
	return sub, nil
}

// ensureRosSubscriber lazily creates the goroslib subscriber for topic if one
// does not already exist. It is safe to call concurrently.
func (p *RosProvider) ensureRosSubscriber(topic string) error {
	p.mtx.Lock()
	_, exists := p.rosSubscribers[topic]
	p.mtx.Unlock()
	if exists {
		return nil
	}

	node, err := p.getNode()
	if err != nil {
		return err
	}

	sub, err := p.newRosSubscriber(topic, node)
	if err != nil {
		return err
	}
	if sub == nil {
		return nil // synthetic or unknown topic — no goroslib subscriber needed
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()
	if _, exists := p.rosSubscribers[topic]; !exists {
		p.rosSubscribers[topic] = sub
	} else {
		sub.Close() // lost the race — discard the duplicate
	}
	return nil
}

func (p *RosProvider) resetSubscribers() {
	// Step 1: tear down the node so goroslib stops delivering messages.
	if p.node != nil {
		p.node.Close()
		p.node = nil
	}

	// Step 2: under the lock, close all goroslib subscribers and extract the
	// "gui" mowing-path RosSubscriber for deferred close (its callback holds
	// p.mtx, so we must not close it while the lock is held).
	p.mtx.Lock()
	for topic, sub := range p.rosSubscribers {
		sub.Close()
		delete(p.rosSubscribers, topic)
	}
	var guiRosSub *RosSubscriber
	if xbPose, ok := p.subscribers["/xbot_positioning/xb_pose"]; ok {
		if g, ok := xbPose["gui"]; ok {
			guiRosSub = g
			delete(xbPose, "gui")
		}
	}
	p.mowingPaths = []*nav_msgs.Path{}
	p.mowingPath = nil
	p.mowingPathOrigin = nil
	p.mtx.Unlock()

	// Step 3: close the "gui" subscriber after releasing the lock.
	if guiRosSub != nil {
		guiRosSub.Close()
	}
}

// reconnectSubscribers restores subscriptions after the node reconnects.
// It recreates goroslib subscribers for any topic that still has active
// browser clients, and restores the internal mowing-path tracker if needed.
func (p *RosProvider) reconnectSubscribers() {
	// Restore the mowing-path internal subscriber if it was cleared by reset.
	p.mtx.Lock()
	var guiExists bool
	if xbPose, ok := p.subscribers["/xbot_positioning/xb_pose"]; ok {
		_, guiExists = xbPose["gui"]
	}
	p.mtx.Unlock()

	if !guiExists {
		if err := p.initMowingPathSubscriber(); err != nil {
			logrus.Error(xerrors.Errorf("failed to restore mowing path subscriber: %w", err))
		}
	}

	// Recreate goroslib subscribers for topics with active browser clients
	// that lost their backing during the reset.
	p.mtx.Lock()
	topicsNeedingReconnect := make([]string, 0, len(p.subscribers))
	for topic, subs := range p.subscribers {
		if len(subs) > 0 {
			if _, exists := p.rosSubscribers[topic]; !exists {
				topicsNeedingReconnect = append(topicsNeedingReconnect, topic)
			}
		}
	}
	p.mtx.Unlock()

	for _, topic := range topicsNeedingReconnect {
		if err := p.ensureRosSubscriber(topic); err != nil {
			logrus.Error(xerrors.Errorf("failed to reconnect to %s: %w", topic, err))
		}
	}
}

func (p *RosProvider) initMowingPathSubscriber() error {
	err := p.Subscribe("/xbot_positioning/xb_pose", "gui", func(msg []byte) {
		p.mtx.Lock()
		defer p.mtx.Unlock()
		var pose xbot_msgs.AbsolutePose
		err := json.Unmarshal(msg, &pose)
		if err != nil {
			logrus.Error(xerrors.Errorf("failed to unmarshal pose: %w", err))
			return
		}
		hlsLastMessage, ok := p.lastMessage["/mower_logic/current_state"]
		if ok {
			var highLevelStatus mower_msgs.HighLevelStatus
			err := json.Unmarshal(hlsLastMessage, &highLevelStatus)
			if err != nil {
				logrus.Error(xerrors.Errorf("failed to unmarshal high level status: %w", err))
				return
			}
			switch highLevelStatus.StateName {
			case "MOWING":
				sLastMessage, ok := p.lastMessage["/ll/mower_status"]
				if ok {
					var status mower_msgs.Status
					err := json.Unmarshal(sLastMessage, &status)
					if err != nil {
						logrus.Error(xerrors.Errorf("failed to unmarshal status: %w", err))
						return
					}
					if status.MowerMotorRpm > 0 {
						if p.mowingPath == nil {
							p.mowingPath = &nav_msgs.Path{}
							p.mowingPathOrigin = orb.LineString{}
							p.mowingPaths = append(p.mowingPaths, p.mowingPath)
						}
						p.mowingPathOrigin = append(p.mowingPathOrigin, orb.Point{
							pose.Pose.Pose.Position.X, pose.Pose.Pose.Position.Y,
						})
						if len(p.mowingPathOrigin)%5 == 0 {
							// low threshold just removes the colinear point
							reduced := simplify.DouglasPeucker(0.03).LineString(p.mowingPathOrigin.Clone())
							p.mowingPath.Poses = lo.Map(reduced, func(p orb.Point, idx int) geometry_msgs.PoseStamped {
								return geometry_msgs.PoseStamped{
									Pose: geometry_msgs.Pose{
										Position: geometry_msgs.Point{
											X: p[0],
											Y: p[1],
										},
									},
								}
							})
						}
						msgJson, _ := json.Marshal(p.mowingPaths)
						p.lastMessage["/mowing_path"] = msgJson
						subscribers, hasSubscriber := p.subscribers["/mowing_path"]
						if hasSubscriber {
							for _, cb := range subscribers {
								cb.Publish(msgJson)
							}
						}
					} else {
						p.mowingPath = nil
						p.mowingPathOrigin = nil
					}
				}
				break
			default:
				p.mowingPaths = []*nav_msgs.Path{}
				p.mowingPath = nil
				p.mowingPathOrigin = nil
			}
		}
	})
	return err
}

func (p *RosProvider) CallService(ctx context.Context, srvName string, srv any, req any, res any) error {
	rosNode, err := p.getNode()
	if err != nil {
		return err
	}
	serviceClient, err := goroslib.NewServiceClient(goroslib.ServiceClientConf{
		Node: rosNode,
		Name: srvName,
		Srv:  srv,
	})
	if err != nil {
		return err
	}
	defer serviceClient.Close()
	err = serviceClient.CallContext(ctx, req, res)
	if err != nil {
		return err
	}
	return nil
}

// Subscribe registers cb to receive JSON-encoded messages for topic. The
// underlying goroslib subscriber is created lazily on first call for a given
// topic and torn down when the last subscriber unregisters.
func (p *RosProvider) Subscribe(topic string, id string, cb func(msg []byte)) error {
	if err := p.ensureRosSubscriber(topic); err != nil {
		return err
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.subscribers[topic] == nil {
		p.subscribers[topic] = make(map[string]*RosSubscriber)
	}
	if _, hasCallback := p.subscribers[topic][id]; !hasCallback {
		p.subscribers[topic][id] = NewRosSubscriber(topic, id, cb)
	}
	if lastMessage, hasLastMessage := p.lastMessage[topic]; hasLastMessage {
		p.subscribers[topic][id].Publish(lastMessage)
	}
	return nil
}

func (p *RosProvider) Publisher(topic string, obj interface{}) (*goroslib.Publisher, error) {
	rosNode, err := p.getNode()
	if err != nil {
		return nil, err
	}
	publisher, err := goroslib.NewPublisher(goroslib.PublisherConf{
		Node:  rosNode,
		Topic: topic,
		Msg:   obj,
	})
	return publisher, nil
}

// UnSubscribe removes the subscriber identified by (topic, id). When the last
// subscriber for a topic is removed, the backing goroslib subscriber is closed
// and the ROS subscription is dropped.
func (p *RosProvider) UnSubscribe(topic string, id string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	topicSubs, ok := p.subscribers[topic]
	if !ok {
		return
	}
	sub, exists := topicSubs[id]
	if !exists {
		return
	}
	sub.Close()
	delete(topicSubs, id)

	// Close the goroslib subscriber when the last client leaves.
	if len(topicSubs) == 0 {
		if rosSub, ok := p.rosSubscribers[topic]; ok {
			rosSub.Close()
			delete(p.rosSubscribers, topic)
			logrus.Infof("Unsubscribed from %s (no remaining clients)", topic)
		}
	}
}

func cbHandler[T any](p *RosProvider, topic string) func(msg T) {
	return func(msg T) {
		p.mtx.Lock()
		defer p.mtx.Unlock()
		msgJson, err := json.Marshal(msg)
		if err != nil {
			logrus.Error(xerrors.Errorf("failed to marshal message: %w", err))
			return
		}
		p.lastMessage[topic] = msgJson
		subscribers, hasSubscriber := p.subscribers[topic]
		if hasSubscriber {
			for _, cb := range subscribers {
				cb.Publish(msgJson)
			}
		}
	}
}

// jsonMapPoint represents a point in the JSON map format
type jsonMapPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// jsonMapArea represents an area in the JSON map format
type jsonMapArea struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties"`
	Outline    []jsonMapPoint         `json:"outline"`
}

// jsonDockingStation represents a docking station in the JSON map format
type jsonDockingStation struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties"`
	Position   jsonMapPoint           `json:"position"`
	Heading    float64                `json:"heading"`
}

// jsonMapData represents the full JSON map from mower_map_service
type jsonMapData struct {
	Areas           []jsonMapArea        `json:"areas"`
	DockingStations []jsonDockingStation `json:"docking_stations"`
}

func (p *RosProvider) jsonMapHandler(msg *std_msgs.String) {
	var mapData jsonMapData
	if err := json.Unmarshal([]byte(msg.Data), &mapData); err != nil {
		logrus.Error(xerrors.Errorf("failed to parse JSON map: %w", err))
		return
	}

	// Convert to xbot_msgs.Map format
	var result xbot_msgs.Map

	// Calculate map bounds
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64

	// The JSON areas are flat: mow/nav/obstacle entries in order.
	// Obstacles follow their parent mowing area (implicit ordering from mower_map_service).
	// We need to reconstruct the nested structure the frontend expects.
	var lastMowIndex int = -1

	for _, area := range mapData.Areas {
		areaType, _ := area.Properties["type"].(string)

		outlineToPolygon := func(outline []jsonMapPoint) geometry_msgs.Polygon {
			var poly geometry_msgs.Polygon
			for _, pt := range outline {
				poly.Points = append(poly.Points, geometry_msgs.Point32{
					X: float32(pt.X), Y: float32(pt.Y), Z: 0,
				})
			}
			return poly
		}

		switch areaType {
		case "obstacle":
			// Attach to the last mowing area
			if lastMowIndex >= 0 && lastMowIndex < len(result.WorkingArea) {
				result.WorkingArea[lastMowIndex].Obstacles = append(
					result.WorkingArea[lastMowIndex].Obstacles,
					outlineToPolygon(area.Outline),
				)
			}
		case "nav":
			var mapArea xbot_msgs.MapArea
			if name, ok := area.Properties["name"].(string); ok {
				mapArea.Name = name
			}
			mapArea.Area = outlineToPolygon(area.Outline)
			result.NavigationAreas = append(result.NavigationAreas, mapArea)
		default: // "mow" or any other type treated as mowing area
			var mapArea xbot_msgs.MapArea
			if name, ok := area.Properties["name"].(string); ok {
				mapArea.Name = name
			}
			mapArea.Area = outlineToPolygon(area.Outline)
			result.WorkingArea = append(result.WorkingArea, mapArea)
			lastMowIndex = len(result.WorkingArea) - 1
		}

		// Update bounds from all area types
		for _, pt := range area.Outline {
			if pt.X < minX {
				minX = pt.X
			}
			if pt.X > maxX {
				maxX = pt.X
			}
			if pt.Y < minY {
				minY = pt.Y
			}
			if pt.Y > maxY {
				maxY = pt.Y
			}
		}
	}

	if minX != math.MaxFloat64 {
		result.MapWidth = maxX - minX
		result.MapHeight = maxY - minY
		result.MapCenterX = (minX + maxX) / 2
		result.MapCenterY = (minY + maxY) / 2
	}

	if len(mapData.DockingStations) > 0 {
		dock := mapData.DockingStations[0]
		result.DockX = dock.Position.X
		result.DockY = dock.Position.Y
		result.DockHeading = dock.Heading
	}

	// Publish as the topic the GUI expects
	const topic = "/xbot_monitoring/map"
	p.mtx.Lock()
	defer p.mtx.Unlock()
	msgJson, err := json.Marshal(result)
	if err != nil {
		logrus.Error(xerrors.Errorf("failed to marshal map: %w", err))
		return
	}
	p.lastMessage[topic] = msgJson
	subscribers, hasSubscriber := p.subscribers[topic]
	if hasSubscriber {
		for _, cb := range subscribers {
			cb.Publish(msgJson)
		}
	}
}
