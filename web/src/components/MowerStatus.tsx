import { useHighLevelStatus } from "../hooks/useHighLevelStatus.ts";
import { useGPS } from "../hooks/useGPS.ts";
import { Col, Row, Statistic } from "antd";

import { BatteryCharge, BatteryEmpty, BatteryLow, BatteryMid, BatteryHigh, BatteryFull} from "./IconBatteryComponent.tsx";
import { MapPinCheck, MapPinApproximation, MapPinOff, BroadcastTower, BroadcastTowerOff} from "./IconGeolocationComponent.tsx";
import { stateRenderer } from "./utils.tsx";
import { AbsolutePoseFlags as Flags } from "../types/ros.ts";
import { useEffect, useState } from "react";

const getBatteryIcon = (batteryLevel: number) => {
  if (batteryLevel > 0.99) return <BatteryFull />;
  if (batteryLevel > 0.66) return <BatteryHigh />;
  if (batteryLevel > 0.33) return <BatteryMid />;
  if (batteryLevel < 0.1) return <BatteryLow />;
  return <BatteryLow stroke={'grey'} fill={'grey'} />;
};

const getNextBatteryIcon = (batteryLevel: number) => {
    if (batteryLevel > 0.9) return <BatteryCharge />;
    if (batteryLevel > 0.66) return <BatteryFull />;
    if (batteryLevel > 0.33) return <BatteryHigh />;
    if (batteryLevel > 0.1) return <BatteryMid />;
    return <BatteryLow stroke={'red'} fill={'red'} />;
  };

  const getRTKIcon = (flags: number, rtk: number) => {
        if ((flags & rtk) !== 0) {
            return <><BroadcastTower  {...{strokeWidth: '1', color: '#049F0B'}} /></>;
        } else {
            return <><BroadcastTowerOff  {...{strokeWidth: '1', color: '#EB0000'}} /></>;
        }
    }

    const getGPSIcon = (flags: number) => {

        if ((flags & Flags.FIXED) != 0) {
          // fixType = "FIX";
          return <><MapPinCheck {...{strokeWidth: '1', color: '#049F0B'}} /></>;
        } else if ((flags & Flags.FLOAT) != 0) {
          // fixType = "FLOAT";
          return <><MapPinApproximation {...{strokeWidth: '1', color: '#E03300'}}/></>;
        } else {
            return <><MapPinOff {...{strokeWidth: '1', color: '#E00000'}}/></>;
        }
    }

export const MowerStatus = () => {
  const { highLevelStatus } = useHighLevelStatus();
  const gps = useGPS();

  const flags = gps.Flags ?? 0;

  const batteryCharging = highLevelStatus.IsCharging;
  const batteryLevel = highLevelStatus.BatteryPercent ?? 0;
  const [animationStep, setAnimationStep] = useState(0);

  useEffect(() => {
    if (batteryCharging) {
      const interval = setInterval(() => {
        setAnimationStep((prev) => (prev === 0 ? 1 : 0));
      }, 500); // Change the interval duration as needed
      return () => clearInterval(interval);
    }
  }, [batteryCharging]);

  let batteryIcon;
  if (batteryCharging) {
    batteryIcon = (
      <>
        {animationStep === 0 ? getBatteryIcon(batteryLevel) : getNextBatteryIcon(batteryLevel)}
      </>
    );
  } else {
    batteryIcon = getBatteryIcon(batteryLevel);
  }

    let RTKIcon = getRTKIcon(flags, Flags.RTK);

    let GPSIcon = getGPSIcon(flags);

  return (
    <Row gutter={[16, 16]} style={{ margin: 0 }}>
      <Col>
        <Statistic
          valueStyle={{ color: "#3f8600", fontSize: "16px" }}
          value={stateRenderer(highLevelStatus.StateName)}
        />
      </Col>
      <Col>
        <>
        {RTKIcon}
        </>
      </Col>
      <Col>
        <Statistic
          prefix={GPSIcon}
          valueStyle={{ fontSize: "13px" }}
          precision={1}
          value={gps.PositionAccuracy !== undefined ? gps.PositionAccuracy * 100 : "?"}
          suffix={"cm"}
        />
      </Col>
      <Col>
        <Statistic
          prefix={<span style={{height: '24px'}}>{batteryIcon}</span>}
          valueStyle={{ fontSize: "13px", height: "24px" }}
          precision={0}
          value={batteryLevel * 100}
          suffix={"%"}
        />
      </Col>
    </Row>
  );
};
