#!/usr/bin/env python3
"""
Configure u-blox ZED-F9P/F9R GPS output protocol on USB.

The ZED-F9P is connected to the Raspberry Pi via USB (/dev/gps → /dev/ttyACM0).
This script switches the GPS between:

  UBX   – u-blox binary protocol (required by xbot_driver_gps when OM_GPS_PROTOCOL=UBX)
  NMEA  – standard NMEA-0183 sentences (required when OM_GPS_PROTOCOL=NMEA)

After a firmware flash or ROM-BOOT recovery the ZED-F9P reverts to NMEA output on USB.
Run with --protocol UBX to restore UBX mode without re-flashing the full GPS config file.

Config is saved to RAM + BBR + Flash (layers=0x07) so it survives power cycles.

Usage:
  python3 configure_gps_protocol.py --protocol UBX
  python3 configure_gps_protocol.py --protocol NMEA
  python3 configure_gps_protocol.py --protocol UBX --dry-run
"""

import argparse
import struct
import sys
import time
import serial

# ---------------------------------------------------------------------------
# UBX helpers
# ---------------------------------------------------------------------------

def ubx_checksum(data: bytes):
    ck_a = ck_b = 0
    for b in data:
        ck_a = (ck_a + b) & 0xFF
        ck_b = (ck_b + ck_a) & 0xFF
    return ck_a, ck_b


def build_ubx(cls: int, msg_id: int, payload: bytes) -> bytes:
    length = struct.pack('<H', len(payload))
    body = bytes([cls, msg_id]) + length + payload
    ck_a, ck_b = ubx_checksum(body)
    return b'\xb5\x62' + body + bytes([ck_a, ck_b])


def build_valset(configs: list) -> bytes:
    """
    Build a UBX-CFG-VALSET (06 8A) packet with multiple key/value pairs.
    configs: list of (key: int, value: int, size_bytes: int)
    Layers 0x07 = RAM | BBR | Flash – config persists across power cycles.
    """
    # version=0, layers=0x07 (RAM+BBR+Flash), reserved=0x0000
    payload = struct.pack('<BBH', 0x00, 0x07, 0x0000)
    for key, value, nbytes in configs:
        payload += struct.pack('<I', key)
        if nbytes == 1:
            payload += struct.pack('<B', value)
        elif nbytes == 2:
            payload += struct.pack('<H', value)
        elif nbytes == 4:
            payload += struct.pack('<I', value)
    return build_ubx(0x06, 0x8A, payload)


def wait_for_ack(ser: serial.Serial, cls: int, msg_id: int, timeout: float = 2.0) -> str:
    """
    Wait for a UBX-ACK-ACK or UBX-ACK-NAK for the given (cls, msg_id).
    Returns 'ACK', 'NAK', or 'TIMEOUT'.

    Note: when the ROS GPS driver is running alongside this script it may
    consume the ACK bytes before we read them. TIMEOUT does NOT mean failure –
    the GPS still applies the configuration.
    """
    deadline = time.monotonic() + timeout
    buf = b''
    while time.monotonic() < deadline:
        chunk = ser.read(64)
        if chunk:
            buf += chunk
        idx = 0
        while idx + 1 < len(buf):
            if buf[idx] == 0xb5 and buf[idx + 1] == 0x62:
                if idx + 6 > len(buf):
                    break
                pkt_cls = buf[idx + 2]
                pkt_id  = buf[idx + 3]
                pkt_len = struct.unpack_from('<H', buf, idx + 4)[0]
                total   = 8 + pkt_len
                if idx + total > len(buf):
                    break
                if pkt_cls == 0x05 and pkt_len >= 2:
                    acked_cls = buf[idx + 6]
                    acked_id  = buf[idx + 7]
                    if acked_cls == cls and acked_id == msg_id:
                        return 'ACK' if pkt_id == 0x01 else 'NAK'
                buf = buf[idx + total:]
                idx = 0
            else:
                idx += 1
    return 'TIMEOUT'


# ---------------------------------------------------------------------------
# Configuration tables (all keys target the USB port)
# ---------------------------------------------------------------------------

