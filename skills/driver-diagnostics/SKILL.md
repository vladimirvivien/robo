---
name: driver-diagnostics
description: Diagnoses missing drivers, PnP hardware error codes, driver signing, and kernel hotplug ring logs.
triggers:
  keywords: ["driver info", "missing driver", "device error", "pnp device", "kernel module", "hotplug", "dmesg hardware", "hardware driver", "device status"]
---

# Hardware Driver & Hotplug Diagnostics Guidelines

When diagnosing hardware malfunctions, missing kernel drivers, or connection events:

## 1. PnP Status & Error Code Inspection
* **Linux:**
  * PCIe device listing with active kernel driver and modules: `lspci -nnk`
  * Check loaded kernel modules: `lsmod`
  * Inspect specific module info: `modinfo <module_name>`
* **Windows PowerShell:**
  * Find devices with errors or warnings:
    `Get-PnpDevice | Where-Object { $_.Status -notin @("OK", "Unknown") } | Select-Object Class, FriendlyName, Status, Problem, InstanceId`
  * Inspect signed driver info:
    `Get-CimInstance Win32_PnPSignedDriver | Where-Object { $_.DeviceName -like "*<pattern>*" } | Select-Object DeviceName, DriverVersion, DriverProviderName, Signer`

## 2. Kernel Hardware & Hotplug Ring Logs
* **Linux:**
  * Recent hardware events (USB, PCI, power, drivers):
    `dmesg -T | grep -iE "usb|pci|driver|attached|detached|fail|error" | tail -n 30`
  * Monitor live udev events: `udevadm monitor --environment --kernel`
* **macOS:**
  * Kernel logs: `log show --predicate 'process == "kernel"' --last 10m`

## 3. Resolving Driver Issues
* **Linux:**
  * Load module manually: `sudo modprobe <module_name>`
  * Unload module: `sudo modprobe -r <module_name>`
* **Windows:**
  * Restart device: `Disable-PnpDevice -InstanceId <Id> -Confirm:$false; Enable-PnpDevice -InstanceId <Id> -Confirm:$false`
