---
name: gpu-discovery
description: Discovers discrete/integrated graphics, AI accelerators, VRAM, and driver telemetry.
triggers:
  keywords: ["gpu info", "graphics card", "nvidia-smi", "vram", "video controller", "cuda gpu", "display adapter", "accelerator", "rocm"]
---

# GPU & AI Accelerator Discovery Guidelines

When inspecting graphics adapters, discrete GPUs, AI accelerators, or driver versions:

## 1. Graphics Adapter & Model Discovery
* **Linux:**
  * PCIe display controllers: `lspci -nnk | grep -A 3 -i "vga\|3d\|display"`.
  * OpenCL / Vulkan runtime devices: `vulkaninfo --summary` or `clinfo -l 2>/dev/null`.
* **macOS:** `system_profiler SPDisplaysDataType`.
* **Windows PowerShell:**
  `Get-CimInstance Win32_VideoController | Select-Object Name, DriverVersion, @{Name="VRAM(MB)";Expression={[math]::Round($_.AdapterRAM/1MB,2)}}, Status`.

## 2. NVIDIA GPU Diagnostics & Telemetry
If NVIDIA hardware is present:
* Summary overview: `nvidia-smi`
* Concise CSV telemetry query:
  `nvidia-smi --query-gpu=index,name,driver_version,memory.total,memory.used,utilization.gpu,temperature.gpu --format=csv,noheader`
* CUDA version check: `nvcc --version` (or from driver: `nvidia-smi | grep "CUDA Version"`).

## 3. AMD ROCm / Intel Arc Telemetry
* AMD ROCm: `rocm-smi` or `rocminfo`.
* Intel GPUs: `intel_gpu_top -s 1000` (requires root) or inspect `/sys/class/drm/card*/device/`.