# Enable UBX binary output on USB, disable NMEA
UBX_CONFIGS = [
    # (name, key, value, size_bytes)
    ("CFG-USBOUTPROT-UBX",           0x10780001, 1, 1),  # enable UBX on USB
    ("CFG-USBOUTPROT-NMEA",          0x10780002, 0, 1),  # disable NMEA on USB
    ("CFG-USBINPROT-UBX",            0x10770001, 1, 1),  # keep UBX input (config cmds + RTCM)
    ("CFG-MSGOUT-UBX_NAV_PVT_USB",   0x20910009, 1, 1),  # position/velocity/time @ 1 Hz
]

# Enable NMEA output on USB, disable UBX binary
NMEA_CONFIGS = [
    ("CFG-USBOUTPROT-UBX",            0x10780001, 0, 1),  # disable UBX on USB
    ("CFG-USBOUTPROT-NMEA",           0x10780002, 1, 1),  # enable NMEA on USB
    ("CFG-USBINPROT-UBX",             0x10770001, 1, 1),  # keep UBX input (config cmds + RTCM)
    ("CFG-MSGOUT-UBX_NAV_PVT_USB",    0x20910009, 0, 1),  # disable NAV-PVT (not needed for NMEA)
    ("CFG-MSGOUT-NMEA_ID_GGA_USB",    0x209100bd, 1, 1),  # GGA – fix, lat, lon, alt @ 1 Hz
    ("CFG-MSGOUT-NMEA_ID_RMC_USB",    0x209100ae, 1, 1),  # RMC – speed, course @ 1 Hz
    ("CFG-MSGOUT-NMEA_ID_VTG_USB",    0x209100b3, 1, 1),  # VTG – track/speed @ 1 Hz
]


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument('--protocol', required=True, choices=['UBX', 'NMEA'],
                        help='Target GPS output protocol')
    parser.add_argument('--port',    default='/dev/gps',
                        help='Serial/USB device (default: /dev/gps → /dev/ttyACM0)')
    parser.add_argument('--baud',    type=int, default=460800,
                        help='Baud rate (default: 460800; ignored on USB CDC)')
    parser.add_argument('--dry-run', action='store_true',
                        help='Build packets but do not send them')
    args = parser.parse_args()

    protocol = args.protocol.upper()
    configs = UBX_CONFIGS if protocol == 'UBX' else NMEA_CONFIGS

    print(f"Configuring GPS for {protocol} output on {args.port} …")

    if not args.dry_run:
        try:
            ser = serial.Serial(args.port, args.baud, timeout=0.5)
        except serial.SerialException as exc:
            print(f"ERROR: cannot open port: {exc}", file=sys.stderr)
            sys.exit(1)
        ser.reset_input_buffer()
        ser.reset_output_buffer()
        time.sleep(0.2)

    kv = [(key, value, size) for (_, key, value, size) in configs]
    pkt = build_valset(kv)

    print(f"[{protocol} config – {len(configs)} keys, {len(pkt)} bytes]")
    print(' '.join(f'{b:02x}' for b in pkt))

    if args.dry_run:
        print("[dry-run] Done. No data was sent.")
        return

    print("Sending … ", end='', flush=True)
    ser.write(pkt)
    result = wait_for_ack(ser, cls=0x06, msg_id=0x8A, timeout=2.0)
    print(result)

    ser.close()

    if result == 'NAK':
        print("ERROR: GPS rejected the configuration.", file=sys.stderr)
        sys.exit(1)

    if result == 'TIMEOUT':
        print("  → No ACK received (the ROS driver may have consumed it).")
        print("    The GPS still applies the configuration. This is normal.")

    print(f"""
Done — GPS output switched to {protocol}.
  • Config saved to Flash (layers=0x07) and will survive power cycling.
  • If the mowgli-openmower container is running, changes take effect immediately.
    For UBX→NMEA or NMEA→UBX switches the driver needs a restart to re-sync.
""")


if __name__ == '__main__':
    main()
