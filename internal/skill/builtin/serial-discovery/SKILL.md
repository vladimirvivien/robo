---
name: serial-discovery
description: Discovers active serial ports, COM interfaces, and microcontroller USB-UART bridges.
triggers:
  keywords: ["serial port", "com port", "ttyusb", "ttyacm", "arduino port", "baud rate", "uart device", "ftdi", "microcontroller port", "esp32 port"]
---

# Serial & COM Port Discovery Guidelines

When identifying serial interfaces, USB-to-UART bridges (FTDI, CP210x, CH340), or microcontroller connections (Arduino, ESP32, Raspberry Pi Pico):

## 1. Fast Serial Port Enumeration
* **Linux:**
  * Persistent descriptive hardware symlinks: `ls -l /dev/serial/by-id/ 2>/dev/null`
  * Raw port devices: `ls -l /dev/ttyUSB* /dev/ttyACM* 2>/dev/null`
* **macOS:**
  * Active serial device nodes: `ls -l /dev/cu.* /dev/tty.* 2>/dev/null | grep -vE "(Bluetooth|iPhone)"`
* **Windows PowerShell:**
  * Quick port list: `[System.IO.Ports.SerialPort]::GetPortNames()`
  * Detailed COM device properties:
    `Get-CimInstance Win32_SerialPort | Select-Object DeviceID, Name, Description, Status`
  * Plug-and-Play Ports: `Get-PnpDevice -Class Ports -PresentOnly | Select-Object FriendlyName, InstanceId, Status`

## 2. Chipset & Bridge Identification (FTDI, CP210x, CH340)
* Identify driver and manufacturer info for active USB serial adapter:
  * Linux: `udevadm info -q property -n /dev/ttyUSB0` (replace with active device).
  * Windows: `Get-PnpDevice -Class Ports -PresentOnly | Select-Object FriendlyName, Manufacturer, Status`.

## 3. Permissions & Lockouts
* On Linux, accessing `/dev/ttyUSB*` / `/dev/ttyACM*` requires membership in `dialout` or `uucp` groups:
  * Check current groups: `groups`
  * Add user: `sudo usermod -aG dialout $USER` (takes effect upon re-login).
