# Windows MSI installer

The MSI is built in GitHub Actions with WiX Toolset 4. It installs and starts the worker as an automatic Windows service, then opens `http://127.0.0.1:5173` for configuration. The service remains running in the background; no executable needs to be launched manually.
