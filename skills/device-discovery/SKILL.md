---
name: device-discovery
description: Discovers attached hardware devices (USB, PCI, GPU, serial, storage), driver bindings, vendor IDs, and device status.
version: 1.0.0
triggers:
  keywords: ["attached devices", "usb devices", "pci devices", "hardware devices", "list devices", "device manager", "driver info", "serial ports", "com ports", "gpu info", "lspci", "lsusb", "system_profiler", "Get-PnpDevice", "hardware inventory"]
---

# Attached Device & Hardware Discovery Guidelines

When inspecting attached hardware peripherals, buses, drivers, or device capabilities:

## 1. USB Peripheral Inspection
Query connected USB devices with Vendor/Product IDs (`VID:PID`) and serial numbers:
* **Linux:** `lsusb -v` (summary: `lsusb` or `udevadm info --export-db | grep ID_MODEL`).
* **macOS:** `system_profiler SPUSBDataType` (or JSON: `system_profiler SPUSBDataType -json`).
* **Windows PowerShell:**
  `Get-PnpDevice -Class USB -PresentOnly | Select-Object FriendlyName, InstanceId, Status`
  (To query detailed USB tree: `Get-CimInstance Win32_USBControllerDevice | ForEach-Object { [Wmi]$_.Dependent } | Select-Object Name, DeviceID, Status`).

## 2. PCIe & GPU Accelerators
Identify discrete/integrated graphics, AI accelerators, and active kernel driver bindings:
* **Linux:**
  * PCIe device listing with driver in use: `lspci -nnk`.
  * Graphics controllers only: `lspci -nnk | grep -A 3 -i "vga\|3d\|display"`.
  * NVIDIA GPU telemetry (if present): `nvidia-smi --query-gpu=name,driver_version,memory.total,utilization.gpu --format=csv`.
* **macOS:** `system_profiler SPDisplaysDataType`.
* **Windows PowerShell:**
  `Get-CimInstance Win32_VideoController | Select-Object Name, DriverVersion, @{Name="VRAM(MB)";Expression={[math]::Round($_.AdapterRAM/1MB,2)}}, Status`.

## 3. Serial & COM Port Discovery (Microcontrollers & Embedded)
Identify active serial connections, Arduino/FTDI chips, and communication ports:
* **Linux:**
  * Persistent hardware symlinks: `ls -l /dev/serial/by-id/ 2>/dev/null`
  * Raw port devices: `ls -l /dev/ttyUSB* /dev/ttyACM* 2>/dev/null`
* **macOS:** `ls -l /dev/cu.* /dev/tty.* 2>/dev/null | grep -vE "(Bluetooth|iPhone)"`
* **Windows PowerShell:**
  `Get-CimInstance Win32_SerialPort | Select-Object DeviceID, Name, Description`
  (or quick list: `[System.IO.Ports.SerialPort]::GetPortNames()`).

## 4. Physical Storage & Bus Types
Inspect physical drive models, serial numbers, media types, and transport bus (NVMe/SATA/USB):
* **Linux:** `lsblk -o NAME,SIZE,TYPE,MODEL,SERIAL,TRAN,FSTYPE,MOUNTPOINT`.
* **macOS:** `diskutil list` and `system_profiler SPStorageDataType`.
* **Windows PowerShell:**
  `Get-PhysicalDisk | Select-Object DeviceId, FriendlyName, MediaType, BusType, @{Name="Size(GB)";Expression={[math]::Round($_.Size/1GB,2)}}, OperationalStatus`.

## 5. Network Adapters & Link Hardware
* **Linux:** `ip -br link` (or query interface capabilities: `ethtool <interface>`).
* **macOS:** `networksetup -listallhardwareports`.
* **Windows PowerShell:**
  `Get-NetAdapter | Select-Object Name, InterfaceDescription, Status, LinkSpeed, MacAddress, DriverVersion`.

## 6. Audio Devices & Video Capture
* **Linux:** `v4l2-ctl --list-devices 2>/dev/null` (Video) and `arecord -l && aplay -l` (Audio).
* **macOS:** `system_profiler SPCameraDataType` and `system_profiler SPAudioDataType`.
* **Windows PowerShell:** `Get-PnpDevice -Class Camera, Media -PresentOnly | Select-Object FriendlyName, Status`.

## 7. Driver Diagnostics & Hotplug Events
When diagnosing malfunctioning devices or inspecting recent connection/disconnection events:
* **Linux:** Check kernel ring buffer: `dmesg -T | grep -iE "usb|pci|driver|attached|detached|error" | tail -n 30`.
* **Windows PowerShell:** Inspect signed driver details:
  `Get-CimInstance Win32_PnPSignedDriver | Where-Object { $_.DeviceName -like "*<pattern>*" } | Select-Object DeviceName, DriverVersion, DriverProviderName, Signer`.
