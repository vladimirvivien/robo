---
name: storage-discovery
description: Discovers physical drives, NVMe/SATA transport buses, media types (SSD/HDD), and drive serial numbers.
triggers:
  keywords: ["physical disk", "nvme drive", "storage drives", "list disks", "lsblk", "diskutil", "Get-PhysicalDisk", "hard drive", "ssd info", "block devices"]
---

# Physical Storage & Drive Discovery Guidelines

When inspecting physical storage devices, NVMe controllers, SATA interfaces, or external drives:

## 1. Block Device & Partition Topologies
* **Linux:**
  * Block device hierarchy with transport and model:
    `lsblk -o NAME,SIZE,TYPE,MODEL,SERIAL,TRAN,FSTYPE,MOUNTPOINT`
  * Partition UUIDs & labels: `blkid`
* **macOS:**
  * Disk summary: `diskutil list`
  * Detailed storage profile: `system_profiler SPStorageDataType` (or `SPNVMeDataType`)
* **Windows PowerShell:**
  * Physical Disks:
    `Get-PhysicalDisk | Select-Object DeviceId, FriendlyName, MediaType, BusType, @{Name="Size(GB)";Expression={[math]::Round($_.Size/1GB,2)}}, OperationalStatus, HealthStatus`
  * Volume Mappings: `Get-Volume | Select-Object DriveLetter, FileSystemLabel, FileSystem, SizeRemaining, Size`

## 2. NVMe Specific Controller Inspection
* Linux: `nvme list` (if `nvme-cli` installed) or inspect `/sys/class/nvme/`.
* Windows: `Get-PhysicalDisk | Where-Object { $_.BusType -eq "NVMe" }`.

## 3. Drive Health & S.M.A.R.T. Checks
* Linux: `smartctl -H /dev/sdX` (or `/dev/nvme0n1`).
* Windows PowerShell: `Get-PhysicalDisk | Select-Object FriendlyName, HealthStatus, OperationalStatus`.
