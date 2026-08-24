@echo off
setlocal
set "WORKER=%~dp0image-relay-worker-windows-amd64.exe"
if exist "%WORKER%" (
  "%WORKER%" --configure
) else (
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0configure.ps1"
)
