---
name: usb-discovery
description: Discovers connected USB devices, hubs, Vendor/Product IDs (VID:PID), and device descriptors.
triggers:
  keywords: ["usb devices", "lsusb", "usb ports", "plugged in usb", "usb hub", "vendor id", "product id", "usb tree"]
---

# USB Device & Peripheral Discovery Guidelines

When inspecting connected USB peripherals, hubs, or controllers:

## 1. Fast USB Device Listing
* **Linux:** `lsusb` (or with hierarchy: `lsusb -t`).
* **macOS:** `system_profiler SPUSBDataType` (or JSON: `system_profiler SPUSBDataType -json`).
* **Windows PowerShell:**
  `Get-PnpDevice -Class USB -PresentOnly | Select-Object FriendlyName, InstanceId, Status`

## 2. Detailed Hardware Descriptors (VID:PID, Serials & Power)
* **Linux:**
  * Detailed verbose descriptor: `lsusb -v -d <VID>:<PID>`.
  * From udev database: `udevadm info --export-db | grep -iE "ID_VENDOR|ID_MODEL|ID_SERIAL"`.
* **macOS:** `ioreg -p IOUSB -l -w 0`.
* **Windows PowerShell:**
  `Get-CimInstance Win32_USBControllerDevice | ForEach-Object { [Wmi]$_.Dependent } | Select-Object Name, DeviceID, Status`.

## 3. Troubleshooting Missing USB Devices
* Check if the device is drawing power or acknowledged on the bus:
  * Linux: `dmesg -T | grep -i "usb" | tail -n 20`.
  * Windows: `Get-PnpDevice -Class USB | Where-Object { $_.Status -ne "OK" }`.
